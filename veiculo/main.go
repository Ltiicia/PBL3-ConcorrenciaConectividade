package main

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"
)

type Transacao struct {
	Tipo    string  `json:"tipo"`
	Placa   string  `json:"placa"`
	Valor   float64 `json:"valor"`
	Ponto   string  `json:"ponto"`
	Empresa string  `json:"empresa"`
}

// Estruturas para sistema de reservas
type ReservaRequest struct {
	PlacaVeiculo string   `json:"placa_veiculo"`
	Pontos       []string `json:"pontos"`
	EmpresaID    string   `json:"empresa_id"`
}

type ReservaResponse struct {
	Status    string `json:"status"`
	Ponto     string `json:"ponto"`
	Mensagem  string `json:"mensagem"`
	EmpresaID string `json:"empresa_id"`
	Hash      string `json:"hash"`
}

type VerificacaoHash struct {
	Hash      string                 `json:"hash"`
	Valido    bool                   `json:"valido"`
	Transacao map[string]interface{} `json:"transacao"`
	Mensagem  string                 `json:"mensagem"`
}

type Bloco struct {
	Index        int       `json:"index"`
	Timestamp    string    `json:"timestamp"`
	Transacao    Transacao `json:"transacao"`
	HashAnterior string    `json:"hash_anterior"`
	Hash         string    `json:"hash"`
	Autor        string    `json:"autor"`
	Assinatura   string    `json:"assinatura"`
}

type Veiculos struct {
	Placas map[string]bool `json:"placas"`
}

type Blockchain struct {
	Chain []Bloco `json:"blocos"`
}

var empresasAPI = map[string]string{
	"001": "http://empresa_001:8001",
	"002": "http://empresa_002:8002",
	"003": "http://empresa_003:8003",
}

var placa_veiculo string
var veiculo_atual VeiculoCompleto

// Estruturas para recargas e pagamentos
type RecargaInfo struct {
	Ponto         string  `json:"ponto"`
	Empresa       string  `json:"empresa"`
	Valor         float64 `json:"valor"`
	WattsHora     float64 `json:"watts_hora"`
	HashRecarga   string  `json:"hash_recarga"`
	Pago          bool    `json:"pago"`
	HashPagamento string  `json:"hash_pagamento,omitempty"`
}

// Sistema de armazenamento de recargas pendentes
var recargasPendentesStorage = make(map[string][]RecargaInfo) // placa -> recargas

// Função para limpeza segura ao sair do sistema
func limpezaSistema(placa string) {
	fmt.Println("\n🧹 Executando limpeza do sistema...")

	// Desconectar MQTT
	desconectarMqtt()

	// Remover placa da lista ativa
	removerPlaca(placa)

	// Remover sessão ativa para liberar a placa
	err := removerSessaoAtiva(placa)
	if err != nil {
		fmt.Printf("⚠️  Aviso: Erro ao remover sessão: %v\n", err)
	} else {
		fmt.Printf("✅ Sessão da placa %s liberada com sucesso\n", placa)
	}

	fmt.Println("✅ Limpeza concluída!")
}

// Configura tratamento de sinais para limpeza em caso de interrupção
func configurarTratamentoSinais(placa *string) {
	c := make(chan os.Signal, 1)
	signal.Notify(c, os.Interrupt, syscall.SIGTERM, syscall.SIGQUIT)

	go func() {
		<-c
		fmt.Println("\n🛑 Sinal de interrupção recebido...")

		if placa != nil && *placa != "" {
			limpezaSistema(*placa)
		}

		fmt.Println("👋 Sistema encerrado com sucesso!")
		os.Exit(0)
	}()
}

func main() {
	fmt.Println("🚗 Sistema de Veículos Elétricos com Blockchain")
	fmt.Println("============================================")
	fmt.Println("Para iniciar, informe a placa do seu veículo...")

	leitor := bufio.NewReader(os.Stdin)
	placa_validada := false
	var clienteID string
	var placa_veiculo string

	// Configura tratamento de sinais para limpeza automática
	configurarTratamentoSinais(&placa_veiculo)

	for !placa_validada {
		fmt.Print("Placa: ")
		placa, _ := leitor.ReadString('\n')
		placa = strings.TrimSpace(placa)

		if placa == "" {
			fmt.Println("❌ Placa inválida")
			continue
		}

		// Verifica se a placa já está sendo usada
		placaAtiva, mensagem := verificarPlacaAtiva(placa)
		if placaAtiva {
			fmt.Println(mensagem)
			fmt.Println("🔒 Não é possível usar uma placa que já está ativa no sistema.")
			fmt.Println("⏳ Aguarde o outro veículo encerrar a sessão ou use uma placa diferente.")
			continue
		}

		// Tenta login ou cadastro
		veiculo, isLogin, err := loginOuCadastro(placa)
		if err != nil {
			fmt.Printf("❌ Erro ao processar veículo: %v\n", err)
			continue
		}

		// Registra sessão ativa
		clienteID, err = registrarSessaoAtiva(placa)
		if err != nil {
			fmt.Printf("❌ Erro ao registrar sessão: %v\n", err)
			continue
		}

		veiculo_atual = veiculo
		placa_veiculo = placa
		placa_validada = true

		if !isLogin {
			// Só cadastra na lista de placas ativas se for novo veículo
			if !cadastrarPlaca(placa) {
				fmt.Println("⚠️  Aviso: Erro ao registrar placa na lista ativa")
			}
		}
		// Inicializa MQTT para este veículo usando ID único
		fmt.Println("🔌 Conectando ao sistema MQTT...")
		inicializaMqttVeiculoComID(placa, clienteID)
		if mqttConectado() {
			fmt.Println("✅ Conectado ao sistema de comunicação!")
		} else {
			fmt.Println("⚠️  Aviso: Sistema MQTT não disponível, usando apenas HTTP")
		}
	}
	for {
		fmt.Println("\n ============== Menu ==============")
		fmt.Println("1 - Programar viagem")
		fmt.Println("2 - Realizar recarga")
		fmt.Println("3 - Pagar recargas pendentes")
		fmt.Println("4 - Consultar extrato")
		fmt.Println("5 - Verificar hash")
		fmt.Println("6 - Ver histórico completo")
		fmt.Println("7 - Ver histórico de viagens")
		fmt.Println("0 - Sair")
		fmt.Print("Selecione uma opção: ")
		opcao, _ := leitor.ReadString('\n')
		opcao = strings.TrimSpace(opcao)
		switch opcao {
		case "1":
			programarViagem(placa_veiculo, leitor)
		case "2":
			realizarRecarga(placa_veiculo, leitor)
		case "3":
			pagarRecargasPendentes(placa_veiculo)
		case "4":
			verExtrato(placa_veiculo)
		case "5":
			verificarHash(leitor)
		case "6":
			verHistoricoCompleto(placa_veiculo)
		case "7":
			verHistoricoViagens(placa_veiculo)
		case "0":
			fmt.Println("👋 Encerrando sistema...")
			limpezaSistema(placa_veiculo)
			fmt.Println("✅ Sistema encerrado com sucesso!")
			return
		default:
			fmt.Println("Opção inválida! Tente novamente")
		}
	}
}

func listCapitaisNordeste() {
	fmt.Println("\n======= Cidades com Servico de Recarga =======")
	fmt.Println("(1) - Salvador")
	fmt.Println("(2) - Aracaju")
	fmt.Println("(3) - Maceio")
	fmt.Println("(4) - Recife")
	fmt.Println("(5) - Joao Pessoa")
	fmt.Println("(6) - Natal")
	fmt.Println("(7) - Fortaleza")
	fmt.Println("(8) - Teresina")
	fmt.Println("(9) - Sao Luis")
	fmt.Println("(0) - Retornar ao Menu")
}

func realizarRecarga(placa string, leitor *bufio.Reader) {
	pontos := map[string]string{
		"1": "Salvador",
		"2": "Aracaju",
		"3": "Maceio",
		"4": "Recife",
		"5": "Joao Pessoa",
		"6": "Natal",
		"7": "Fortaleza",
		"8": "Teresina",
		"9": "Sao Luis",
	}
	listCapitaisNordeste()
	fmt.Print("Selecione a cidade que deseja recarregar: ")
	opcao, _ := leitor.ReadString('\n')
	opcao = strings.TrimSpace(opcao)
	ponto, ok := pontos[opcao]
	if !ok {
		fmt.Println("Cidade inválido")
		return
	}
	fmt.Print("Valor da recarga: R$ ")
	valorStr, _ := leitor.ReadString('\n')
	valorStr = strings.TrimSpace(valorStr)
	var valor float64
	fmt.Sscanf(valorStr, "%f", &valor)
	if valor <= 0 {
		fmt.Println("Valor inválido")
		return
	}
	empresa_id := empresaPorPonto(ponto)
	fmt.Printf("🏢 Empresa responsável pelo ponto %s: %s\n", ponto, empresa_id)
	if empresa_id == "" {
		fmt.Println("❌ Empresa não encontrada para o ponto!")
		return
	}

	// Tenta primeiro via MQTT se disponível
	if mqttConectado() {
		fmt.Println("📡 Enviando recarga via MQTT...")
		solicitarRecargaMqtt(placa, ponto, valor)

		// Aguarda resposta por alguns segundos
		resposta := aguardarRespostaMqtt(5 * time.Second)
		if resposta != "" {
			partes := strings.Split(resposta, ",")
			if len(partes) >= 4 && partes[0] == "recarga_confirmada" {
				fmt.Printf("✅ Recarga confirmada! Hash: %s\n", partes[3])
				return
			}
		}
		fmt.Println("⚠️  Timeout MQTT, tentando via HTTP...")
	}

	// Fallback para HTTP
	transacao := Transacao{
		Tipo:    "RECARGA",
		Placa:   placa,
		Valor:   valor,
		Ponto:   ponto,
		Empresa: empresa_id,
	}
	json_data, _ := json.Marshal(transacao)
	fmt.Printf("🔄 Enviando recarga para %s\n", empresasAPI[empresa_id]+"/recarga")
	resp, err := http.Post(empresasAPI[empresa_id]+"/recarga", "application/json", bytes.NewBuffer(json_data))
	if err != nil || resp.StatusCode != 201 {
		fmt.Printf("❌ Erro ao registrar recarga: %v, status: %v\n", err, resp)
		return
	}
	fmt.Println("✅ Recarga registrada com sucesso!")
}

func pagarRecargasPendentes(placa string) {
	fmt.Println("\n💳 ========== Pagar Recargas Pendentes ==========")

	// Verificar se há recargas no novo sistema
	recargas, exists := recargasPendentesStorage[placa]
	if exists && len(recargas) > 0 {
		fmt.Printf("📋 Recargas pendentes encontradas: %d\n", len(recargas))

		pendentes := 0
		for _, recarga := range recargas {
			if !recarga.Pago {
				pendentes++
			}
		}

		if pendentes > 0 {
			fmt.Printf("💰 Recargas não pagas: %d\n", pendentes)
			processarPagamentosRecargas(placa)
			return
		} else {
			fmt.Println("✅ Todas as recargas do sistema novo já foram pagas!")
		}
	}

	// Fallback para o sistema antigo (blockchain)
	fmt.Println("🔍 Verificando recargas no sistema blockchain...")
	chain := buscarBlockchain()
	pendentes := recargasPendentes(placa, chain)

	if len(pendentes) == 0 {
		fmt.Println("📭 Nenhuma recarga pendente encontrada")
		return
	}

	fmt.Printf("📋 Recargas pendentes no blockchain: %d\n", len(pendentes))

	for _, rec := range pendentes {
		fmt.Printf("💰 Processando pagamento para recarga em %s - empresa (%s) valor: R$ %.2f\n", rec.Ponto, rec.Empresa, rec.Valor)
		transacao := Transacao{
			Tipo:    "PAGAMENTO",
			Placa:   placa,
			Valor:   rec.Valor,
			Ponto:   rec.Ponto,
			Empresa: rec.Empresa,
		}
		json_data, _ := json.Marshal(transacao)

		resp, err := http.Post(empresasAPI[rec.Empresa]+"/pagamento", "application/json", bytes.NewBuffer(json_data))
		if err != nil || resp.StatusCode != 201 {
			fmt.Printf("❌ Erro ao pagar recarga em %s!\n", rec.Ponto)
			continue
		}

		// Buscar hash do pagamento
		chainAtualizada := buscarBlockchain()
		hashPagamento := ""
		if len(chainAtualizada.Chain) > 0 {
			for i := len(chainAtualizada.Chain) - 1; i >= 0; i-- {
				bloco := chainAtualizada.Chain[i]
				if bloco.Transacao.Placa == placa &&
					bloco.Transacao.Tipo == "PAGAMENTO" &&
					bloco.Transacao.Ponto == rec.Ponto &&
					bloco.Transacao.Valor == rec.Valor {
					hashPagamento = bloco.Hash
					break
				}
			}
		}
		fmt.Printf("✅ Pagamento realizado para recarga em %s!\n", rec.Ponto)
		if hashPagamento != "" {
			fmt.Printf("🧾 Hash do pagamento: %s\n", hashPagamento)
		}
	}
}

func verExtrato(placa string) {
	chain := buscarBlockchain()
	fmt.Println("\nExtrato de transações:")
	for _, bloco := range chain.Chain {
		if bloco.Transacao.Placa == placa {
			fmt.Printf("%s | %s     | %s    | %s | R$ %.2f\n", bloco.Timestamp, bloco.Transacao.Tipo, bloco.Transacao.Ponto, bloco.Transacao.Empresa, bloco.Transacao.Valor)
		}
	}
}

func buscarBlockchain() Blockchain {
	ids := []string{"001", "002", "003"}
	for _, id := range ids {
		response, erro := http.Get(empresasAPI[id] + "/blockchain")
		if erro == nil {
			defer response.Body.Close()
			body, _ := io.ReadAll(response.Body)
			var chain Blockchain
			json.Unmarshal(body, &chain)
			fmt.Printf("Blockchain recebida da empresa (%s) com %d blocos\n", id, len(chain.Chain))
			return chain
		}
		fmt.Printf("Erro ao buscar blockchain da empresa %s: %v\n", id, erro)
	}
	fmt.Println("Não foi possível buscar a blockchain de nenhuma empresa")
	return Blockchain{}
}

func recargasPendentes(placa string, chain Blockchain) []Transacao {
	var recargas []Transacao
	var pagamentos []Transacao
	for _, bloco := range chain.Chain {
		if bloco.Transacao.Placa == placa {
			if bloco.Transacao.Tipo == "RECARGA" {
				recargas = append(recargas, bloco.Transacao)
			} else if bloco.Transacao.Tipo == "PAGAMENTO" {
				pagamentos = append(pagamentos, bloco.Transacao)
			}
		}
	}
	fmt.Printf("Total de recargas: %d, pagamentos: %d\n", len(recargas), len(pagamentos))
	var pendentes []Transacao
	for _, recarga := range recargas {
		pago := false
		for _, pagamento := range pagamentos {
			if pagamento.Ponto == recarga.Ponto && pagamento.Valor == recarga.Valor && pagamento.Empresa == recarga.Empresa {
				pago = true
				break
			}
		}
		if !pago {
			pendentes = append(pendentes, recarga)
		}
	}
	fmt.Printf("Recargas pendentes: %d\n", len(pendentes))
	return pendentes
}

func empresaPorPonto(ponto string) string {
	// Usa o mapeamento atualizado do veiculo_data.go
	if empresa, exists := pontoParaEmpresa[ponto]; exists {
		return empresa
	}
	return ""
}

func cadastrarPlaca(placa string) bool {
	path := "data/veiculos.json"
	var veiculos Veiculos

	file, err := os.ReadFile(path)
	if err == nil {
		json.Unmarshal(file, &veiculos)
	} else {
		veiculos.Placas = make(map[string]bool)
	}
	if veiculos.Placas[placa] {
		return false
	}
	veiculos.Placas[placa] = true
	data, _ := json.MarshalIndent(veiculos, "", "  ")
	os.WriteFile(path, data, 0644)
	return true
}

func removerPlaca(placa string) {
	path := "data/veiculos.json"
	var veiculos Veiculos
	file, err := os.ReadFile(path)
	if err == nil {
		json.Unmarshal(file, &veiculos)
		if veiculos.Placas[placa] {
			delete(veiculos.Placas, placa)
			data, _ := json.MarshalIndent(veiculos, "", "  ")
			os.WriteFile(path, data, 0644)
			fmt.Printf("Placa %s removida do registro.\n", placa)
		}
	}
}

// Programar viagem com reservas
func programarViagem(placa string, leitor *bufio.Reader) {
	fmt.Println("\n========== Programar Viagem ==========")

	// Limpa registros de reservas confirmadas da viagem anterior
	limparReservasConfirmadas()

	// Selecionar origem
	origem := selecionarCidade("origem", leitor)
	if origem == "" {
		return
	}

	// Selecionar destino
	destino := selecionarCidade("destino", leitor)
	if destino == "" {
		return
	}

	if origem == destino {
		fmt.Println("❌ Origem e destino não podem ser iguais!")
		return
	}
	// Calcular rota e pontos necessários
	rota := calcularRotaViagem(origem, destino)
	distanciaTotal := calcularDistanciaTotal(rota)
	pontosNecessarios := calcularPontosRecarga(rota, &veiculo_atual)

	fmt.Printf("\n🗺️  Rota planejada: %s → %s\n", cidadeParaID[origem], cidadeParaID[destino])
	fmt.Printf("📍 Cidades na rota: %v\n", rota)
	fmt.Printf("📏 Distância total: %.1f km\n", distanciaTotal)

	if len(pontosNecessarios) == 0 {
		fmt.Println("✅ Para este trajeto não será necessário recarregar!")
		salvarViagem(placa, cidadeParaID[origem], cidadeParaID[destino], rota, "COMPLETA_SEM_RECARGA")
		simularViagemSemRecarga(placa, cidadeParaID[origem], cidadeParaID[destino])
		return
	}

	fmt.Printf("\n🔋 Pontos necessários para recarga:\n")
	for i, ponto := range pontosNecessarios {
		empresaID := pontoParaEmpresa[ponto]
		fmt.Printf("   [%d] %s (Empresa: %s)\n", i+1, ponto, empresaID)
	}

	// Confirmar viagem
	fmt.Print("\n❓ Deseja confirmar esta viagem? (S/N): ")
	confirmacao, _ := leitor.ReadString('\n')
	confirmacao = strings.TrimSpace(strings.ToLower(confirmacao))

	if confirmacao != "s" && confirmacao != "sim" {
		fmt.Println("❌ Viagem cancelada!")
		return
	}
	// Realizar reservas atômicas - todos os pontos devem ser reservados com sucesso
	fmt.Println("\n🔄 Realizando reservas atômicas...")
	fmt.Println("⚠️  Todos os pontos devem ser reservados com sucesso, ou nenhum será reservado!")
	reservasConfirmadas := fazerReservasAtomicas(placa, pontosNecessarios)

	// Verificar se todas as reservas foram confirmadas
	if len(reservasConfirmadas) == 0 {
		fmt.Println("\n❌ Não foi possível realizar nenhuma reserva! Pontos não disponíveis.")
		fmt.Println("💡 Tente novamente mais tarde quando os pontos estiverem disponíveis.")
		return
	}

	if len(reservasConfirmadas) < len(pontosNecessarios) {
		fmt.Printf("\n❌ Reserva atômica falhou! Apenas %d/%d pontos estavam disponíveis.\n", len(reservasConfirmadas), len(pontosNecessarios))
		fmt.Println("💡 Tente novamente mais tarde quando todos os pontos estiverem disponíveis.")
		// Nota: O cancelamento das reservas parciais já foi feito na função fazerReservasAtomicas
		return
	}

	// Todas as reservas foram bem-sucedidas
	status := "RESERVAS_COMPLETAS"
	fmt.Printf("\n✅ Reserva atômica bem-sucedida! %d/%d pontos reservados com sucesso!\n", len(reservasConfirmadas), len(pontosNecessarios))

	// Iniciar simulação da viagem
	fmt.Println("\n🚗 Iniciando simulação da viagem...")
	simularViagemComRecargas(placa, cidadeParaID[origem], cidadeParaID[destino], reservasConfirmadas, leitor)

	// Salvar viagem no histórico
	pontosReservados := make([]string, 0, len(reservasConfirmadas))
	for ponto := range reservasConfirmadas {
		pontosReservados = append(pontosReservados, ponto)
	}
	salvarViagem(placa, cidadeParaID[origem], cidadeParaID[destino], pontosReservados, status)
}

// Fazer reserva atômica com melhor controle de concorrência
func fazerReservaAtomica(placa, ponto, empresaID string) string {
	// fmt.Printf("🔄 Fazendo reserva para %s no ponto %s...\n", placa, ponto)

	// Canal para controlar timeout e resposta
	respChan := make(chan string, 1)

	// Goroutine para tentar MQTT primeiro
	go func() {
		if mqttConectado() {
			fmt.Println("📡 Enviando reserva via MQTT...")
			solicitarReservaMqtt(placa, ponto)

			// Aguarda resposta específica para esta reserva
			deadline := time.Now().Add(3 * time.Second)
			for time.Now().Before(deadline) {
				resposta := aguardarRespostaMqtt(100 * time.Millisecond)
				if resposta != "" {
					partes := strings.Split(resposta, ",")
					if len(partes) >= 3 && partes[0] == "reserva_confirmada" && partes[1] == ponto {
						respChan <- partes[2] // Hash da reserva
						return
					}
				}
			}
		}

		// Fallback para HTTP
		fmt.Println("⚠️  Timeout MQTT, tentando via HTTP...")
		hash := tentarReservaHTTP(placa, ponto, empresaID)
		respChan <- hash
	}()

	// Aguarda resposta com timeout
	select {
	case hash := <-respChan:
		if hash != "" {
			return hash
		}
	case <-time.After(5 * time.Second):
		fmt.Println("⏰ Timeout na reserva")
	}

	return ""
}

// Tenta reserva via HTTP
func tentarReservaHTTP(placa, ponto, empresaID string) string {
	transacao := Transacao{
		Tipo:    "RESERVA",
		Placa:   placa,
		Valor:   0.0,
		Ponto:   ponto,
		Empresa: empresaID,
	}

	jsonData, _ := json.Marshal(transacao)
	resp, err := http.Post(empresasAPI[empresaID]+"/reserva", "application/json", bytes.NewBuffer(jsonData))

	if err != nil || resp.StatusCode != 201 {
		fmt.Printf("❌ Erro HTTP na reserva: %v\n", err)
		return ""
	}

	// Busca o hash da transação criada
	chain := buscarBlockchain()
	if len(chain.Chain) > 0 {
		// Busca a transação mais recente desta placa/ponto
		for i := len(chain.Chain) - 1; i >= 0; i-- {
			bloco := chain.Chain[i]
			if bloco.Transacao.Placa == placa &&
				bloco.Transacao.Tipo == "RESERVA" &&
				bloco.Transacao.Ponto == ponto {
				return bloco.Hash
			}
		}
	}

	return "HASH_HTTP_" + ponto + "_" + time.Now().Format("150405")
}

// Fazer reserva e retornar hash (função original mantida para compatibilidade)
func fazerReserva(placa, ponto, empresaID string) string {
	fmt.Printf("🔄 Fazendo reserva para %s no ponto %s...\n", placa, ponto)

	// Tenta primeiro via MQTT se disponível
	if mqttConectado() {
		fmt.Println("📡 Enviando reserva via MQTT...")
		solicitarReservaMqtt(placa, ponto)

		// Aguarda resposta por alguns segundos
		resposta := aguardarRespostaMqtt(5 * time.Second)
		if resposta != "" {
			partes := strings.Split(resposta, ",")
			if len(partes) >= 3 && partes[0] == "reserva_confirmada" {
				return partes[2] // Retorna o hash
			}
		}
		fmt.Println("⚠️  Timeout MQTT, tentando via HTTP...")
	}

	// Fallback para HTTP
	transacao := Transacao{
		Tipo:    "RESERVA",
		Placa:   placa,
		Valor:   0.0, // Reserva não tem valor
		Ponto:   ponto,
		Empresa: empresaID,
	}

	jsonData, _ := json.Marshal(transacao)

	// Simula reserva criando transação no blockchain
	resp, err := http.Post(empresasAPI[empresaID]+"/reserva", "application/json", bytes.NewBuffer(jsonData))
	if err != nil || resp.StatusCode != 201 {
		fmt.Printf("❌ Erro HTTP na reserva: %v\n", err)
		return ""
	}

	// Busca o último bloco para obter o hash da reserva
	chain := buscarBlockchain()
	if len(chain.Chain) > 0 {
		ultimoBloco := chain.Chain[len(chain.Chain)-1]
		if ultimoBloco.Transacao.Placa == placa && ultimoBloco.Transacao.Tipo == "RESERVA" && ultimoBloco.Transacao.Ponto == ponto {
			return ultimoBloco.Hash
		}
	}

	return "HASH_SIMULADO_" + ponto + "_" + placa
}

// Verificar hash de transação
func verificarHash(leitor *bufio.Reader) {
	fmt.Println("\n========== Verificar Hash ==========")
	fmt.Print("Digite o hash a ser verificado: ")

	hash, _ := leitor.ReadString('\n')
	hash = strings.TrimSpace(hash)

	if hash == "" {
		fmt.Println("Hash inválido!")
		return
	}

	fmt.Printf("Verificando hash: %s\n", hash)

	// Procura o hash em todas as empresas
	encontrado := false
	for empresaID, api := range empresasAPI {
		if verificarHashEmpresa(hash, empresaID, api) {
			encontrado = true
			break
		}
	}

	if !encontrado {
		fmt.Println("❌ Hash não encontrado no sistema!")
	}
}

// Verificar hash em uma empresa específica
func verificarHashEmpresa(hash, empresaID, api string) bool {
	// Busca blockchain da empresa
	resp, err := http.Get(api + "/blockchain")
	if err != nil {
		return false
	}
	defer resp.Body.Close()

	var chain Blockchain
	json.NewDecoder(resp.Body).Decode(&chain)

	// Procura o hash na blockchain
	for _, bloco := range chain.Chain {
		if bloco.Hash == hash {
			fmt.Printf("✅ Hash encontrado na empresa %s!\n", empresaID)
			fmt.Printf("📄 Detalhes da transação:\n")
			fmt.Printf("   Tipo: %s\n", bloco.Transacao.Tipo)
			fmt.Printf("   Veículo: %s\n", bloco.Transacao.Placa)
			fmt.Printf("   Ponto: %s\n", bloco.Transacao.Ponto)
			fmt.Printf("   Valor: R$ %.2f\n", bloco.Transacao.Valor)
			fmt.Printf("   Data/Hora: %s\n", bloco.Timestamp)
			fmt.Printf("   Empresa: %s\n", bloco.Transacao.Empresa)
			fmt.Printf("   Índice do Bloco: %d\n", bloco.Index)
			return true
		}
	}

	return false
}

// Ver histórico completo do veículo
func verHistoricoCompleto(placa string) {
	fmt.Println("\n========== Histórico Completo ==========")
	fmt.Printf("Veículo: %s\n", placa)

	var todasTransacoes []Bloco
	totalRecargas := 0
	totalPagamentos := 0
	totalReservas := 0
	valorTotalRecargas := 0.0
	valorTotalPagamentos := 0.0

	// Coleta transações de todas as empresas
	for empresaID, api := range empresasAPI {
		resp, err := http.Get(api + "/blockchain")
		if err != nil {
			fmt.Printf("⚠️  Erro ao conectar com empresa %s\n", empresaID)
			continue
		}
		defer resp.Body.Close()

		var chain Blockchain
		json.NewDecoder(resp.Body).Decode(&chain)

		// Filtra transações do veículo
		for _, bloco := range chain.Chain {
			if bloco.Transacao.Placa == placa {
				todasTransacoes = append(todasTransacoes, bloco)

				switch bloco.Transacao.Tipo {
				case "RECARGA":
					totalRecargas++
					valorTotalRecargas += bloco.Transacao.Valor
				case "PAGAMENTO":
					totalPagamentos++
					valorTotalPagamentos += bloco.Transacao.Valor
				case "RESERVA":
					totalReservas++
				}
			}
		}
	}

	if len(todasTransacoes) == 0 {
		fmt.Println("Nenhuma transação encontrada para este veículo.")
		return
	}

	// Ordena por índice do bloco (aproximadamente cronológico)
	for i := 0; i < len(todasTransacoes)-1; i++ {
		for j := 0; j < len(todasTransacoes)-i-1; j++ {
			if todasTransacoes[j].Index > todasTransacoes[j+1].Index {
				todasTransacoes[j], todasTransacoes[j+1] = todasTransacoes[j+1], todasTransacoes[j]
			}
		}
	}

	// Exibe resumo
	fmt.Printf("\n📊 Resumo:\n")
	fmt.Printf("   Total de reservas: %d\n", totalReservas)
	fmt.Printf("   Total de recargas: %d (R$ %.2f)\n", totalRecargas, valorTotalRecargas)
	fmt.Printf("   Total de pagamentos: %d (R$ %.2f)\n", totalPagamentos, valorTotalPagamentos)

	saldoPendente := valorTotalRecargas - valorTotalPagamentos
	if saldoPendente > 0 {
		fmt.Printf("   💰 Saldo pendente: R$ %.2f\n", saldoPendente)
	} else {
		fmt.Printf("   ✅ Todas as recargas foram pagas\n")
	}

	// Exibe histórico detalhado
	fmt.Printf("\n📋 Histórico Detalhado:\n")
	fmt.Println("   Data/Hora          | Tipo      | Ponto        | Empresa | Valor    | Hash")
	fmt.Println("   -------------------|-----------|--------------|---------|----------|------------------")

	for _, bloco := range todasTransacoes {
		tipoIcon := ""
		switch bloco.Transacao.Tipo {
		case "RESERVA":
			tipoIcon = "📅"
		case "RECARGA":
			tipoIcon = "🔋"
		case "PAGAMENTO":
			tipoIcon = "💳"
		}

		valorStr := ""
		if bloco.Transacao.Valor > 0 {
			valorStr = fmt.Sprintf("R$ %.2f", bloco.Transacao.Valor)
		} else {
			valorStr = "-"
		}
		// Hash completo para verificação
		hashCompleto := bloco.Hash

		fmt.Printf("   %s | %s %-7s | %-12s | %-7s | %-8s | %s\n",
			bloco.Timestamp,
			tipoIcon,
			bloco.Transacao.Tipo,
			bloco.Transacao.Ponto,
			bloco.Transacao.Empresa,
			valorStr,
			hashCompleto)
	}

	fmt.Printf("\n📝 Total de transações exibidas: %d\n", len(todasTransacoes))
}

// Ver histórico de viagens específicas do veículo
func verHistoricoViagens(placa string) {
	dados, err := carregarDadosVeiculos()
	if err != nil {
		fmt.Printf("❌ Erro ao carregar dados: %v\n", err)
		return
	}

	veiculo, exists := dados.Veiculos[placa]
	if !exists {
		fmt.Println("❌ Veículo não encontrado!")
		return
	}

	if len(veiculo.Historico) == 0 {
		fmt.Println("📭 Nenhuma viagem registrada ainda.")
		return
	}

	fmt.Println("\n🗂️  ===== Histórico de Viagens =====")
	fmt.Printf("📍 Veículo: %s | 🔋 Autonomia: %.0f km | 📊 Bateria: %.1f%%\n\n",
		veiculo.Placa, veiculo.Autonomia, veiculo.NivelBateriaAtual)

	for i, viagem := range veiculo.Historico {
		var statusIcon string
		switch viagem.Status {
		case "COMPLETA_SEM_RECARGA":
			statusIcon = "✅"
		case "RESERVAS_COMPLETAS":
			statusIcon = "🟢"
		case "RESERVAS_PARCIAIS":
			statusIcon = "🟡"
		default:
			statusIcon = "❓"
		}

		fmt.Printf("%s Viagem #%d\n", statusIcon, i+1)
		fmt.Printf("   📅 Data: %s\n", viagem.Data)
		fmt.Printf("   🚀 %s → %s\n", viagem.Origem, viagem.Destino)

		if len(viagem.Pontos) > 0 {
			fmt.Printf("   🔌 Pontos visitados: %v\n", viagem.Pontos)
		} else {
			fmt.Printf("   🔌 Nenhuma recarga necessária\n")
		}

		fmt.Printf("   📊 Status: %s\n\n", viagem.Status)
	}

	fmt.Printf("📈 Total de viagens: %d\n", len(veiculo.Historico))
}

// Simular viagem sem necessidade de recarga
func simularViagemSemRecarga(placa, origem, destino string) {
	fmt.Println("\n🚗 ========== Simulação da Viagem ==========")
	fmt.Printf("🚀 Iniciando viagem: %s → %s\n", origem, destino)
	fmt.Println("⚡ Bateria suficiente para toda a viagem!")

	// Simula tempo de viagem
	fmt.Println("🛣️  Viajando...")
	time.Sleep(2 * time.Second)

	fmt.Printf("🏁 Chegada em %s concluída com sucesso!\n", destino)
	fmt.Println("🔋 Bateria restante suficiente")
	fmt.Println("✅ Viagem concluída sem necessidade de recarga")
}

// Simular viagem com recargas
func simularViagemComRecargas(placa, origem, destino string, reservasConfirmadas map[string]string, leitor *bufio.Reader) {
	fmt.Println("\n🚗 ========== Simulação da Viagem ==========")
	fmt.Printf("🚀 Iniciando viagem: %s → %s\n", origem, destino)
	fmt.Printf("🔌 %d pontos de recarga reservados\n", len(reservasConfirmadas))

	recargasRealizadas := []RecargaInfo{}

	// Converter map para slice para iteração ordenada
	pontos := make([]string, 0, len(reservasConfirmadas))
	for ponto := range reservasConfirmadas {
		pontos = append(pontos, ponto)
	}

	// Simula viagem com paradas para recarga
	for i, ponto := range pontos {
		fmt.Printf("\n📍 ========== Parada %d/%d ==========\n", i+1, len(pontos))
		fmt.Printf("🛣️  Viajando para %s...\n", ponto)
		time.Sleep(2 * time.Second)

		fmt.Printf("🏁 Chegada em %s\n", ponto)
		fmt.Printf("🔌 Iniciando recarga no ponto reservado...\n")

		// Simular recarga
		empresaID := pontoParaEmpresa[ponto]
		hashReserva := reservasConfirmadas[ponto]
		recargaInfo := realizarRecargaSimulada(placa, ponto, empresaID, hashReserva)
		if recargaInfo.HashRecarga != "" {
			recargasRealizadas = append(recargasRealizadas, recargaInfo)
			fmt.Printf("✅ Recarga concluída em %s!\n", ponto)
			fmt.Printf("🧾 Hash da recarga: %s\n", recargaInfo.HashRecarga)
			fmt.Printf("💰 Valor: R$ %.2f (%.1f kWh)\n", recargaInfo.Valor, recargaInfo.WattsHora)
		} else {
			fmt.Printf("❌ Erro na recarga em %s\n", ponto)
		}

		time.Sleep(1 * time.Second)
	}

	fmt.Printf("\n🏁 Chegada ao destino: %s\n", destino)
	fmt.Println("✅ Viagem concluída com sucesso!")

	// Armazenar recargas pendentes para pagamento posterior
	if len(recargasRealizadas) > 0 {
		recargasPendentesStorage[placa] = append(recargasPendentesStorage[placa], recargasRealizadas...)

		fmt.Printf("\n💳 ========== Resumo Financeiro ==========\n")
		valorTotal := 0.0
		for _, recarga := range recargasRealizadas {
			valorTotal += recarga.Valor
			fmt.Printf("💰 %s: R$ %.2f (%.1f kWh)\n", recarga.Ponto, recarga.Valor, recarga.WattsHora)
		}
		fmt.Printf("💵 Total a pagar: R$ %.2f\n", valorTotal)
		fmt.Println("ℹ️  Use o menu 'Pagar recargas pendentes' para efetuar o pagamento")

		// Perguntar se deseja pagar agora
		fmt.Print("\n❓ Deseja efetuar o pagamento agora? (S/N): ")
		resposta, _ := leitor.ReadString('\n')
		resposta = strings.TrimSpace(strings.ToLower(resposta))

		if resposta == "s" || resposta == "sim" {
			processarPagamentosRecargas(placa)
		}
	}
}

// Realizar recarga simulada com cálculo de valores
func realizarRecargaSimulada(placa, ponto, empresaID, hashReserva string) RecargaInfo {
	fmt.Printf("🔌 Iniciando recarga no ponto %s...\n", ponto)

	// Valores simulados por ponto (kWh fixo para recarga completa e preço por kWh)
	pontosCaracteristicas := map[string]struct {
		KWhCompleto float64 // kWh para recarga completa (100%)
		PrecoPorKWh float64
	}{
		"Salvador":    {KWhCompleto: 50.0, PrecoPorKWh: 0.85},
		"Aracaju":     {KWhCompleto: 50.0, PrecoPorKWh: 0.78},
		"Maceio":      {KWhCompleto: 50.0, PrecoPorKWh: 0.82},
		"Recife":      {KWhCompleto: 50.0, PrecoPorKWh: 0.80},
		"Joao Pessoa": {KWhCompleto: 50.0, PrecoPorKWh: 0.75},
		"Natal":       {KWhCompleto: 50.0, PrecoPorKWh: 0.77},
		"Fortaleza":   {KWhCompleto: 50.0, PrecoPorKWh: 0.84},
		"Teresina":    {KWhCompleto: 50.0, PrecoPorKWh: 0.81},
		"Sao Luis":    {KWhCompleto: 50.0, PrecoPorKWh: 0.79},
	}

	caracteristicas, exists := pontosCaracteristicas[ponto]
	if !exists {
		// Valores padrão - recarga completa sempre 50 kWh
		caracteristicas.KWhCompleto = 50.0
		caracteristicas.PrecoPorKWh = 0.80
	}

	// Simular processo de recarga
	fmt.Println("⚡ Conectando ao ponto de recarga...")
	time.Sleep(1 * time.Second)

	fmt.Println("🔋 Iniciando carregamento...")
	for i := 10; i <= 100; i += 20 {
		fmt.Printf("🔋 Carregando: %d%%\n", i)
		time.Sleep(500 * time.Millisecond)
	}

	fmt.Println("✅ Recarga concluída - Bateria 100%!")

	// AGORA que a recarga foi concluída, calcula e mostra o valor
	valor := caracteristicas.KWhCompleto * caracteristicas.PrecoPorKWh
	fmt.Printf("💰 Valor da recarga: R$ %.2f (%.1f kWh × R$ %.2f/kWh)\n",
		valor, caracteristicas.KWhCompleto, caracteristicas.PrecoPorKWh)

	// Criar transação de recarga
	transacao := Transacao{
		Tipo:    "RECARGA",
		Placa:   placa,
		Valor:   valor,
		Ponto:   ponto,
		Empresa: empresaID,
	}

	// Tentar registrar via HTTP
	jsonData, _ := json.Marshal(transacao)
	resp, err := http.Post(empresasAPI[empresaID]+"/recarga", "application/json", bytes.NewBuffer(jsonData))

	hashRecarga := ""
	if err == nil && resp.StatusCode == 201 {
		// Buscar hash da blockchain
		chain := buscarBlockchain()
		if len(chain.Chain) > 0 {
			// Procurar a transação mais recente
			for i := len(chain.Chain) - 1; i >= 0; i-- {
				bloco := chain.Chain[i]
				if bloco.Transacao.Placa == placa &&
					bloco.Transacao.Tipo == "RECARGA" &&
					bloco.Transacao.Ponto == ponto &&
					bloco.Transacao.Valor == valor {
					hashRecarga = bloco.Hash
					break
				}
			}
		}
	}

	if hashRecarga == "" {
		// Hash simulado se não conseguir registrar
		hashRecarga = fmt.Sprintf("RECARGA_%s_%s_%d", ponto, placa, time.Now().Unix())
	}

	return RecargaInfo{
		Ponto:       ponto,
		Empresa:     empresaID,
		Valor:       valor,
		WattsHora:   caracteristicas.KWhCompleto,
		HashRecarga: hashRecarga,
		Pago:        false,
	}
}

// Processar pagamentos das recargas
func processarPagamentosRecargas(placa string) {
	fmt.Println("\n💳 ========== Processando Pagamentos ==========")

	recargas, exists := recargasPendentesStorage[placa]
	if !exists || len(recargas) == 0 {
		fmt.Println("📭 Nenhuma recarga pendente para pagamento")
		return
	}

	totalPago := 0.0
	pagamentosRealizados := 0

	for i, recarga := range recargas {
		if recarga.Pago {
			continue // Pula recargas já pagas
		}

		fmt.Printf("\n💰 Processando pagamento %d/%d\n", pagamentosRealizados+1, len(recargas))
		fmt.Printf("🔌 Ponto: %s\n", recarga.Ponto)
		fmt.Printf("💵 Valor: R$ %.2f\n", recarga.Valor)

		// Criar transação de pagamento
		transacao := Transacao{
			Tipo:    "PAGAMENTO",
			Placa:   placa,
			Valor:   recarga.Valor,
			Ponto:   recarga.Ponto,
			Empresa: recarga.Empresa,
		}

		// Registrar pagamento
		jsonData, _ := json.Marshal(transacao)
		resp, err := http.Post(empresasAPI[recarga.Empresa]+"/pagamento", "application/json", bytes.NewBuffer(jsonData))

		if err == nil && resp.StatusCode == 201 {
			// Buscar hash do pagamento
			chain := buscarBlockchain()
			hashPagamento := ""
			if len(chain.Chain) > 0 {
				// Procurar a transação mais recente
				for j := len(chain.Chain) - 1; j >= 0; j-- {
					bloco := chain.Chain[j]
					if bloco.Transacao.Placa == placa &&
						bloco.Transacao.Tipo == "PAGAMENTO" &&
						bloco.Transacao.Ponto == recarga.Ponto &&
						bloco.Transacao.Valor == recarga.Valor {
						hashPagamento = bloco.Hash
						break
					}
				}
			}

			if hashPagamento == "" {
				hashPagamento = fmt.Sprintf("PAGAMENTO_%s_%s_%d", recarga.Ponto, placa, time.Now().Unix())
			}
			// Atualizar status da recarga
			recargas[i].Pago = true
			recargas[i].HashPagamento = hashPagamento

			fmt.Printf("✅ Pagamento realizado com sucesso!\n")
			fmt.Printf("🧾 Hash do pagamento: %s\n", hashPagamento)

			totalPago += recarga.Valor
			pagamentosRealizados++
		} else {
			fmt.Printf("❌ Erro ao processar pagamento para %s\n", recarga.Ponto)
		}

		time.Sleep(1 * time.Second)
	}

	// Atualizar storage
	recargasPendentesStorage[placa] = recargas

	fmt.Printf("\n📊 ========== Resumo Final ==========\n")
	fmt.Printf("✅ Pagamentos processados: %d/%d\n", pagamentosRealizados, len(recargas))
	fmt.Printf("💰 Total pago: R$ %.2f\n", totalPago)

	// Verificar se ainda há pendências
	pendentes := 0
	for _, recarga := range recargas {
		if !recarga.Pago {
			pendentes++
		}
	}

	if pendentes > 0 {
		fmt.Printf("⚠️  Recargas ainda pendentes: %d\n", pendentes)
	} else {
		fmt.Println("🎉 Todas as recargas foram pagas!")
		// Limpar storage quando tudo estiver pago
		delete(recargasPendentesStorage, placa)
	}
}

// Fazer reservas atômicas - todos os pontos devem ser reservados ou nenhum
func fazerReservasAtomicas(placa string, pontosNecessarios []string) map[string]string {
	fmt.Println("🔄 Iniciando processo de reserva atômica...")

	// Fase 1: Verificar disponibilidade de todos os pontos
	fmt.Println("📋 Fase 1: Verificando disponibilidade de todos os pontos...")
	pontosDisponiveis := make(map[string]string) // ponto -> empresaID

	for _, ponto := range pontosNecessarios {
		empresaID := pontoParaEmpresa[ponto]
		if empresaID == "" {
			fmt.Printf("❌ Erro: Empresa não encontrada para %s\n", ponto)
			return make(map[string]string) // Retorna vazio se algum ponto não tem empresa
		}
		pontosDisponiveis[ponto] = empresaID
		fmt.Printf("✓ Ponto %s mapeado para empresa %s\n", ponto, empresaID)
	}

	// Fase 2: Tentar reservar todos os pontos simultaneamente
	fmt.Println("📋 Fase 2: Tentando reservar todos os pontos simultaneamente...")
	reservasConfirmadas := make(map[string]string) // ponto -> hash
	reservasFalhas := make([]string, 0)

	// Tenta reservar cada ponto
	for ponto, empresaID := range pontosDisponiveis {
		fmt.Printf("🔄 Reservando %s na empresa %s...\n", ponto, empresaID)
		hash := fazerReservaAtomica(placa, ponto, empresaID)
		if hash != "" {
			reservasConfirmadas[ponto] = hash
			fmt.Printf("✅ Sucesso: %s reservado - Hash: %s\n", ponto, hash)
		} else {
			reservasFalhas = append(reservasFalhas, ponto)
			fmt.Printf("❌ Falha: %s não pôde ser reservado\n", ponto)
		}
	}

	// Fase 3: Verificar se todas as reservas foram bem-sucedidas
	if len(reservasFalhas) > 0 {
		fmt.Printf("❌ Reserva atômica falhou! %d pontos não disponíveis: %v\n", len(reservasFalhas), reservasFalhas)
		fmt.Println("🔄 Cancelando reservas parciais...")

		// Cancela todas as reservas que foram feitas com sucesso
		cancelarReservasParciais(placa, reservasConfirmadas)
		return make(map[string]string) // Retorna vazio
	}

	fmt.Printf("✅ Reserva atômica bem-sucedida! Todos os %d pontos foram reservados!\n", len(reservasConfirmadas))
	return reservasConfirmadas
}

// Cancela reservas que foram feitas parcialmente quando a reserva atômica falha
func cancelarReservasParciais(placa string, reservasParciais map[string]string) {
	if len(reservasParciais) == 0 {
		return
	}

	fmt.Printf("🔄 Cancelando %d reservas parciais...\n", len(reservasParciais))

	// Ativar supressão de mensagens automáticas de cancelamento
	suprimirMensagensCancelamento = true
	defer func() {
		suprimirMensagensCancelamento = false // Reativar mensagens após o cancelamento
	}()

	for ponto, hash := range reservasParciais {
		empresaID := pontoParaEmpresa[ponto]
		if empresaID == "" {
			continue
		}

		fmt.Printf("❌ Cancelando reserva de %s (Hash: %s)...\n", ponto, hash)

		// Tenta cancelar via MQTT primeiro
		if mqttConectado() {
			cancelarReservaMqtt(placa, ponto)
		} else {
			// Fallback para HTTP
			cancelarReservaHTTP(placa, ponto, empresaID)
		}
	}

	fmt.Println("✅ Cancelamento de reservas parciais concluído")
}

// Cancela reserva via MQTT
func cancelarReservaMqtt(placa, ponto string) {
	if mqttClientVeiculo != nil && mqttClientVeiculo.IsConnected() {
		mensagem := fmt.Sprintf("CANCELAR,%s,%s", placa, ponto)
		token := mqttClientVeiculo.Publish("mensagens/cliente", 0, false, mensagem)
		token.Wait()
		fmt.Printf("📡 Cancelamento enviado via MQTT para %s\n", ponto)
	}
}

// Cancela reserva via HTTP
func cancelarReservaHTTP(placa, ponto, empresaID string) {
	type CancelRequest struct {
		PlacaVeiculo string   `json:"placa_veiculo"`
		Pontos       []string `json:"pontos"`
	}

	req := CancelRequest{
		PlacaVeiculo: placa,
		Pontos:       []string{ponto},
	}

	jsonData, _ := json.Marshal(req)
	resp, err := http.Post(empresasAPI[empresaID]+"/cancelamento", "application/json", bytes.NewBuffer(jsonData))

	if err != nil || resp.StatusCode != 200 {
		fmt.Printf("⚠️  Erro ao cancelar via HTTP: %v\n", err)
	} else {
		fmt.Printf("✅ Cancelamento HTTP bem-sucedido para %s\n", ponto)
	}
}

func init() {
	// Tratamento de sinais para desligamento gracioso
	signalChan := make(chan os.Signal, 1)
	signal.Notify(signalChan, syscall.SIGINT, syscall.SIGTERM)

	go func() {
		<-signalChan
		fmt.Println("\n🔌 Desconectando MQTT e encerrando o programa...")
		desconectarMqtt()
		os.Exit(0)
	}()
}
