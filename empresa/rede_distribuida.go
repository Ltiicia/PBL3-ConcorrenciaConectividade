package main

import (
	"fmt"
	"os"
	"strings"
)

// Configuração de rede para execução distribuída
// Inspirado no modelo PBL2 onde cada servidor pode rodar em máquina diferente

// IPs das máquinas para execução distribuída (configurar conforme ambiente)
var ConfiguracaoRede = struct {
	// IPs para máquinas físicas
	IP_Maquina_001 string
	IP_Maquina_002 string
	IP_Maquina_003 string

	// Portas das empresas
	Porta_001 string
	Porta_002 string
	Porta_003 string
}{
	IP_Maquina_001: "192.168.1.100", // Configurar com IP real da máquina 1
	IP_Maquina_002: "192.168.1.101", // Configurar com IP real da máquina 2
	IP_Maquina_003: "192.168.1.102", // Configurar com IP real da máquina 3
	Porta_001:      "8001",
	Porta_002:      "8002",
	Porta_003:      "8003",
}

// Detecta automaticamente o ambiente de execução e retorna a lista de servidores apropriada
func ObterServidoresDistribuidos() []string {
	// Verifica se está em ambiente Docker
	if _, err := os.Stat("/.dockerenv"); err == nil {
		fmt.Printf("[REDE] Detectado ambiente Docker - usando nomes de containers\n")
		return []string{
			"http://empresa_001:8001",
			"http://empresa_002:8002",
			"http://empresa_003:8003",
		}
	}

	// Caso contrário, usa IPs das máquinas físicas
	fmt.Printf("[REDE] Detectado ambiente físico - usando IPs das máquinas\n")
	return []string{
		"http://" + ConfiguracaoRede.IP_Maquina_001 + ":" + ConfiguracaoRede.Porta_001,
		"http://" + ConfiguracaoRede.IP_Maquina_002 + ":" + ConfiguracaoRede.Porta_002,
		"http://" + ConfiguracaoRede.IP_Maquina_003 + ":" + ConfiguracaoRede.Porta_003,
	}
}

// Obtém o endereço deste servidor baseado no ID da empresa
func ObterMeuEnderecoDistribuido(empresaID string) string {
	servidores := ObterServidoresDistribuidos()

	// Busca pelo endereço que contém o ID da empresa ou a porta correspondente
	var porta string
	switch empresaID {
	case "001":
		porta = ":" + ConfiguracaoRede.Porta_001
	case "002":
		porta = ":" + ConfiguracaoRede.Porta_002
	case "003":
		porta = ":" + ConfiguracaoRede.Porta_003
	default:
		porta = ":8000"
	}

	for _, servidor := range servidores {
		if strings.Contains(servidor, empresaID) || strings.Contains(servidor, porta) {
			return servidor
		}
	}

	// Fallback: constrói endereço baseado no ambiente
	if _, err := os.Stat("/.dockerenv"); err == nil {
		// Ambiente Docker
		return "http://empresa_" + empresaID + porta
	} else {
		// Ambiente físico
		var ip string
		switch empresaID {
		case "001":
			ip = ConfiguracaoRede.IP_Maquina_001
		case "002":
			ip = ConfiguracaoRede.IP_Maquina_002
		case "003":
			ip = ConfiguracaoRede.IP_Maquina_003
		default:
			ip = "localhost"
		}
		return "http://" + ip + porta
	}
}

// Obtém lista de outros servidores (excluindo este servidor)
func ObterOutrosServidoresDistribuidos(empresaID string) []string {
	servidores := ObterServidoresDistribuidos()
	meuEndereco := ObterMeuEnderecoDistribuido(empresaID)

	var outrosServidores []string
	for _, servidor := range servidores {
		if servidor != meuEndereco {
			outrosServidores = append(outrosServidores, servidor)
		}
	}

	fmt.Printf("[REDE] Configurados %d outros servidores para comunicação\n", len(outrosServidores))
	for _, servidor := range outrosServidores {
		fmt.Printf("[REDE] - %s\n", servidor)
	}

	return outrosServidores
}

// Função para atualizar IPs das máquinas em runtime (útil para configuração dinâmica)
func ConfigurarIPsMaquinas(ip001, ip002, ip003 string) {
	if ip001 != "" {
		ConfiguracaoRede.IP_Maquina_001 = ip001
	}
	if ip002 != "" {
		ConfiguracaoRede.IP_Maquina_002 = ip002
	}
	if ip003 != "" {
		ConfiguracaoRede.IP_Maquina_003 = ip003
	}

	fmt.Printf("[REDE] IPs das máquinas atualizados:\n")
	fmt.Printf("[REDE] - Máquina 001: %s\n", ConfiguracaoRede.IP_Maquina_001)
	fmt.Printf("[REDE] - Máquina 002: %s\n", ConfiguracaoRede.IP_Maquina_002)
	fmt.Printf("[REDE] - Máquina 003: %s\n", ConfiguracaoRede.IP_Maquina_003)
}
