package main

import (
	"bufio"
	"encoding/json"
	"fmt"
	"math"
	"math/rand"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// Estrutura completa do veículo com dados de bateria e autonomia
type VeiculoCompleto struct {
	Placa             string   `json:"placa"`
	Autonomia         float64  `json:"autonomia"`
	NivelBateriaAtual float64  `json:"bateria_atual"`
	UltimoLogin       string   `json:"ultimo_login"`
	Historico         []Viagem `json:"historico,omitempty"`
}

// Estrutura para representar uma viagem
type Viagem struct {
	Data    string   `json:"data"`
	Origem  string   `json:"origem"`
	Destino string   `json:"destino"`
	Pontos  []string `json:"pontos_visitados"`
	Status  string   `json:"status"`
}

// Estrutura para pontos de recarga
type PontoRecarga struct {
	ID        int     `json:"id"`
	Cidade    string  `json:"cidade"`
	Estado    string  `json:"estado"`
	Latitude  float64 `json:"latitude"`
	Longitude float64 `json:"longitude"`
	EmpresaID string  `json:"empresa_id"`
}

// Estrutura para dados completos dos veículos
type DadosVeiculosCompletos struct {
	Veiculos map[string]VeiculoCompleto `json:"veiculos"`
}

// Estrutura para controle de sessões ativas
type SessaoAtiva struct {
	Placa        string `json:"placa"`
	ClienteID    string `json:"cliente_id"`
	HorarioLogin string `json:"horario_login"`
	ProcessID    int    `json:"process_id"`
}

// Estrutura para gerenciar sessões ativas
type ControleSessoes struct {
	SessoesAtivas map[string]SessaoAtiva `json:"sessoes_ativas"`
}

// Mapa de cidades para IDs
var cidadeParaID = map[string]string{
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

// Mapa de pontos para empresas (baseado na localização geográfica)
var pontoParaEmpresa = map[string]string{
	"Salvador":    "001", // N-Sul
	"Aracaju":     "001", // N-Sul
	"Maceio":      "001", // N-Sul
	"Recife":      "002", // N-Centro
	"Joao Pessoa": "002", // N-Centro
	"Natal":       "002", // N-Centro (CORRIGIDO: era 003)
	"Fortaleza":   "003", // N-Norte
	"Teresina":    "003", // N-Norte
	"Sao Luis":    "003", // N-Norte
}

// Dados dos pontos de recarga com coordenadas
var pontosDeRecarga = []PontoRecarga{
	{1, "Salvador", "BA", -12.9714, -38.5014, "001"},
	{2, "Aracaju", "SE", -10.9472, -37.0731, "001"},
	{3, "Maceio", "AL", -9.6658, -35.7353, "001"},
	{4, "Recife", "PE", -8.0476, -34.8770, "002"},
	{5, "Joao Pessoa", "PB", -7.1195, -34.8450, "002"},
	{6, "Natal", "RN", -5.7945, -35.2110, "002"},
	{7, "Fortaleza", "CE", -3.7172, -38.5434, "003"},
	{8, "Teresina", "PI", -5.0892, -42.8019, "003"},
	{9, "Sao Luis", "MA", -2.5387, -44.2825, "003"},
}

// Gera dados iniciais aleatórios para o veículo
func gerarDadosIniciais() (float64, float64) {
	rand.Seed(time.Now().UnixNano())
	bateria := 10 + rand.Float64()*80     // 10% a 90%
	autonomia := 500 + rand.Float64()*200 // 500km a 700km
	return bateria, autonomia
}

// Carrega dados completos dos veículos
// Carrega dados completos dos veículos do arquivo JSON com múltiplas tentativas de caminho
func carregarDadosVeiculos() (DadosVeiculosCompletos, error) {
	var dados DadosVeiculosCompletos

	// Tenta primeiro o caminho do container, depois caminho local
	caminhos := []string{
		"/app/data/veiculos_completos.json",       // Caminho no container
		"../empresa/data/veiculos_completos.json", // Caminho local (relativo)
	}

	var file *os.File
	var err error

	for _, caminho := range caminhos {
		file, err = os.Open(caminho)
		if err == nil {
			break // Arquivo encontrado
		}
	}

	if err != nil {
		// Se não existe em nenhum local, cria estrutura vazia
		dados.Veiculos = make(map[string]VeiculoCompleto)
		return dados, nil
	}
	defer file.Close()

	decoder := json.NewDecoder(file)
	err = decoder.Decode(&dados)
	if err != nil {
		dados.Veiculos = make(map[string]VeiculoCompleto)
	}

	return dados, nil
}

// Salva dados completos dos veículos
// Salva dados dos veículos no arquivo JSON com múltiplas tentativas de caminho
func salvarDadosVeiculos(dados DadosVeiculosCompletos) error {
	// Tenta primeiro o caminho do container, depois caminho local
	caminhos := []string{
		"/app/data/veiculos_completos.json",       // Caminho no container
		"../empresa/data/veiculos_completos.json", // Caminho local (relativo)
	}

	var err error
	for _, caminho := range caminhos {
		// Verifica se o diretório existe
		dir := filepath.Dir(caminho)
		if _, err := os.Stat(dir); os.IsNotExist(err) {
			continue // Diretório não existe, tenta próximo caminho
		}

		file, err := os.Create(caminho)
		if err != nil {
			continue // Erro ao criar arquivo, tenta próximo caminho
		}
		defer file.Close()

		encoder := json.NewEncoder(file)
		encoder.SetIndent("", "  ")
		return encoder.Encode(dados)
	}

	return fmt.Errorf("não foi possível salvar dados de veículos em nenhum local: %v", err)
}

// Carrega controle de sessões ativas
func carregarSessoesAtivas() (ControleSessoes, error) {
	var controle ControleSessoes
	controle.SessoesAtivas = make(map[string]SessaoAtiva) // Inicializa o map sempre

	file, err := os.ReadFile("data/sessoes_ativas.json")
	if err != nil {
		if os.IsNotExist(err) {
			return controle, nil
		}
		return controle, err
	}

	err = json.Unmarshal(file, &controle)
	if err != nil {
		// Se falhou o unmarshalling, retorna o controle com map inicializado
		return controle, nil
	}

	// Garante que o map não é nil mesmo após unmarshalling
	if controle.SessoesAtivas == nil {
		controle.SessoesAtivas = make(map[string]SessaoAtiva)
	}

	return controle, nil
}

// Salva controle de sessões ativas
func salvarSessoesAtivas(controle ControleSessoes) error {
	file, err := json.MarshalIndent(controle, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile("data/sessoes_ativas.json", file, 0644)
}

// Verifica se uma placa já está ativa no sistema
// Verifica se placa já está sendo usada em sessão ativa para evitar login duplo
func verificarPlacaAtiva(placa string) (bool, string) {
	controle, err := carregarSessoesAtivas()
	if err != nil {
		return false, ""
	}

	if sessao, existe := controle.SessoesAtivas[placa]; existe {
		return true, fmt.Sprintf("❌ Placa %s já está sendo usada desde %s (Cliente: %s)",
			placa, sessao.HorarioLogin, sessao.ClienteID)
	}

	return false, ""
}

// Registra uma nova sessão ativa
// Registra nova sessão ativa e gera ID único para cliente MQTT
func registrarSessaoAtiva(placa string) (string, error) {
	controle, err := carregarSessoesAtivas()
	if err != nil {
		return "", err
	}

	// Gera um ID único para esta sessão
	clienteID := fmt.Sprintf("veiculo_%s_%d", placa, time.Now().Unix())

	sessao := SessaoAtiva{
		Placa:        placa,
		ClienteID:    clienteID,
		HorarioLogin: time.Now().Format("15:04:05 02/01/2006"),
		ProcessID:    os.Getpid(),
	}

	controle.SessoesAtivas[placa] = sessao
	err = salvarSessoesAtivas(controle)
	if err != nil {
		return "", err
	}

	fmt.Printf("✅ Sessão registrada: %s\n", clienteID)
	return clienteID, nil
}

// Remove uma sessão ativa
// Remove sessão ativa do veículo durante logout ou limpeza do sistema
func removerSessaoAtiva(placa string) error {
	controle, err := carregarSessoesAtivas()
	if err != nil {
		return err
	}

	if _, existe := controle.SessoesAtivas[placa]; existe {
		delete(controle.SessoesAtivas, placa)
		err = salvarSessoesAtivas(controle)
		if err == nil {
			fmt.Printf("✅ Sessão da placa %s removida com sucesso\n", placa)
		}
	}

	return err
}

// Verifica se a placa existe e faz login ou cadastro
// Gerencia login de veículo existente ou cadastro de novo veículo no sistema
func loginOuCadastro(placa string) (VeiculoCompleto, bool, error) {
	dados, err := carregarDadosVeiculos()
	if err != nil {
		return VeiculoCompleto{}, false, err
	}

	// Verifica se veículo já existe (login)
	if veiculo, exists := dados.Veiculos[placa]; exists {
		veiculo.UltimoLogin = time.Now().Format("15:04:05 02/01/2006")
		dados.Veiculos[placa] = veiculo
		salvarDadosVeiculos(dados)

		fmt.Printf("🚗 Bem-vindo de volta, veículo %s!\n", placa)
		fmt.Printf("📊 Bateria atual: %.1f%%\n", veiculo.NivelBateriaAtual)
		fmt.Printf("🔋 Autonomia: %.0f km\n", veiculo.Autonomia)
		fmt.Printf("📅 Último login: %s\n", veiculo.UltimoLogin)

		return veiculo, true, nil // true = login
	}

	// Novo veículo (cadastro)
	bateria, autonomia := gerarDadosIniciais()
	novoVeiculo := VeiculoCompleto{
		Placa:             placa,
		Autonomia:         autonomia,
		NivelBateriaAtual: bateria,
		UltimoLogin:       time.Now().Format("15:04:05 02/01/2006"),
		Historico:         []Viagem{},
	}

	dados.Veiculos[placa] = novoVeiculo
	salvarDadosVeiculos(dados)

	fmt.Printf("✅ Veículo %s cadastrado com sucesso!\n", placa)
	fmt.Printf("📊 Bateria inicial: %.1f%%\n", bateria)
	fmt.Printf("🔋 Autonomia: %.0f km\n", autonomia)

	return novoVeiculo, false, nil // false = cadastro
}

// Salva uma viagem no histórico
// Registra viagem no histórico do veículo com informações de pontos visitados e status
func salvarViagem(placa string, origem, destino string, pontos []string, status string) error {
	dados, err := carregarDadosVeiculos()
	if err != nil {
		return err
	}

	veiculo := dados.Veiculos[placa]
	novaViagem := Viagem{
		Data:    time.Now().Format("15:04:05 02/01/2006"),
		Origem:  origem,
		Destino: destino,
		Pontos:  pontos,
		Status:  status,
	}

	veiculo.Historico = append(veiculo.Historico, novaViagem)
	dados.Veiculos[placa] = veiculo

	return salvarDadosVeiculos(dados)
}

// Calcula distância entre dois pontos usando fórmula de Haversine
func calcularDistancia(lat1, lon1, lat2, lon2 float64) float64 {
	const R = 6371 // Raio da Terra em km

	// Converte graus para radianos
	lat1Rad := lat1 * math.Pi / 180
	lon1Rad := lon1 * math.Pi / 180
	lat2Rad := lat2 * math.Pi / 180
	lon2Rad := lon2 * math.Pi / 180

	// Diferenças
	deltaLat := lat2Rad - lat1Rad
	deltaLon := lon2Rad - lon1Rad

	// Fórmula de Haversine
	a := math.Sin(deltaLat/2)*math.Sin(deltaLat/2) +
		math.Cos(lat1Rad)*math.Cos(lat2Rad)*
			math.Sin(deltaLon/2)*math.Sin(deltaLon/2)
	c := 2 * math.Atan2(math.Sqrt(a), math.Sqrt(1-a))

	return R * c
}

// Obter ponto por cidade
func obterPontoPorCidade(cidade string) *PontoRecarga {
	for _, ponto := range pontosDeRecarga {
		if ponto.Cidade == cidade {
			return &ponto
		}
	}
	return nil
}

// Obtém pontos de recarga por lista de cidades
func obterPontosPorCidades(cidades []string) []PontoRecarga {
	var pontos []PontoRecarga
	for _, cidade := range cidades {
		if ponto := obterPontoPorCidade(cidade); ponto != nil {
			pontos = append(pontos, *ponto)
		}
	}
	return pontos
}

// Calcula a distância total de uma rota
// Calcula distância total da rota somando distâncias entre cidades consecutivas
func calcularDistanciaTotal(rota []string) float64 {
	pontosRota := obterPontosPorCidades(rota)
	if len(pontosRota) < 2 {
		return 0.0
	}

	distanciaTotal := 0.0
	for i := 0; i < len(pontosRota)-1; i++ {
		pontoAtual := pontosRota[i]
		proximoPonto := pontosRota[i+1]

		distancia := calcularDistancia(
			pontoAtual.Latitude, pontoAtual.Longitude,
			proximoPonto.Latitude, proximoPonto.Longitude,
		)
		distanciaTotal += distancia
	}

	return distanciaTotal
}

// Seleção de cidade com interface melhorada
// Interface para seleção de cidade (origem/destino) com validação de entrada
func selecionarCidade(tipo string, leitor *bufio.Reader) string {
	for {
		fmt.Printf("\n======= Cidades com Serviço de Recarga =======\n")
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

		fmt.Printf("Selecione a cidade de %s: ", tipo)
		opcao, _ := leitor.ReadString('\n')
		opcao = strings.TrimSpace(opcao)

		switch opcao {
		case "1", "2", "3", "4", "5", "6", "7", "8", "9":
			return opcao
		case "0":
			return ""
		default:
			fmt.Println("❌ Opção inválida. Tente novamente!")
		}
	}
}

// Calcula rota entre origem e destino
// Calcula rota de viagem entre origem e destino incluindo cidades intermediárias
func calcularRotaViagem(origem, destino string) []string {
	// Rota principal do Nordeste em ordem geográfica
	rotaCompleta := []string{"Salvador", "Aracaju", "Maceio", "Recife", "Joao Pessoa", "Natal", "Fortaleza", "Teresina", "Sao Luis"}

	origemCidade := cidadeParaID[origem]
	destinoCidade := cidadeParaID[destino]

	// Encontrar índices das cidades
	var indiceOrigem, indiceDestino int = -1, -1
	for i, cidade := range rotaCompleta {
		if cidade == origemCidade {
			indiceOrigem = i
		}
		if cidade == destinoCidade {
			indiceDestino = i
		}
	}

	if indiceOrigem == -1 || indiceDestino == -1 {
		return []string{origemCidade, destinoCidade}
	}

	// Calcular rota
	var rota []string
	if indiceOrigem <= indiceDestino {
		rota = rotaCompleta[indiceOrigem : indiceDestino+1]
	} else {
		// Rota inversa
		for i := indiceOrigem; i >= indiceDestino; i-- {
			rota = append(rota, rotaCompleta[i])
		}
	}

	return rota
}

// Calcula pontos onde é necessário recarregar baseado na autonomia e distâncias reais
// Determina pontos de recarga necessários com base na autonomia do veículo e distâncias
func calcularPontosRecarga(rota []string, veiculo *VeiculoCompleto) []string {
	var pontosNecessarios []string
	bateriaAtual := veiculo.NivelBateriaAtual
	autonomiaTotal := veiculo.Autonomia

	// Obtém pontos de recarga para as cidades da rota
	pontosRota := obterPontosPorCidades(rota)

	for i := 0; i < len(pontosRota)-1; i++ {
		pontoAtual := pontosRota[i]
		proximoPonto := pontosRota[i+1]

		// Calcula distância real entre os pontos usando coordenadas
		distanciaProximo := calcularDistancia(
			pontoAtual.Latitude, pontoAtual.Longitude,
			proximoPonto.Latitude, proximoPonto.Longitude,
		)

		// Calcula autonomia restante em km
		autonomiaRestante := (bateriaAtual / 100) * autonomiaTotal

		// Se não conseguir chegar na próxima cidade, precisa recarregar
		if autonomiaRestante < distanciaProximo {
			pontosNecessarios = append(pontosNecessarios, pontoAtual.Cidade)
			bateriaAtual = 95.0 // Recarga para 95%
			autonomiaRestante = (bateriaAtual / 100) * autonomiaTotal
			// fmt.Printf("🔌 Recarga necessária em %s! Nova bateria: %.1f%%\n", cidadeAnterior, bateriaAtual)
		}

		// Consome bateria para chegar na próxima cidade
		percentualConsumido := (distanciaProximo / autonomiaTotal) * 100
		bateriaAtual -= percentualConsumido
	}

	return pontosNecessarios
}

// Remove duplicatas de uma slice
// func removerDuplicatas(slice []string) []string {
// 	keys := make(map[string]bool)
// 	var result []string

// 	for _, item := range slice {
// 		if !keys[item] {
// 			keys[item] = true
// 			result = append(result, item)
// 		}
// 	}
// 	return result
// }
