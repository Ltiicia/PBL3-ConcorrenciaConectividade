package main

import (
	"bufio"
	"encoding/json"
	"fmt"
	"math/rand"
	"os"
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
	"Maceio":      "002", // N-Centro
	"Recife":      "002", // N-Centro
	"Joao Pessoa": "002", // N-Centro
	"Natal":       "003", // N-Norte
	"Fortaleza":   "003", // N-Norte
	"Teresina":    "003", // N-Norte
	"Sao Luis":    "003", // N-Norte
}

// Dados dos pontos de recarga com coordenadas
var pontosDeRecarga = []PontoRecarga{
	{1, "Salvador", "BA", -12.9714, -38.5014, "001"},
	{2, "Aracaju", "SE", -10.9472, -37.0731, "001"},
	{3, "Maceio", "AL", -9.6658, -35.7353, "002"},
	{4, "Recife", "PE", -8.0476, -34.8770, "002"},
	{5, "Joao Pessoa", "PB", -7.1195, -34.8450, "002"},
	{6, "Natal", "RN", -5.7945, -35.2110, "003"},
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
func carregarDadosVeiculos() (DadosVeiculosCompletos, error) {
	var dados DadosVeiculosCompletos

	file, err := os.Open("/app/data/veiculos_completos.json")
	if err != nil {
		// Se não existe, cria estrutura vazia
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
func salvarDadosVeiculos(dados DadosVeiculosCompletos) error {
	file, err := os.Create("/app/data/veiculos_completos.json")
	if err != nil {
		return err
	}
	defer file.Close()

	encoder := json.NewEncoder(file)
	encoder.SetIndent("", "  ")
	return encoder.Encode(dados)
}

// Verifica se a placa existe e faz login ou cadastro
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

	// Simplified distance calculation using absolute difference
	return R * 2 * (3.14159265359 / 180) * (lat1 - lat2)
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

// Seleção de cidade com interface melhorada
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

// Calcula pontos onde é necessário recarregar baseado na autonomia
func calcularPontosRecarga(rota []string, veiculo *VeiculoCompleto) []string {
	var pontosNecessarios []string
	bateriaAtual := veiculo.NivelBateriaAtual
	autonomiaTotal := veiculo.Autonomia

	fmt.Printf("\n🔋 Simulação da viagem para o veículo [%s]:\n", veiculo.Placa)
	fmt.Printf("   Bateria inicial: %.1f%% (%.1f km de autonomia)\n", bateriaAtual, (bateriaAtual/100)*autonomiaTotal)

	for i, cidade := range rota {
		if i == 0 {
			fmt.Printf("🚗 Partindo de %s - Bateria: %.1f%%\n", cidade, bateriaAtual)
			continue
		}

		// Distância aproximada entre cidades consecutivas (200km em média)
		distanciaProximo := 200.0
		if i == len(rota)-1 {
			distanciaProximo = 150.0 // Última cidade mais próxima
		}

		// Calcula autonomia restante em km
		autonomiaRestante := (bateriaAtual / 100) * autonomiaTotal

		// Se não conseguir chegar na próxima cidade, precisa recarregar
		if autonomiaRestante < distanciaProximo {
			cidadeAnterior := rota[i-1]
			pontosNecessarios = append(pontosNecessarios, cidadeAnterior)
			bateriaAtual = 95.0 // Recarga para 95%
			autonomiaRestante = (bateriaAtual / 100) * autonomiaTotal
			fmt.Printf("🔌 Recarga necessária em %s! Nova bateria: %.1f%%\n", cidadeAnterior, bateriaAtual)
		}

		// Consome bateria para chegar na próxima cidade
		percentualConsumido := (distanciaProximo / autonomiaTotal) * 100
		bateriaAtual -= percentualConsumido

		if i == len(rota)-1 {
			fmt.Printf("🏁 Chegada em %s - Bateria final: %.1f%%\n", cidade, bateriaAtual)
		} else {
			fmt.Printf("📍 Passando por %s - Bateria: %.1f%%\n", cidade, bateriaAtual)
		}
	}

	return pontosNecessarios
}

// Remove duplicatas de uma slice
func removerDuplicatas(slice []string) []string {
	keys := make(map[string]bool)
	var result []string

	for _, item := range slice {
		if !keys[item] {
			keys[item] = true
			result = append(result, item)
		}
	}
	return result
}
