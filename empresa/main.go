package main

import (
	"bytes"
	"crypto"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"crypto/x509"
	"encoding/hex"
	"encoding/json"
	"encoding/pem"
	"fmt"
	"log"
	"net/http"
	"os"
	"strconv"
	"sync"
	"time"
)

var mutex sync.Mutex
var processar_transacoes = make(chan Bloco, 100)

type Empresa struct {
	ID         string          `json:"id"`
	Nome       string          `json:"nome"`
	API        string          `json:"api"`
	SaldoAtual float64         `json:"saldo_atual"`
	Placas     map[string]bool `json:"placas"`
	Pontos     []string        `json:"pontos"`
}

type Veiculos struct {
	Placas map[string]bool `json:"placas"`
}

type Transacao struct {
	Tipo    string  `json:"tipo"`
	Placa   string  `json:"placa"`
	Valor   float64 `json:"valor"`
	Ponto   string  `json:"ponto"`
	Empresa string  `json:"empresa"`
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

type Blockchain struct {
	Chain []Bloco `json:"blocos"`
}

var (
	empresa            Empresa
	blockchain         Blockchain
	chave_privada_path string
	chave_publica_path string
	empresasAPI        = map[string]string{
		"001": "http://empresa_001:8001",
		"002": "http://empresa_002:8002",
		"003": "http://empresa_003:8003",
	}
)

// Calcula hash SHA256 de um bloco para garantir integridade na blockchain
func CalcularHash(bloco Bloco) string {
	index := strconv.Itoa(bloco.Index)
	valor := fmt.Sprintf("%.2f", bloco.Transacao.Valor)
	dados := index + bloco.Timestamp + bloco.Transacao.Tipo + bloco.Transacao.Placa + valor + bloco.Transacao.Ponto + bloco.Transacao.Empresa + bloco.HashAnterior + bloco.Autor
	hash := sha256.Sum256([]byte(dados))
	return hex.EncodeToString(hash[:])
}

// Cria novo bloco na blockchain com transação, referência ao bloco anterior e assinatura
func NovoBloco(transacao Transacao, bloco_anterior Bloco, autor string, assinatura string) Bloco {
	prox_index := bloco_anterior.Index + 1
	timestamp := formatarTimestamp(time.Now().Format(time.RFC3339))
	novo_bloco := Bloco{
		Index:        prox_index,
		Timestamp:    timestamp,
		Transacao:    transacao,
		HashAnterior: bloco_anterior.Hash,
		Autor:        autor,
		Assinatura:   assinatura,
	}
	novo_bloco.Hash = CalcularHash(novo_bloco)
	return novo_bloco
}

// Valida integridade de novo bloco verificando hash e referência ao bloco anterior
func ValidarBloco(novo_bloco, bloco_anterior Bloco) bool {
	if bloco_anterior.Index+1 != novo_bloco.Index {
		return false
	}
	if bloco_anterior.Hash != novo_bloco.HashAnterior {
		return false
	}
	if CalcularHash(novo_bloco) != novo_bloco.Hash {
		return false
	}
	return true
}

// Carrega blockchain existente do arquivo JSON ou cria blockchain vazia
func CarregarBlockchain(path string) (Blockchain, error) {
	var chain Blockchain
	file, erro := os.ReadFile(path)
	if erro != nil {
		return chain, erro
	}
	erro = json.Unmarshal(file, &chain)
	return chain, erro
}

// Salva blockchain completa no arquivo JSON
func SalvarBlockchain(path string, chain Blockchain) error {
	data, erro := json.MarshalIndent(chain, "", "  ")
	if erro != nil {
		return erro
	}
	return os.WriteFile(path, data, 0644)
}

// Gera par de chaves RSA (privada/pública) para assinatura digital dos blocos
func GerarChaves(privada_path, publica_path string) error {
	privada, erro := rsa.GenerateKey(rand.Reader, 2048)
	if erro != nil {
		return erro
	}
	privada_bytes := x509.MarshalPKCS1PrivateKey(privada)
	privada_pem := pem.EncodeToMemory(&pem.Block{Type: "RSA PRIVATE KEY", Bytes: privada_bytes})
	erro = os.WriteFile(privada_path, privada_pem, 0600)
	if erro != nil {
		return erro
	}
	publica_bytes := x509.MarshalPKCS1PublicKey(&privada.PublicKey)
	pub_pem := pem.EncodeToMemory(&pem.Block{Type: "RSA PUBLIC KEY", Bytes: publica_bytes})
	return os.WriteFile(publica_path, pub_pem, 0644)
}

// assina o bloco com a chave privada da empresa
// Assina hash do bloco usando chave privada RSA para autenticação
func AssinarBloco(hash string, privada_path string) (string, error) {
	privada_pem, erro := os.ReadFile(privada_path)
	if erro != nil {
		return "", erro
	}
	block, _ := pem.Decode(privada_pem)
	privada, erro := x509.ParsePKCS1PrivateKey(block.Bytes)
	if erro != nil {
		return "", erro
	}
	hash_bytes := []byte(hash)
	hash_sum := sha256.Sum256(hash_bytes)
	assinatura, erro := rsa.SignPKCS1v15(rand.Reader, privada, crypto.SHA256, hash_sum[:])
	if erro != nil {
		return "", erro
	}
	return hex.EncodeToString(assinatura), nil
}

// valida a assinatura do bloco usando a chave publica da empresa
// Valida assinatura digital do bloco usando chave pública RSA
func ValidarAssinatura(hash, assinatura, publica_path string) bool {
	pub_pem, erro := os.ReadFile(publica_path)
	if erro != nil {
		return false
	}
	block, _ := pem.Decode(pub_pem)
	publica, erro := x509.ParsePKCS1PublicKey(block.Bytes)
	if erro != nil {
		return false
	}
	assinatura_decode, erro := hex.DecodeString(assinatura)
	if erro != nil {
		return false
	}
	hash_bytes := []byte(hash)
	hash_sum := sha256.Sum256(hash_bytes)
	erro = rsa.VerifyPKCS1v15(publica, crypto.SHA256, hash_sum[:], assinatura_decode)
	return erro == nil
}

// Configura rotas HTTP da API REST da empresa
func inicializarAPI() {
	empresa_id := os.Getenv("EMPRESA_ID")
	if empresa_id == "" {
		log.Fatal("EMPRESA_ID indefinido")
	}

	// Carrega dados da empresa
	empresa_path := "data/empresa_" + empresa_id + ".json"
	file, erro := os.ReadFile(empresa_path)
	if erro != nil {
		log.Fatalf("Erro ao carregar empresa: %v", erro)
	}
	if erro := json.Unmarshal(file, &empresa); erro != nil {
		log.Fatalf("Erro ao decodificar empresa: %v", erro)
	}

	// Define caminhos das chaves
	chave_privada_path = "data/empresa_" + empresa_id + "_private.pem"
	chave_publica_path = "data/empresa_" + empresa_id + "_public.pem"

	// Gera chaves se ainda nao existirem
	if _, erro := os.Stat(chave_privada_path); os.IsNotExist(erro) {
		log.Println("Gerando par de chaves...")
		if erro := GerarChaves(chave_privada_path, chave_publica_path); erro != nil {
			log.Fatalf("Erro ao gerar chaves: %v", erro)
		}
	}

	// Carrega blockchain
	chain_path := "data/chain_" + empresa_id + ".json"
	if _, erro := os.Stat(chain_path); os.IsNotExist(erro) {
		// Cria arquivo o vazio se ainda nao existir
		blockchain = Blockchain{Chain: []Bloco{}}
		SalvarBlockchain(chain_path, blockchain)
	}

	blockchain, erro = CarregarBlockchain(chain_path)
	if erro != nil {
		log.Fatalf("Erro ao carregar blockchain: %v", erro)
	}
	// Cria bloco genesis
	if len(blockchain.Chain) == 0 {
		blocoGenesis := Bloco{
			Index:        0,
			Timestamp:    "2025-01-01T00:00:00Z",
			Transacao:    Transacao{Tipo: "GENESIS", Empresa: "GENESIS"},
			HashAnterior: "",
			Autor:        "GENESIS",
			Assinatura:   "",
		}
		blocoGenesis.Hash = CalcularHash(blocoGenesis)
		blockchain.Chain = append(blockchain.Chain, blocoGenesis)
		SalvarBlockchain(chain_path, blockchain)
	}
}

// Aguarda outras empresas ficarem disponíveis para sincronização da blockchain
func aguardarEmpresasDisponiveis() {
	ids := []string{"001", "002", "003"}
	for {
		todasOk := true
		for _, id := range ids {
			if id == empresa.ID {
				continue
			}
			api := empresasAPI[id]
			resp, err := http.Get(api + "/blockchain")
			if err != nil || resp.StatusCode != 200 {
				todasOk = false
				fmt.Printf("[LOG] Empresa %s ainda não está disponível. Tentando novamente...\n", id)
				break
			}
			resp.Body.Close()
		}
		if todasOk {
			fmt.Println("Todas as empresas estão disponíveis")
			break
		}
		time.Sleep(1 * time.Second)
	}
}

// exibe blockchain
// Handler HTTP para retornar blockchain completa da empresa
func blockchainHandler(writer http.ResponseWriter, r *http.Request) {
	writer.Header().Set("Content-Type", "application/json")
	json.NewEncoder(writer).Encode(blockchain)
}

// processa transacoes em sequencia
// Inicia processador de blocos em goroutine para validação e adição à blockchain
func iniciarProcessadorDeBlocos() {
	go func() {
		for bloco := range processar_transacoes {
			mutex.Lock()
			if blocoDuplicado(bloco) {
				fmt.Printf("Bloco duplicado detectado - index [%d] hash (%s). Rejeitando...\n", bloco.Index, bloco.Hash)
				mutex.Unlock()
				continue
			}
			ultimo := blockchain.Chain[len(blockchain.Chain)-1]
			chave_publica_path := "data/empresa_" + bloco.Autor + "_public.pem"
			if ValidarBloco(bloco, ultimo) && ValidarAssinatura(bloco.Hash, bloco.Assinatura, chave_publica_path) {
				blockchain.Chain = append(blockchain.Chain, bloco)
				SalvarBlockchain("data/chain_"+empresa.ID+".json", blockchain)
				fmt.Printf("Bloco da empresa %s ACEITO index [%d]\n", bloco.Autor, bloco.Index)
			} else {
				fmt.Printf("Bloco da empresa %s REJEITADO index [%d]\n", bloco.Autor, bloco.Index)
			}
			mutex.Unlock()
		}
	}()
}

func blocoDuplicado(bloco Bloco) bool {
	for _, b := range blockchain.Chain {
		if b.Index == bloco.Index || b.Hash == bloco.Hash {
			return true
		}
	}
	return false
}

// recebe bloco de outra empresa
func receberBlocoHandler(writer http.ResponseWriter, request *http.Request) {
	var bloco Bloco
	if erro := json.NewDecoder(request.Body).Decode(&bloco); erro != nil {
		fmt.Println("Erro ao decodificar bloco recebido:", erro)
		writer.WriteHeader(http.StatusBadRequest)
		return
	}
	// Enfileira o bloco recebidp no canal para processar em sequencia
	processar_transacoes <- bloco
	writer.WriteHeader(http.StatusAccepted)
}

// sincronizacao inicial
func sincronizarHandler(writer http.ResponseWriter, request *http.Request) {
	var chain_remota Blockchain
	if erro := json.NewDecoder(request.Body).Decode(&chain_remota); erro != nil {
		fmt.Println("Erro ao decodificar blockchain recebida:", erro)
		writer.WriteHeader(http.StatusBadRequest)
		return
	}
	if len(chain_remota.Chain) > len(blockchain.Chain) && validarBlockchainCompleta(chain_remota) {
		blockchain = chain_remota
		SalvarBlockchain("data/chain_"+empresa.ID+".json", blockchain)
		fmt.Println("Blockchain local atualizada via sincronização inicial.")
		writer.WriteHeader(http.StatusOK)
	} else {
		fmt.Println("Blockchain recebida ignorada - (inválida).")
		writer.WriteHeader(http.StatusForbidden)
	}
}

// validaa a blockchain recebida completa
func validarBlockchainCompleta(chain Blockchain) bool {
	for i := 1; i < len(chain.Chain); i++ {
		anterior := chain.Chain[i-1]
		atual := chain.Chain[i]
		pub_path := "data/empresa_" + atual.Autor + "_public.pem"
		if !ValidarBloco(atual, anterior) || !ValidarAssinatura(atual.Hash, atual.Assinatura, pub_path) {
			fmt.Printf("Falha ao validar bloco index [%d] da blockchain\n", atual.Index)
			return false
		}
	}
	return true
}

func tentarCorrigirBlockchainCorrompida(chain_path string) bool {
	for id, api := range empresasAPI {
		if id == empresa.ID {
			continue
		}
		fmt.Printf("Buscando blockchain válida da empresa %s para correção...\n", id)
		resp, err := http.Get(api + "/blockchain")
		if err != nil {
			fmt.Printf("[LOG] Erro ao buscar blockchain de %s: %v\n", id, err)
			continue
		}
		defer resp.Body.Close()
		var chain_remota Blockchain
		if err := json.NewDecoder(resp.Body).Decode(&chain_remota); err != nil {
			fmt.Printf("Erro ao decodificar blockchain de %s: %v\n", id, err)
			continue
		}
		if validarBlockchainCompleta(chain_remota) {
			fmt.Printf("Blockchain válida encontrada na empresa %s. Corrigindo\n", id)
			SalvarBlockchain(chain_path, chain_remota)
			return true
		}
	}
	return false
}

func formatarTimestamp(data_hora string) string {
	data, erro := time.Parse(time.RFC3339, data_hora)
	if erro != nil {
		return data_hora
	}
	return data.Format("15:04:05 02/01/2006")
}

func sincronizarComOutrasEmpresas() {
	fmt.Println("Sincronizando blockchain...")
	max_tentativas := 3
	for tentativa := 1; tentativa <= max_tentativas; tentativa++ {
		sincronizou := false
		for id, api := range empresasAPI {
			if id == empresa.ID {
				continue
			}
			resp, erro := http.Get(api + "/blockchain")
			if erro != nil {
				continue
			}
			defer resp.Body.Close()
			var chain_remota Blockchain
			if erro := json.NewDecoder(resp.Body).Decode(&chain_remota); erro != nil {
				continue
			}
			if len(chain_remota.Chain) == len(blockchain.Chain) && len(chain_remota.Chain) > 0 {
				hashLocal := blockchain.Chain[len(blockchain.Chain)-1].Hash
				hashRemoto := chain_remota.Chain[len(chain_remota.Chain)-1].Hash
				if hashLocal == hashRemoto {
					continue
				}
			}
			if len(chain_remota.Chain) > len(blockchain.Chain) && validarBlockchainCompleta(chain_remota) {
				blockchain = chain_remota
				SalvarBlockchain("data/chain_"+empresa.ID+".json", blockchain)
				fmt.Printf("Blockchain sincronizada com a da empresa %s.\n", id)
				sincronizou = true
			}
		}
		if sincronizou {
			break
		}
		if tentativa < max_tentativas {
			time.Sleep(10 * time.Second)
		}
	}
	fmt.Println("Sincronização concluída")
}

// Handler para recarga
func recargaHandler(writer http.ResponseWriter, request *http.Request) {
	var transacao Transacao
	if erro := json.NewDecoder(request.Body).Decode(&transacao); erro != nil {
		writer.WriteHeader(http.StatusBadRequest)
		return
	}
	mutex.Lock()
	defer mutex.Unlock()
	ultimo := blockchain.Chain[len(blockchain.Chain)-1]
	hash := CalcularHash(Bloco{
		Index:        (ultimo.Index + 1),
		Timestamp:    formatarTimestamp(time.Now().Format(time.RFC3339)),
		Transacao:    transacao,
		HashAnterior: ultimo.Hash,
		Autor:        empresa.ID,
	})
	assinatura, erro := AssinarBloco(hash, chave_privada_path)
	if erro != nil {
		writer.WriteHeader(http.StatusInternalServerError)
		return
	}
	novo_bloco := NovoBloco(transacao, ultimo, empresa.ID, assinatura)
	if blocoDuplicado(novo_bloco) {
		fmt.Printf("Bloco duplicado detectado - index [%d] hash (%s). Rejeitando...\n", novo_bloco.Index, novo_bloco.Hash)
		writer.WriteHeader(http.StatusConflict)
		return
	}
	if !propagarBlocoComConsenso(novo_bloco) {
		fmt.Println("Consenso não atingido. Bloco REJEITADO")
		writer.WriteHeader(http.StatusPreconditionFailed)
		return
	}
	blockchain.Chain = append(blockchain.Chain, novo_bloco)
	SalvarBlockchain("data/chain_"+empresa.ID+".json", blockchain)

	// Libera automaticamente o ponto após recarga completa
	liberarPontoCompleto(transacao.Ponto, transacao.Placa)

	fmt.Printf("[HTTP] Recarga processada e ponto %s liberado para %s - Hash: %s\n",
		transacao.Ponto, transacao.Placa, novo_bloco.Hash)
	fmt.Println("Consenso atingido. Bloco ADICIONADO")
	writer.WriteHeader(http.StatusCreated)
}

func pagamentoHandler(writer http.ResponseWriter, r *http.Request) {
	var transacao Transacao
	if erro := json.NewDecoder(r.Body).Decode(&transacao); erro != nil {
		writer.WriteHeader(http.StatusBadRequest)
		return
	}
	mutex.Lock()
	defer mutex.Unlock()
	ultimo := blockchain.Chain[len(blockchain.Chain)-1]
	hash := CalcularHash(Bloco{
		Index:        (ultimo.Index + 1),
		Timestamp:    formatarTimestamp(time.Now().Format(time.RFC3339)),
		Transacao:    transacao,
		HashAnterior: ultimo.Hash,
		Autor:        empresa.ID,
	})
	assinatura, erro := AssinarBloco(hash, chave_privada_path)
	if erro != nil {
		writer.WriteHeader(http.StatusInternalServerError)
		return
	}
	novo_bloco := NovoBloco(transacao, ultimo, empresa.ID, assinatura)
	if blocoDuplicado(novo_bloco) {
		fmt.Printf("Bloco duplicado detectado - index [%d] hash (%s). Rejeitando...\n", novo_bloco.Index, novo_bloco.Hash)
		writer.WriteHeader(http.StatusConflict)
		return
	}
	if !propagarBlocoComConsenso(novo_bloco) {
		fmt.Println("Consenso não atingido. Bloco REJEITADO")
		writer.WriteHeader(http.StatusPreconditionFailed)
		return
	}
	blockchain.Chain = append(blockchain.Chain, novo_bloco)
	SalvarBlockchain("data/chain_"+empresa.ID+".json", blockchain)
	if transacao.Tipo == "PAGAMENTO" && transacao.Empresa == empresa.ID {
		// Atualiza o saldo da empresa se for pagamento
		empresa.SaldoAtual += transacao.Valor
		empresa_path := "data/empresa_" + empresa.ID + ".json"
		data, _ := json.MarshalIndent(empresa, "", "  ")
		os.WriteFile(empresa_path, data, 0644)
	}
	fmt.Println("Consenso atingido. Bloco ADICIONADO")
	writer.WriteHeader(http.StatusCreated)
}

// Handler para reserva com controle de concorrência PBL2
func reservaHandler(writer http.ResponseWriter, request *http.Request) {
	var transacao Transacao
	if erro := json.NewDecoder(request.Body).Decode(&transacao); erro != nil {
		writer.WriteHeader(http.StatusBadRequest)
		return
	}

	// Verifica se o ponto pertence a esta empresa
	pontoValido := false
	for _, ponto := range empresa.Pontos {
		if ponto == transacao.Ponto {
			pontoValido = true
			break
		}
	}

	if !pontoValido {
		writer.WriteHeader(http.StatusNotFound)
		return
	}

	ponto := transacao.Ponto
	placa := transacao.Placa

	// PBL2 CONCURRENCY: Acquire per-point lock before any operations
	lock := ponto_locks[ponto]
	lock.Lock()
	defer lock.Unlock()

	fmt.Printf("[HTTP] Processando reserva para %s no ponto %s\n", placa, ponto)

	// PBL2 CONCURRENCY: Check point availability within lock
	if !marcarPontoReservado(ponto, placa) {
		fmt.Printf("[HTTP] Ponto %s não disponível para %s\n", ponto, placa)
		response := map[string]string{
			"status":  "error",
			"message": fmt.Sprintf("Ponto %s não está disponível para reserva", ponto),
		}
		writer.Header().Set("Content-Type", "application/json")
		writer.WriteHeader(http.StatusConflict)
		json.NewEncoder(writer).Encode(response)
		return
	}

	// PBL2 CONCURRENCY: Process blockchain transaction within lock
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
		// PBL2 CONCURRENCY: Rollback reservation on error
		liberarPontoCompleto(ponto, placa)
		fmt.Printf("[HTTP] Erro ao assinar bloco para %s: %v\n", placa, erro)
		writer.WriteHeader(http.StatusInternalServerError)
		return
	}

	novo_bloco := NovoBloco(transacao, ultimo, empresa.ID, assinatura)
	if blocoDuplicado(novo_bloco) {
		mutex.Unlock()
		// PBL2 CONCURRENCY: Rollback reservation on duplicate
		liberarPontoCompleto(ponto, placa)
		fmt.Printf("[HTTP] Bloco duplicado detectado para %s - index [%d] hash (%s). Rejeitando...\n", placa, novo_bloco.Index, novo_bloco.Hash)
		writer.WriteHeader(http.StatusConflict)
		return
	}

	blockchain.Chain = append(blockchain.Chain, novo_bloco)
	SalvarBlockchain("data/chain_"+empresa.ID+".json", blockchain)
	mutex.Unlock()
	// PBL2 CONCURRENCY: Update reservation memory within lock
	reservas_mutex.Lock()
	if _, existe := reservas[placa]; !existe {
		reservas[placa] = make(map[string]string)
	}
	reservas[placa][ponto] = "confirmado"
	reservas_mutex.Unlock()

	// Update hash in point control
	atualizarHashReserva(ponto, placa, novo_bloco.Hash)

	// Propagate block
	propagarBlocoComConsenso(novo_bloco)

	// PBL2 CONCURRENCY: Start timeout system
	go liberaPorTimeout(placa, []string{ponto}, 5*time.Minute)

	fmt.Printf("[HTTP] Reserva confirmada para %s no ponto %s (Hash: %s)\n", placa, ponto, novo_bloco.Hash)

	// Retorna o hash da reserva para o cliente
	response := map[string]string{
		"status":  "success",
		"hash":    novo_bloco.Hash,
		"message": fmt.Sprintf("Reserva confirmada para %s no ponto %s", placa, ponto),
	}
	writer.Header().Set("Content-Type", "application/json")
	json.NewEncoder(writer).Encode(response)
	writer.WriteHeader(http.StatusCreated)
}

func propagarBloco(bloco Bloco) {
	jsonBloco, _ := json.Marshal(bloco)
	for id, api := range empresasAPI {
		if id == empresa.ID {
			continue
		}
		fmt.Printf("Enviando bloco para empresa %s...\n", id)
		resp, err := http.Post(api+"/bloco", "application/json", bytes.NewBuffer(jsonBloco))
		if err != nil {
			fmt.Printf("Erro ao enviar bloco para %s: %v\n", id, err)
			continue
		}
		resp.Body.Close()
	}
}

func propagarBlocoComConsenso(bloco Bloco) bool {
	sucesso := true
	jsonBloco, _ := json.Marshal(bloco)

	for id, api := range empresasAPI {
		if id == empresa.ID {
			continue
		}
		fmt.Printf("Enviando bloco para consenso na empresa %s...\n", id)
		resp, err := http.Post(api+"/bloco", "application/json", bytes.NewBuffer(jsonBloco))
		if err != nil {
			fmt.Printf("Erro ao enviar bloco para empresa %s: %v\n", id, err)
			sucesso = false
			break
		}
		if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusAccepted {
			fmt.Printf("Empresa %s rejeitou o bloco (status %d)\n", id, resp.StatusCode)
			sucesso = false
			break
		}
		fmt.Printf("Empresa %s aceitou o bloco.\n", id)
	}
	return sucesso
}

func RecargasPendentes(placa string, chain Blockchain) []Transacao {
	var recargas []Transacao
	var pagamentos []Transacao
	//identifica o tipo das transacoes
	for _, bloco := range chain.Chain {
		if bloco.Transacao.Placa == placa {
			if bloco.Transacao.Tipo == "RECARGA" {
				recargas = append(recargas, bloco.Transacao)
			} else if bloco.Transacao.Tipo == "PAGAMENTO" {
				pagamentos = append(pagamentos, bloco.Transacao)
			}
		}
	}
	//identifica as recargas que nao tem pagamento correspondente
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
	return pendentes
}

// Função principal da empresa - inicializa servidor HTTP, MQTT e processamento de transações
func main() {
	inicializarAPI() // carrega empresa, blockchain, chaves, bloco gênese
	iniciarProcessadorDeBlocos()

	// Inicializa os handlers REST ANTES de subir o servidor
	inicializaREST()

	//sobe a api
	go func() {
		porta := ":8" + empresa.ID
		log.Printf("Empresa %s [%s] iniciada na porta %s", empresa.Nome, empresa.ID, porta)
		log.Fatal(http.ListenAndServe(porta, nil))
	}()

	go func() {
		time.Sleep(1 * time.Second) // pequena espera para garantir que a API subiu
		aguardarEmpresasDisponiveis()
		chain_path := "data/chain_" + empresa.ID + ".json"
		if !validarBlockchainCompleta(blockchain) {
			fmt.Println("Blockchain corrompida! Corrigindo...")
			if !tentarCorrigirBlockchainCorrompida(chain_path) {
				log.Fatalf("Blockchain inválida e não foi possível corrigir com as outras empresas. Arquivo %s", chain_path)
			}
			var erro error
			blockchain, erro = CarregarBlockchain(chain_path)
			if erro != nil {
				log.Fatalf("Erro ao recarregar blockchain corrigida: %v", erro)
			}
			if !validarBlockchainCompleta(blockchain) {
				log.Fatalf("Blockchain ainda inválida após tentativa de correção. Arquivo %s", chain_path)
			}
			fmt.Println("Blockchain corrigida com sucesso!")
		}
		sincronizarComOutrasEmpresas()
		fmt.Println("Sincronização concluída")
	}()

	sincronizarComOutrasEmpresas() // Inicia sistemas de comunicação
	inicializaMqtt(empresa.ID)

	// Mantém o programa em execução
	select {}
}
