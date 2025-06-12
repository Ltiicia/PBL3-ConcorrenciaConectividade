package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"strings"
	"sync"
	"time"
)

// Estruturas para comunicação REST
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

type StatusResponse struct {
	Status     string                 `json:"status"`
	EmpresaID  string                 `json:"empresa_id"`
	Blockchain map[string]interface{} `json:"blockchain_info"`
}

type VerificacaoHashRequest struct {
	Hash string `json:"hash"`
}

type VerificacaoHashResponse struct {
	Encontrado bool                   `json:"encontrado"`
	EmpresaID  string                 `json:"empresa_id"`
	Bloco      map[string]interface{} `json:"bloco"`
	Mensagem   string                 `json:"mensagem"`
}

// Controle de concorrência para reservas
var reservas_mutex sync.Mutex
var reservas = make(map[string]map[string]string)

// Status dos pontos de recarga
var status_ponto = struct {
	sync.RWMutex
	status map[string]bool
}{status: make(map[string]bool)}

var ponto_locks = make(map[string]*sync.Mutex)

// Configuração de rede (adaptável para Docker)
var servidores = []string{
	"http://empresa_001:8001",
	"http://empresa_002:8002",
	"http://empresa_003:8003",
}

// Estruturas para controle de pontos
type PontoStatus struct {
	Placa            string `json:"placa"`
	TimestampReserva string `json:"timestamp_reserva"`
	Status           string `json:"status"`
	HashReserva      string `json:"hash_reserva"`
}

type ControlePontos struct {
	sync.RWMutex
	pontos map[string]PontoStatus
}

var controlePontos = ControlePontos{
	pontos: make(map[string]PontoStatus),
}

// Inicializa o servidor REST com todos os endpoints
func inicializaREST() {
	// Endpoints principais
	http.HandleFunc("/blockchain", blockchainHandler)
	http.HandleFunc("/bloco", receberBlocoHandler)
	http.HandleFunc("/reserva", reservaHandler)
	http.HandleFunc("/recarga", recargaHandler)
	http.HandleFunc("/pagamento", pagamentoHandler)
	http.HandleFunc("/sincronizar", sincronizarHandler)

	// Novos endpoints para integração completa
	http.HandleFunc("/api/status", handleStatus)
	http.HandleFunc("/api/verificar-hash", handleVerificarHash)
	http.HandleFunc("/api/historico", handleHistorico)
	http.HandleFunc("/api/reservas", handleReservasCoordnadas)
	http.HandleFunc("/api/cancelamento", handleCancelamento)
	http.HandleFunc("/api/pontos/status", handleStatusPontos)
	// Inicializa controle de pontos
	inicializaControlePontos()

	fmt.Printf("[REST] Handlers registrados para empresa %s\n", empresa.ID)
}

// Inicializa o controle de status dos pontos
func inicializaControlePontos() {
	// Inicializa locks para cada ponto
	for _, ponto := range empresa.Pontos {
		ponto_locks[ponto] = &sync.Mutex{}
		// Define pontos da própria empresa como ativos
		status_ponto.Lock()
		status_ponto.status[ponto] = true
		status_ponto.Unlock()
	}

	// Carrega o controle de pontos do arquivo
	err := carregarControlePontos()
	if err != nil {
		fmt.Printf("[ERRO] Falha ao carregar controle de pontos: %v\n", err)
	}

	// Inicia monitoramento periódico
	go monitorarPontos()
}

// Monitora periodicamente o status dos pontos
func monitorarPontos() {
	for {
		time.Sleep(30 * time.Second)
		verificarStatusPontos()
	}
}

// Verifica o status de todos os pontos
func verificarStatusPontos() {
	for _, ponto := range empresa.Pontos {
		// Simula verificação de conectividade
		// Em implementação real, faria ping ou verificação de rede
		status := true // Por enquanto, sempre online

		status_ponto.Lock()
		statusAnterior := status_ponto.status[ponto]
		status_ponto.status[ponto] = status
		status_ponto.Unlock()

		if statusAnterior != status {
			if status {
				fmt.Printf("[PONTOS] Ponto %s voltou online\n", ponto)
			} else {
				fmt.Printf("[PONTOS] Ponto %s está offline\n", ponto)
				cancelarReservasPontoOffline(ponto)
			}
		}
	}
}

// Cancela reservas de pontos offline
func cancelarReservasPontoOffline(ponto string) {
	reservas_mutex.Lock()
	defer reservas_mutex.Unlock()

	for placa, pontosMap := range reservas {
		if _, reservado := pontosMap[ponto]; reservado {
			delete(pontosMap, ponto)
			fmt.Printf("[PONTOS] Reserva cancelada para %s no ponto %s (offline)\n", placa, ponto)

			// Notifica via MQTT se disponível
			if mqttClient != nil && mqttClient.IsConnected() {
				mensagem := fmt.Sprintf("reserva_cancelada,%s,Ponto offline", ponto)
				publicaMensagemMqtt(mqttClient, "mensagens/cliente/"+placa, mensagem)
			}
		}
	}
}

// Handler para status da empresa
func handleStatus(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	response := StatusResponse{
		Status:    "online",
		EmpresaID: empresa.ID,
		Blockchain: map[string]interface{}{
			"total_blocos": len(blockchain.Chain),
			"ultimo_hash":  "",
		},
	}

	if len(blockchain.Chain) > 0 {
		response.Blockchain["ultimo_hash"] = blockchain.Chain[len(blockchain.Chain)-1].Hash
	}

	json.NewEncoder(w).Encode(response)
}

// Handler para verificar hash específico
func handleVerificarHash(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Método não permitido", http.StatusMethodNotAllowed)
		return
	}

	var req VerificacaoHashRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Erro ao decodificar JSON", http.StatusBadRequest)
		return
	}

	w.Header().Set("Content-Type", "application/json")

	// Procura o hash na blockchain local
	for _, bloco := range blockchain.Chain {
		if bloco.Hash == req.Hash {
			response := VerificacaoHashResponse{
				Encontrado: true,
				EmpresaID:  empresa.ID,
				Bloco: map[string]interface{}{
					"index":         bloco.Index,
					"timestamp":     bloco.Timestamp,
					"tipo":          bloco.Transacao.Tipo,
					"placa":         bloco.Transacao.Placa,
					"ponto":         bloco.Transacao.Ponto,
					"valor":         bloco.Transacao.Valor,
					"empresa":       bloco.Transacao.Empresa,
					"hash":          bloco.Hash,
					"hash_anterior": bloco.HashAnterior,
					"autor":         bloco.Autor,
				},
				Mensagem: "Hash encontrado na blockchain",
			}
			json.NewEncoder(w).Encode(response)
			return
		}
	}

	// Hash não encontrado
	response := VerificacaoHashResponse{
		Encontrado: false,
		EmpresaID:  empresa.ID,
		Mensagem:   "Hash não encontrado nesta empresa",
	}
	json.NewEncoder(w).Encode(response)
}

// Handler para histórico de um veículo
func handleHistorico(w http.ResponseWriter, r *http.Request) {
	placa := r.URL.Query().Get("placa")
	if placa == "" {
		http.Error(w, "Parâmetro 'placa' obrigatório", http.StatusBadRequest)
		return
	}

	w.Header().Set("Content-Type", "application/json")

	var transacoes []map[string]interface{}
	for _, bloco := range blockchain.Chain {
		if bloco.Transacao.Placa == placa {
			transacao := map[string]interface{}{
				"index":     bloco.Index,
				"timestamp": bloco.Timestamp,
				"tipo":      bloco.Transacao.Tipo,
				"ponto":     bloco.Transacao.Ponto,
				"valor":     bloco.Transacao.Valor,
				"empresa":   bloco.Transacao.Empresa,
				"hash":      bloco.Hash,
			}
			transacoes = append(transacoes, transacao)
		}
	}

	response := map[string]interface{}{
		"placa":      placa,
		"empresa_id": empresa.ID,
		"transacoes": transacoes,
		"total":      len(transacoes),
	}

	json.NewEncoder(w).Encode(response)
}

// Handler para reservas coordenadas entre empresas
func handleReservasCoordnadas(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Método não permitido", http.StatusMethodNotAllowed)
		return
	}

	var req ReservaRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Erro ao decodificar JSON", http.StatusBadRequest)
		return
	}

	w.Header().Set("Content-Type", "application/json")

	// Processa reservas em pontos desta empresa
	var respostasLocais []ReservaResponse
	var pontosOutrasEmpresas []string

	for _, ponto := range req.Pontos {
		// Verifica se o ponto pertence a esta empresa
		pertenceEmpresa := false
		for _, pontoDaEmpresa := range empresa.Pontos {
			if ponto == pontoDaEmpresa {
				pertenceEmpresa = true
				break
			}
		}

		if pertenceEmpresa {
			// Processa reserva local
			hash := processarReservaLocal(req.PlacaVeiculo, ponto)
			if hash != "" {
				respostasLocais = append(respostasLocais, ReservaResponse{
					Status:    "confirmado",
					Ponto:     ponto,
					Mensagem:  "Reserva confirmada",
					EmpresaID: empresa.ID,
					Hash:      hash,
				})
			} else {
				respostasLocais = append(respostasLocais, ReservaResponse{
					Status:    "falha",
					Ponto:     ponto,
					Mensagem:  "Erro ao processar reserva",
					EmpresaID: empresa.ID,
				})
			}
		} else {
			pontosOutrasEmpresas = append(pontosOutrasEmpresas, ponto)
		}
	}

	// Se há pontos de outras empresas, coordena com elas
	var respostasExternas []ReservaResponse
	if len(pontosOutrasEmpresas) > 0 {
		respostasExternas = coordenarReservasExternas(req.PlacaVeiculo, pontosOutrasEmpresas, req.EmpresaID)
	}

	// Combina todas as respostas
	todasRespostas := append(respostasLocais, respostasExternas...)

	response := map[string]interface{}{
		"placa":    req.PlacaVeiculo,
		"reservas": todasRespostas,
		"total":    len(todasRespostas),
		"sucesso":  len(todasRespostas) > 0,
	}

	json.NewEncoder(w).Encode(response)
}

// Processa reserva local na empresa
func processarReservaLocal(placa, ponto string) string {
	// Cria transação de reserva
	transacao := Transacao{
		Tipo:    "RESERVA",
		Placa:   placa,
		Valor:   0.0,
		Ponto:   ponto,
		Empresa: empresa.ID,
	}

	mutex.Lock()
	defer mutex.Unlock()

	ultimo := blockchain.Chain[len(blockchain.Chain)-1]
	hash := CalcularHash(Bloco{
		Index:        ultimo.Index + 1,
		Timestamp:    formatarTimestamp(time.Now().UTC().Format(time.RFC3339)),
		Transacao:    transacao,
		HashAnterior: ultimo.Hash,
		Autor:        empresa.ID,
	})

	assinatura, err := AssinarBloco(hash, chave_privada_path)
	if err != nil {
		return ""
	}

	novo_bloco := NovoBloco(transacao, ultimo, empresa.ID, assinatura)
	blockchain.Chain = append(blockchain.Chain, novo_bloco)
	SalvarBlockchain("data/chain_"+empresa.ID+".json", blockchain)

	// Registra reserva
	reservas_mutex.Lock()
	if _, existe := reservas[placa]; !existe {
		reservas[placa] = make(map[string]string)
	}
	reservas[placa][ponto] = "confirmado"
	reservas_mutex.Unlock()

	propagarBloco(novo_bloco)

	// Marca ponto como reservado
	marcarPontoReservado(ponto, placa)

	return novo_bloco.Hash
}

// Coordena reservas com outras empresas
func coordenarReservasExternas(placa string, pontos []string, empresaOrigem string) []ReservaResponse {
	var respostas []ReservaResponse

	for _, servidor := range servidores {
		// Não envia para si mesmo
		if strings.Contains(servidor, empresa.ID) {
			continue
		}

		req := ReservaRequest{
			PlacaVeiculo: placa,
			Pontos:       pontos,
			EmpresaID:    empresaOrigem,
		}

		jsonData, _ := json.Marshal(req)
		resp, err := http.Post(servidor+"/reserva", "application/json", bytes.NewBuffer(jsonData))
		if err != nil {
			continue
		}
		defer resp.Body.Close()

		if resp.StatusCode == http.StatusCreated {
			var response ReservaResponse
			json.NewDecoder(resp.Body).Decode(&response)
			respostas = append(respostas, response)
		}
	}

	return respostas
}

// Handler para cancelamento de reservas com controle PBL2
func handleCancelamento(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Método não permitido", http.StatusMethodNotAllowed)
		return
	}

	var req ReservaRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Erro ao decodificar JSON", http.StatusBadRequest)
		return
	}

	w.Header().Set("Content-Type", "application/json")

	placa := req.PlacaVeiculo
	cancelados := 0

	fmt.Printf("[HTTP] Processando cancelamento para %s (pontos: %v)\n", placa, req.Pontos)

	// PBL2 CONCURRENCY: Process each point with individual locks
	for _, ponto := range req.Pontos {
		// PBL2 CONCURRENCY: Acquire per-point lock
		lock := ponto_locks[ponto]
		lock.Lock()

		fmt.Printf("[HTTP] Verificando cancelamento de %s no ponto %s\n", placa, ponto)

		// Check if this point is reserved by this vehicle
		reservas_mutex.Lock()
		pontoReservado := false
		if pontosMap, existe := reservas[placa]; existe {
			if _, reservado := pontosMap[ponto]; reservado {
				pontoReservado = true
			}
		}
		reservas_mutex.Unlock()

		if pontoReservado {
			// PBL2 CONCURRENCY: Release point completely within lock
			liberarPontoCompleto(ponto, placa)
			cancelados++
			fmt.Printf("[HTTP] Reserva cancelada para %s no ponto %s\n", placa, ponto)
		} else {
			fmt.Printf("[HTTP] Ponto %s não estava reservado para %s\n", ponto, placa)
		}

		lock.Unlock()
	}

	response := map[string]interface{}{
		"placa":      placa,
		"cancelados": cancelados,
		"status":     "success",
		"empresa_id": empresa.ID,
	}

	json.NewEncoder(w).Encode(response)
}

// Handler para status dos pontos
func handleStatusPontos(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	status_ponto.RLock()
	pontosStatus := make(map[string]bool)
	for ponto, status := range status_ponto.status {
		pontosStatus[ponto] = status
	}
	status_ponto.RUnlock()

	response := map[string]interface{}{
		"empresa_id": empresa.ID,
		"pontos":     pontosStatus,
		"timestamp":  time.Now().Format(time.RFC3339),
	}

	json.NewEncoder(w).Encode(response)
}

// Utilitário para fazer requisições REST para outras empresas
func requisicaoRest(metodo, url string, corpo interface{}, resposta interface{}) error {
	jsonCorpo, err := json.Marshal(corpo)
	if err != nil {
		return fmt.Errorf("erro na codificação JSON: %v", err)
	}

	req, err := http.NewRequest(metodo, url, bytes.NewBuffer(jsonCorpo))
	if err != nil {
		return fmt.Errorf("erro na criação da requisição: %v", err)
	}
	req.Header.Set("Content-Type", "application/json")

	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("erro na execução da requisição: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("status de resposta inválido: %d", resp.StatusCode)
	}

	if resposta != nil {
		if err := json.NewDecoder(resp.Body).Decode(resposta); err != nil {
			return fmt.Errorf("erro na decodificação da resposta: %v", err)
		}
	}

	return nil
}

// Carrega o controle de pontos do arquivo
func carregarControlePontos() error {
	controlePontos.Lock()
	defer controlePontos.Unlock()

	fileName := fmt.Sprintf("data/controle_pontos_%s.json", empresa.ID)
	file, err := os.ReadFile(fileName)
	if err != nil {
		if os.IsNotExist(err) {
			// Arquivo não existe, inicializa com pontos vazios
			controlePontos.pontos = make(map[string]PontoStatus)
			return nil
		}
		return err
	}

	return json.Unmarshal(file, &controlePontos.pontos)
}

// Salva o controle de pontos no arquivo
func salvarControlePontos() error {
	controlePontos.RLock()
	defer controlePontos.RUnlock()

	fileName := fmt.Sprintf("data/controle_pontos_%s.json", empresa.ID)
	file, err := json.MarshalIndent(controlePontos.pontos, "", "  ")
	if err != nil {
		return err
	}

	return os.WriteFile(fileName, file, 0644)
}

// Verifica se um ponto está disponível para reserva
func verificarPontoDisponivel(ponto, placa string) bool {
	controlePontos.RLock()
	defer controlePontos.RUnlock()

	status, existe := controlePontos.pontos[ponto]
	if !existe {
		// Ponto não está no controle, significa que está livre
		return true
	}

	// Se já está reservado para a mesma placa, permite
	if status.Placa == placa && status.Status == "RESERVADO" {
		return true
	}

	// Se está reservado para outra placa, não permite
	if status.Status == "RESERVADO" && status.Placa != placa {
		fmt.Printf("[CONTROLE] Ponto %s já reservado por %s\n", ponto, status.Placa)
		return false
	}

	return true
}

// Marca um ponto como reservado
func marcarPontoReservado(ponto, placa string) bool {
	controlePontos.Lock()
	defer controlePontos.Unlock()

	// Verifica novamente dentro do lock para garantir atomicidade
	status, existe := controlePontos.pontos[ponto]
	if existe && status.Status == "RESERVADO" && status.Placa != placa {
		return false
	}

	// Marca como reservado
	controlePontos.pontos[ponto] = PontoStatus{
		Placa:            placa,
		TimestampReserva: time.Now().Format(time.RFC3339),
		Status:           "RESERVADO",
		HashReserva:      "", // Será preenchido quando a transação for criada
	}

	// Salva no arquivo
	err := salvarControlePontosInterno()
	if err != nil {
		fmt.Printf("[ERRO] Falha ao salvar controle de pontos: %v\n", err)
		return false
	}

	fmt.Printf("[CONTROLE] Ponto %s reservado para %s\n", ponto, placa)
	return true
}

// Atualiza o hash da reserva após criar a transação
func atualizarHashReserva(ponto, placa, hash string) {
	controlePontos.Lock()
	defer controlePontos.Unlock()

	if status, existe := controlePontos.pontos[ponto]; existe && status.Placa == placa {
		status.HashReserva = hash
		controlePontos.pontos[ponto] = status
		salvarControlePontosInterno()
	}
}

// Libera um ponto reservado
func liberarPonto(ponto, placa string) {
	controlePontos.Lock()
	defer controlePontos.Unlock()

	if status, existe := controlePontos.pontos[ponto]; existe && status.Placa == placa {
		delete(controlePontos.pontos, ponto)
		salvarControlePontosInterno()
		fmt.Printf("[CONTROLE] Ponto %s liberado por %s\n", ponto, placa)
	}
}

// Função interna para salvar sem lock (deve ser chamada dentro de um lock)
func salvarControlePontosInterno() error {
	fileName := fmt.Sprintf("data/controle_pontos_%s.json", empresa.ID)
	file, err := json.MarshalIndent(controlePontos.pontos, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(fileName, file, 0644)
}
