package main

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"time"
)

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

func main() {
	fmt.Println("Olá! Para iniciar informe a placa do seu veiculo...")
	leitor := bufio.NewReader(os.Stdin)
	placa_validada := false
	for !placa_validada {
		fmt.Print("Placa: ")
		placa, _ := leitor.ReadString('\n')
		placa = strings.TrimSpace(placa)

		if placa == "" {
			fmt.Println("Placa inválida")
		}

		if !cadastrarPlaca(placa) {
			fmt.Println("Placa já está em uso")
			continue
		}
		placa_validada = true
		placa_veiculo = placa
		fmt.Printf("Placa %s registrada\n", placa)
	}

	for {
		fmt.Println("\n ============== Menu ==============")
		fmt.Println("1 - Realizar recarga")
		fmt.Println("2 - Pagar recargas pendentes")
		fmt.Println("3 - Consultar extrato")
		fmt.Println("0 - Sair")
		fmt.Print("Selecione uma opção: ")

		opcao, _ := leitor.ReadString('\n')
		opcao = strings.TrimSpace(opcao)
		switch opcao {
		case "1":
			realizarRecarga(placa_veiculo, leitor)
		case "2":
			pagarRecargasPendentes(placa_veiculo)
		case "3":
			verExtrato(placa_veiculo)
		case "0":
			removerPlaca(placa_veiculo)
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
	fmt.Printf("Empresa responsável pelo ponto %s: %s\n", ponto, empresa_id)
	if empresa_id == "" {
		fmt.Println("Empresa não encontrada para o ponto!")
		return
	}
	transacao := Transacao{
		Tipo:    "RECARGA",
		Placa:   placa,
		Valor:   valor,
		Ponto:   ponto,
		Empresa: empresa_id,
	}
	json_data, _ := json.Marshal(transacao)
	fmt.Printf("Enviando recarga para %s: %s\n", empresasAPI[empresa_id]+"/recarga", string(json_data))
	resp, err := http.Post(empresasAPI[empresa_id]+"/recarga", "application/json", bytes.NewBuffer(json_data))
	if err != nil || resp.StatusCode != 201 {
		fmt.Printf("Erro ao registrar recarga: %v, status: %v\n", err, resp)
		return
	}
	fmt.Println("Recarga registrada com sucesso!")
}

func pagarRecargasPendentes(placa string) {
	chain := buscarBlockchain()
	pendentes := recargasPendentes(placa, chain)
	fmt.Printf("Recargas pendentes localizadas: %d\n", len(pendentes))
	if len(pendentes) == 0 {
		return
	}
	for _, rec := range pendentes {
		fmt.Printf("Gerando pagamento para recarga em %s - empresa (%s) valor: R$ %.2f\n", rec.Ponto, rec.Empresa, rec.Valor)
		transacao := Transacao{
			Tipo:    "PAGAMENTO",
			Placa:   placa,
			Valor:   rec.Valor,
			Ponto:   rec.Ponto,
			Empresa: rec.Empresa,
		}
		json_data, _ := json.Marshal(transacao)
		fmt.Printf("Enviando pagamento para %s: %s\n", empresasAPI[rec.Empresa]+"/pagamento", string(json_data))
		resp, err := http.Post(empresasAPI[rec.Empresa]+"/pagamento", "application/json", bytes.NewBuffer(json_data))
		if err != nil || resp.StatusCode != 201 {
			fmt.Printf("Erro ao pagar recarga em %s!\n", rec.Ponto)
			fmt.Printf("[LOG] Erro: %v, status: %v\n", err, resp)
			continue
		}
		fmt.Printf("Pagamento realizado para recarga em %s!\n", rec.Ponto)
	}
}

func formatarTimestamp(data_hora string) string {
	data, erro := time.Parse(time.RFC3339, data_hora)
	if erro != nil {
		return data_hora
	}
	return data.Format("15:04:05 02/01/2006")
}

func verExtrato(placa string) {
	chain := buscarBlockchain()
	fmt.Println("\nExtrato de transações:")
	for _, bloco := range chain.Chain {
		if bloco.Transacao.Placa == placa {
			fmt.Printf("%s | %s | %s | %s | R$ %.2f\n", formatarTimestamp(bloco.Timestamp), bloco.Transacao.Tipo, bloco.Transacao.Ponto, bloco.Transacao.Empresa, bloco.Transacao.Valor)
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
	pontosEmpresa := map[string][]string{
		"001": {"Salvador", "Aracaju", "Maceio"},
		"002": {"Recife", "Joao Pessoa", "Natal"},
		"003": {"Fortaleza", "Teresina", "Sao Luis"},
	}
	for id, pontos := range pontosEmpresa {
		for _, p := range pontos {
			if p == ponto {
				return id
			}
		}
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
