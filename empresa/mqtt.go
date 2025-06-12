package main

import (
	"fmt"
	"strings"
	"time"

	mqtt "github.com/eclipse/paho.mqtt.golang"
)

var mqttClient mqtt.Client

// Função auxiliar para liberar ponto completamente (controle + reservas)
func liberarPontoCompleto(ponto, placa string) {
	// Libera no sistema de controle de pontos
	liberarPonto(ponto, placa)

	// Libera no sistema de reservas em memória
	reservas_mutex.Lock()
	if pontosMap, existe := reservas[placa]; existe {
		delete(pontosMap, ponto)
		if len(pontosMap) == 0 {
			delete(reservas, placa)
		}
	}
	reservas_mutex.Unlock()

	fmt.Printf("[CLEANUP] Ponto %s liberado completamente para %s\n", ponto, placa)
}

// Publica mensagem em um tópico MQTT específico
func publicaMensagemMqtt(client mqtt.Client, topico string, mensagem string) {
	token := client.Publish(topico, 0, false, mensagem)
	token.Wait()
	fmt.Printf("\n[MQTT] Mensagem enviada para %s: %s\n", topico, mensagem)
}

// Retorna o cliente MQTT atual
func getClienteMqtt() mqtt.Client {
	return mqttClient
}

// Inicializa a conexão MQTT com o broker e configura inscrição nos tópicos
func inicializaMqtt(idCliente string) {
	// O servidor se conecta via TCP ao broker
	opts := mqtt.NewClientOptions().AddBroker("tcp://broker:1883")
	opts.SetClientID(idCliente)

	opts.OnConnect = func(c mqtt.Client) {
		fmt.Println("[MQTT] Servidor inicializado - Empresa " + idCliente + " conectado ao broker")

		// Subscribe para mensagens de clientes
		if token := c.Subscribe("mensagens/cliente", 0, handleMensagens); token.Wait() && token.Error() != nil {
			fmt.Println("[MQTT] Erro ao assinar tópico:", token.Error())
		}

		// Subscribe para mensagens específicas da empresa
		topicoEmpresa := "mensagens/empresa/" + idCliente
		if token := c.Subscribe(topicoEmpresa, 0, handleMensagensEmpresa); token.Wait() && token.Error() != nil {
			fmt.Println("[MQTT] Erro ao assinar tópico da empresa:", token.Error())
		}
	}

	// Conecta ao broker (Mosquitto)
	mqttClient = mqtt.NewClient(opts)
	if token := mqttClient.Connect(); token.Wait() && token.Error() != nil {
		panic(token.Error())
	}
}

// Handler para mensagens gerais de clientes
func handleMensagens(client mqtt.Client, msg mqtt.Message) {
	mensagem := string(msg.Payload())
	fmt.Printf("[MQTT] Mensagem recebida: %s\n", mensagem)

	// Parse da mensagem: "TIPO,PLACA,DADOS"
	partes := strings.Split(mensagem, ",")
	if len(partes) < 2 {
		return
	}

	tipo := partes[0]
	placa := partes[1]

	switch tipo {
	case "RESERVA":
		if len(partes) >= 3 {
			ponto := partes[2]
			handleReservaMqtt(placa, ponto)
		}
	case "RECARGA":
		if len(partes) >= 4 {
			ponto := partes[2]
			valor := partes[3]
			handleRecargaMqtt(placa, ponto, valor)
		}
	case "STATUS":
		handleStatusMqtt(placa)
	case "CANCELAR":
		if len(partes) >= 3 {
			ponto := partes[2]
			handleCancelamentoMqtt(placa, ponto)
		}
	}
}

// Handler para mensagens específicas da empresa
func handleMensagensEmpresa(client mqtt.Client, msg mqtt.Message) {
	mensagem := string(msg.Payload())
	fmt.Printf("[MQTT] Mensagem da empresa recebida: %s\n", mensagem)

	// Processa mensagens específicas da empresa
	partes := strings.Split(mensagem, ",")
	if len(partes) < 2 {
		return
	}

	tipo := partes[0]
	switch tipo {
	case "SYNC":
		// Sincronização de blockchain
		handleSyncMqtt()
	case "STATUS_UPDATE":
		// Atualização de status de pontos
		if len(partes) >= 3 {
			ponto := partes[1]
			status := partes[2]
			handleStatusUpdateMqtt(ponto, status)
		}
	}
}

// Processa reserva via MQTT com controle de concorrência COMPLETO (modelo PBL2)
func handleReservaMqtt(placa, ponto string) {
	// Verifica se o ponto pertence a esta empresa
	pontoValido := false
	for _, pontoDaEmpresa := range empresa.Pontos {
		if ponto == pontoDaEmpresa {
			pontoValido = true
			break
		}
	}

	// Se o ponto não pertence a esta empresa, não processa a reserva
	if !pontoValido {
		return
	}

	// *** CONTROLE DE CONCORRÊNCIA ATÔMICO (PBL2) ***
	// Adquire lock específico para este ponto ANTES de qualquer verificação
	lock := ponto_locks[ponto]
	lock.Lock()
	defer lock.Unlock()

	// Verifica status de conectividade do ponto
	status_ponto.RLock()
	conectado := status_ponto.status[ponto]
	status_ponto.RUnlock()

	if !conectado {
		resposta := fmt.Sprintf("ponto_desconectado,%s,Ponto %s está desconectado", ponto, ponto)
		publicaMensagemMqtt(mqttClient, "mensagens/cliente/"+placa, resposta)
		fmt.Printf("[ERRO] Tentativa de reserva no ponto %s falhou: ponto desconectado.\n", ponto)
		return
	}

	// Verifica se o ponto está disponível ATOMICAMENTE dentro do lock
	if !verificarPontoDisponivel(ponto, placa) {
		resposta := fmt.Sprintf("reserva_erro,%s,Ponto já está reservado por outro veículo", ponto)
		publicaMensagemMqtt(mqttClient, "mensagens/cliente/"+placa, resposta)
		fmt.Printf("[CONFLITO] Tentativa de reserva rejeitada para %s em %s: ponto já ocupado\n", placa, ponto)
		return
	}

	// Marca o ponto como reservado ATOMICAMENTE
	if !marcarPontoReservado(ponto, placa) {
		resposta := fmt.Sprintf("reserva_erro,%s,Falha ao marcar ponto como reservado", ponto)
		publicaMensagemMqtt(mqttClient, "mensagens/cliente/"+placa, resposta)
		fmt.Printf("[ERRO] Falha ao marcar ponto %s como reservado para %s\n", ponto, placa)
		return
	}

	// Atualiza o sistema de reservas em memória
	reservas_mutex.Lock()
	if _, existe := reservas[placa]; !existe {
		reservas[placa] = make(map[string]string)
	}
	reservas[placa][ponto] = "confirmado"
	reservas_mutex.Unlock()

	fmt.Printf("[CONCORRÊNCIA] Ponto %s reservado atomicamente para %s\n", ponto, placa)

	// Cria transação de reserva no blockchain
	transacao := Transacao{
		Tipo:    "RESERVA",
		Placa:   placa,
		Valor:   0.0,
		Ponto:   ponto,
		Empresa: empresa.ID,
	}

	// Adiciona ao blockchain com tratamento de erro robusto
	mutex.Lock()
	ultimo := blockchain.Chain[len(blockchain.Chain)-1]
	hash := CalcularHash(Bloco{
		Index:        (ultimo.Index + 1),
		Timestamp:    formatarTimestamp(time.Now().UTC().Format(time.RFC3339)),
		Transacao:    transacao,
		HashAnterior: ultimo.Hash,
		Autor:        empresa.ID,
	})

	assinatura, erro := AssinarBloco(hash, chave_privada_path)
	if erro != nil {
		mutex.Unlock()
		// Desfaz a reserva em caso de erro
		liberarPontoCompleto(ponto, placa)
		resposta := fmt.Sprintf("reserva_erro,%s,Erro na assinatura digital", ponto)
		publicaMensagemMqtt(mqttClient, "mensagens/cliente/"+placa, resposta)
		fmt.Printf("[ERRO] Falha na assinatura para reserva %s em %s: %v\n", placa, ponto, erro)
		return
	}

	novo_bloco := NovoBloco(transacao, ultimo, empresa.ID, assinatura)
	if blocoDuplicado(novo_bloco) {
		mutex.Unlock()
		// Desfaz a reserva em caso de duplicação
		liberarPontoCompleto(ponto, placa)
		resposta := fmt.Sprintf("reserva_erro,%s,Bloco duplicado", ponto)
		publicaMensagemMqtt(mqttClient, "mensagens/cliente/"+placa, resposta)
		fmt.Printf("[ERRO] Bloco duplicado para reserva %s em %s\n", placa, ponto)
		return
	}

	blockchain.Chain = append(blockchain.Chain, novo_bloco)
	SalvarBlockchain("data/chain_"+empresa.ID+".json", blockchain)
	mutex.Unlock()

	// Atualiza o hash da reserva no controle de pontos
	atualizarHashReserva(ponto, placa, novo_bloco.Hash)

	// Propaga o bloco para outras empresas
	propagarBloco(novo_bloco)

	// Notifica sucesso com hash
	resposta := fmt.Sprintf("reserva_confirmada,%s,%s", ponto, novo_bloco.Hash)
	publicaMensagemMqtt(mqttClient, "mensagens/cliente/"+placa, resposta)

	fmt.Printf("[BLOCKCHAIN] Reserva atômica confirmada: %s -> %s (Hash: %s)\n", placa, ponto, novo_bloco.Hash)
}

// Processa recarga via MQTT
func handleRecargaMqtt(placa, ponto, valorStr string) {
	// Converte valor
	var valor float64
	fmt.Sscanf(valorStr, "%f", &valor)

	// Verifica se o ponto pertence a esta empresa
	pontoValido := false
	for _, pontoDaEmpresa := range empresa.Pontos {
		if ponto == pontoDaEmpresa {
			pontoValido = true
			break
		}
	}

	if !pontoValido {
		resposta := fmt.Sprintf("recarga_negada,%s,Ponto não pertence a esta empresa", ponto)
		publicaMensagemMqtt(mqttClient, "mensagens/cliente/"+placa, resposta)
		return
	}

	// Cria transação de recarga no blockchain
	transacao := Transacao{
		Tipo:    "RECARGA",
		Placa:   placa,
		Valor:   valor,
		Ponto:   ponto,
		Empresa: empresa.ID,
	}

	// Adiciona ao blockchain
	mutex.Lock()
	ultimo := blockchain.Chain[len(blockchain.Chain)-1]
	hash := CalcularHash(Bloco{
		Index:        (ultimo.Index + 1),
		Timestamp:    formatarTimestamp(time.Now().UTC().Format(time.RFC3339)),
		Transacao:    transacao,
		HashAnterior: ultimo.Hash,
		Autor:        empresa.ID,
	})

	assinatura, erro := AssinarBloco(hash, chave_privada_path)
	if erro != nil {
		mutex.Unlock()
		resposta := fmt.Sprintf("recarga_erro,%s,Erro na assinatura digital", ponto)
		publicaMensagemMqtt(mqttClient, "mensagens/cliente/"+placa, resposta)
		return
	}

	novo_bloco := NovoBloco(transacao, ultimo, empresa.ID, assinatura)
	blockchain.Chain = append(blockchain.Chain, novo_bloco)
	SalvarBlockchain("data/chain_"+empresa.ID+".json", blockchain)
	mutex.Unlock()

	// Propaga o bloco
	propagarBloco(novo_bloco)
	// Libera automaticamente o ponto após recarga completa
	liberarPontoAposRecarga(placa, ponto)

	// Notifica sucesso com hash
	resposta := fmt.Sprintf("recarga_confirmada,%s,%.2f,%s", ponto, valor, novo_bloco.Hash)
	publicaMensagemMqtt(mqttClient, "mensagens/cliente/"+placa, resposta)

	fmt.Printf("[BLOCKCHAIN] Recarga registrada para %s no ponto %s - Valor: R$ %.2f - Hash: %s\n", placa, ponto, valor, novo_bloco.Hash)
}

// Processa solicitação de status via MQTT
func handleStatusMqtt(placa string) {
	// Busca histórico do veículo no blockchain
	var transacoes []Bloco
	for _, bloco := range blockchain.Chain {
		if bloco.Transacao.Placa == placa {
			transacoes = append(transacoes, bloco)
		}
	}

	// Prepara resposta com resumo
	totalRecargas := 0
	totalPagamentos := 0
	valorRecargas := 0.0
	valorPagamentos := 0.0

	for _, bloco := range transacoes {
		switch bloco.Transacao.Tipo {
		case "RECARGA":
			totalRecargas++
			valorRecargas += bloco.Transacao.Valor
		case "PAGAMENTO":
			totalPagamentos++
			valorPagamentos += bloco.Transacao.Valor
		}
	}

	saldoPendente := valorRecargas - valorPagamentos

	resposta := fmt.Sprintf("status_resposta,%d,%d,%.2f,%.2f,%.2f",
		totalRecargas, totalPagamentos, valorRecargas, valorPagamentos, saldoPendente)

	publicaMensagemMqtt(mqttClient, "mensagens/cliente/"+placa, resposta)
}

// Processa sincronização via MQTT
func handleSyncMqtt() {
	fmt.Println("[MQTT] Solicitação de sincronização recebida")
	// Aqui poderia implementar sincronização adicional se necessário
}

// Processa atualização de status de pontos via MQTT
func handleStatusUpdateMqtt(ponto, status string) {
	fmt.Printf("[MQTT] Atualização de status recebida - Ponto: %s, Status: %s\n", ponto, status)
	// Implementar lógica de atualização de status se necessário
}

// Libera automaticamente o ponto após recarga completa
func liberarPontoAposRecarga(placa, ponto string) {
	fmt.Printf("[RECARGA] Liberando ponto %s após recarga de %s\n", ponto, placa)

	// Usa liberação completa (controle + reservas)
	liberarPontoCompleto(ponto, placa)

	// Notifica via MQTT que o ponto foi liberado
	mensagem := fmt.Sprintf("ponto_liberado,%s,Ponto liberado após recarga", ponto)
	publicaMensagemMqtt(mqttClient, "mensagens/cliente/"+placa, mensagem)
}

// Sistema de timeout para reservas (modelo PBL2)
// Libera automaticamente reservas após período definido
func liberaPorTimeout(placa string, pontos []string, tempo time.Duration) {
	go func() {
		time.Sleep(tempo)
		fmt.Printf("[TIMEOUT] Verificando timeout para reservas do veículo %s...\n", placa)

		for _, ponto := range pontos {
			// Adquire lock específico do ponto
			lock := ponto_locks[ponto]
			lock.Lock()

			// Verifica se a reserva ainda existe
			reservas_mutex.Lock()
			if pontosMap, existe := reservas[placa]; existe {
				if _, reservado := pontosMap[ponto]; reservado {
					// Remove da memória
					delete(pontosMap, ponto)
					if len(pontosMap) == 0 {
						delete(reservas, placa)
					}

					// Libera o controle do ponto
					liberarPonto(ponto, placa)

					fmt.Printf("[TIMEOUT] Reserva para %s no ponto %s expirada por timeout\n", placa, ponto)

					// Notifica o cliente via MQTT
					if mqttClient != nil && mqttClient.IsConnected() {
						mensagem := fmt.Sprintf("reserva_expirada,%s,Reserva expirou por timeout", ponto)
						publicaMensagemMqtt(mqttClient, "mensagens/cliente/"+placa, mensagem)
					}
				}
			}
			reservas_mutex.Unlock()
			lock.Unlock()
		}
	}()
}

// Processa cancelamento via MQTT com controle de concorrência
func handleCancelamentoMqtt(placa, ponto string) {
	// Verifica se o ponto pertence a esta empresa
	pontoValido := false
	for _, pontoDaEmpresa := range empresa.Pontos {
		if ponto == pontoDaEmpresa {
			pontoValido = true
			break
		}
	}

	if !pontoValido {
		return // Não processa se o ponto não pertence a esta empresa
	}

	// *** CONTROLE DE CONCORRÊNCIA ATÔMICO ***
	lock := ponto_locks[ponto]
	lock.Lock()
	defer lock.Unlock()

	// Cancela a reserva atomicamente
	liberarPontoCompleto(ponto, placa)

	// Notifica sucesso
	resposta := fmt.Sprintf("cancelamento_confirmado,%s,Reserva cancelada com sucesso", ponto)
	publicaMensagemMqtt(mqttClient, "mensagens/cliente/"+placa, resposta)

	fmt.Printf("[CANCELAMENTO] Reserva de %s no ponto %s cancelada\n", placa, ponto)
}
