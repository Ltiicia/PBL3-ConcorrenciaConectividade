package main

import (
	"fmt"
	"strings"
	"time"

	mqtt "github.com/eclipse/paho.mqtt.golang"
)

var mqttClient mqtt.Client

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

// Processa reserva via MQTT
func handleReservaMqtt(placa, ponto string) {
	// Verifica se o ponto pertence a esta empresa
	pontoValido := false
	for _, pontoDaEmpresa := range empresa.Pontos {
		if ponto == pontoDaEmpresa {
			pontoValido = true
			break
		}
	}

	if !pontoValido {
		// Notifica que o ponto não pertence a esta empresa
		resposta := fmt.Sprintf("reserva_negada,%s,Ponto não pertence a esta empresa", ponto)
		publicaMensagemMqtt(mqttClient, "mensagens/cliente/"+placa, resposta)
		return
	}

	// Cria transação de reserva no blockchain
	transacao := Transacao{
		Tipo:    "RESERVA",
		Placa:   placa,
		Valor:   0.0,
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
		resposta := fmt.Sprintf("reserva_erro,%s,Erro na assinatura digital", ponto)
		publicaMensagemMqtt(mqttClient, "mensagens/cliente/"+placa, resposta)
		return
	}

	novo_bloco := NovoBloco(transacao, ultimo, empresa.ID, assinatura)
	if blocoDuplicado(novo_bloco) {
		mutex.Unlock()
		resposta := fmt.Sprintf("reserva_erro,%s,Bloco duplicado", ponto)
		publicaMensagemMqtt(mqttClient, "mensagens/cliente/"+placa, resposta)
		return
	}

	blockchain.Chain = append(blockchain.Chain, novo_bloco)
	SalvarBlockchain("data/chain_"+empresa.ID+".json", blockchain)
	mutex.Unlock()

	// Propaga o bloco para outras empresas
	propagarBloco(novo_bloco)

	// Notifica sucesso com hash
	resposta := fmt.Sprintf("reserva_confirmada,%s,%s", ponto, novo_bloco.Hash)
	publicaMensagemMqtt(mqttClient, "mensagens/cliente/"+placa, resposta)

	fmt.Printf("[BLOCKCHAIN] Reserva registrada para %s no ponto %s - Hash: %s\n", placa, ponto, novo_bloco.Hash)
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
