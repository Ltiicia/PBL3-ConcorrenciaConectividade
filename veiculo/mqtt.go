package main

import (
	"fmt"
	"strings"
	"time"

	mqtt "github.com/eclipse/paho.mqtt.golang"
)

var mqttClientVeiculo mqtt.Client
var mensagemRecebida chan string
var reservasConfirmadas map[string]bool // Para evitar mensagens duplicadas
var suprimirMensagensCancelamento bool  // Para controlar quando não exibir mensagens de cancelamento automático

// Inicializa cliente MQTT para o veículo
func inicializaMqttVeiculo(placa string) {
	reservasConfirmadas = make(map[string]bool)
	mensagemRecebida = make(chan string, 10)

	opts := mqtt.NewClientOptions().AddBroker("tcp://broker:1883")
	opts.SetClientID("veiculo_" + placa)

	opts.OnConnect = func(c mqtt.Client) {

		// Subscribe para mensagens direcionadas a este veículo
		topico := "mensagens/cliente/" + placa
		if token := c.Subscribe(topico, 0, handleMensagemVeiculo); token.Wait() && token.Error() != nil {
			fmt.Printf("[MQTT] Erro ao assinar tópico %s: %v\n", topico, token.Error())
		}

		// Subscribe para mensagens gerais
		if token := c.Subscribe("mensagens/geral", 0, handleMensagemGeral); token.Wait() && token.Error() != nil {
			fmt.Printf("[MQTT] Erro ao assinar tópico geral: %v\n", token.Error())
		}
	}

	opts.OnConnectionLost = func(c mqtt.Client, err error) {
		fmt.Printf("[MQTT] Conexão perdida: %v\n", err)
	}

	mqttClientVeiculo = mqtt.NewClient(opts)
	if token := mqttClientVeiculo.Connect(); token.Wait() && token.Error() != nil {
		fmt.Printf("[MQTT] Erro ao conectar: %v\n", token.Error())
		return
	}
}

// Inicializa cliente MQTT para o veículo com ID único para evitar conflitos
func inicializaMqttVeiculoComID(placa, clienteID string) {
	reservasConfirmadas = make(map[string]bool)
	mensagemRecebida = make(chan string, 10)

	opts := mqtt.NewClientOptions().AddBroker("tcp://broker:1883")
	opts.SetClientID(clienteID) // Usa ID único ao invés de apenas a placa

	opts.OnConnect = func(c mqtt.Client) {
		fmt.Printf("[MQTT] Conectado com ID único: %s\n", clienteID)

		// Subscribe para mensagens direcionadas a este veículo
		topico := "mensagens/cliente/" + placa
		if token := c.Subscribe(topico, 0, handleMensagemVeiculo); token.Wait() && token.Error() != nil {
			fmt.Printf("[MQTT] Erro ao assinar tópico %s: %v\n", topico, token.Error())
		}

		// Subscribe para mensagens gerais
		if token := c.Subscribe("mensagens/geral", 0, handleMensagemGeral); token.Wait() && token.Error() != nil {
			fmt.Printf("[MQTT] Erro ao assinar tópico geral: %v\n", token.Error())
		}
	}
	opts.OnConnectionLost = func(c mqtt.Client, err error) {
		fmt.Printf("[MQTT] Conexão perdida: %v\n", err)
	}

	mqttClientVeiculo = mqtt.NewClient(opts)
	if token := mqttClientVeiculo.Connect(); token.Wait() && token.Error() != nil {
		fmt.Printf("[MQTT] Erro ao conectar: %v\n", token.Error())
		return
	}
}

// Handler para mensagens direcionadas ao veículo
func handleMensagemVeiculo(client mqtt.Client, msg mqtt.Message) {
	mensagem := string(msg.Payload())
	fmt.Printf("\n[MQTT] 📨 Mensagem recebida: %s\n", mensagem)

	// Envia para o canal para processamento
	select {
	case mensagemRecebida <- mensagem:
	default:
		fmt.Println("[MQTT] ⚠️  Canal de mensagens cheio, descartando mensagem")
	}

	// Processa mensagem imediatamente para feedback visual
	processarMensagemVeiculo(mensagem)
}

// Handler para mensagens gerais
func handleMensagemGeral(client mqtt.Client, msg mqtt.Message) {
	mensagem := string(msg.Payload())
	fmt.Printf("\n[MQTT] 📢 Mensagem geral: %s\n", mensagem)
}

// Processa mensagens recebidas pelo veículo
func processarMensagemVeiculo(mensagem string) {
	partes := strings.Split(mensagem, ",")
	if len(partes) < 2 {
		return
	}

	tipo := partes[0]
	switch tipo {
	case "reserva_confirmada":
		if len(partes) >= 3 {
			ponto := partes[1]
			hash := partes[2]

			// Evita mensagens duplicadas para o mesmo ponto
			chaveReserva := fmt.Sprintf("reserva_%s", ponto)
			if !reservasConfirmadas[chaveReserva] {
				reservasConfirmadas[chaveReserva] = true
				fmt.Printf("✅ Reserva confirmada para %s\n", ponto)
				fmt.Printf("🔑 Hash completo: %s\n", hash)
				fmt.Printf("📝 Anote este hash para verificação posterior!\n")
			}
		}
	case "reserva_erro":
		if len(partes) >= 3 {
			ponto := partes[1]
			erro := partes[2]
			fmt.Printf("⚠️  Erro na reserva para %s - Erro: %s\n", ponto, erro)
		}
	case "recarga_confirmada":
		if len(partes) >= 4 {
			ponto := partes[1]
			valor := partes[2]
			hash := partes[3]
			fmt.Printf("🔋 Recarga confirmada em %s - Valor: R$ %s\n", ponto, valor)
			fmt.Printf("🔑 Hash completo: %s\n", hash)
			fmt.Printf("📝 Anote este hash para verificação posterior!\n")
		}
	case "recarga_negada":
		if len(partes) >= 3 {
			ponto := partes[1]
			motivo := partes[2]
			fmt.Printf("❌ Recarga negada em %s - Motivo: %s\n", ponto, motivo)
		}
	case "pagamento_confirmado":
		if len(partes) >= 4 {
			ponto := partes[1]
			valor := partes[2]
			hash := partes[3]
			fmt.Printf("💳 Pagamento confirmado para %s - Valor: R$ %s\n", ponto, valor)
			fmt.Printf("🔑 Hash completo: %s\n", hash)
			fmt.Printf("📝 Anote este hash para verificação posterior!\n")
		}
	case "ponto_liberado":
		if len(partes) >= 3 {
			ponto := partes[1]
			motivo := partes[2]
			fmt.Printf("🔓 Ponto %s foi liberado automaticamente - %s\n", ponto, motivo)
		}
	case "status_resposta":
		if len(partes) >= 6 {
			recargas := partes[1]
			pagamentos := partes[2]
			valorRecargas := partes[3]
			valorPagamentos := partes[4]
			saldo := partes[5]
			fmt.Printf("📊 Status: %s recargas (R$ %s), %s pagamentos (R$ %s), Saldo: R$ %s\n",
				recargas, valorRecargas, pagamentos, valorPagamentos, saldo)
		}
	case "reserva_cancelada":
		if len(partes) >= 3 && !suprimirMensagensCancelamento {
			ponto := partes[1]
			motivo := partes[2]
			// Só exibe se não estiver suprimindo mensagens de cancelamento automático
			if !strings.Contains(motivo, "Nenhuma reserva encontrada") {
				fmt.Printf("🚫 Reserva cancelada para %s - Motivo: %s\n", ponto, motivo)
			}
		}
	case "ponto_desconectado":
		if len(partes) >= 3 {
			ponto := partes[1]
			mensagem := partes[2]
			fmt.Printf("📴 Ponto %s desconectado - %s\n", ponto, mensagem)
		}
	default:
		// fmt.Printf("❓ Mensagem não reconhecida: %s\n", mensagem)
	}
}

// Envia mensagem via MQTT
func enviarMensagemMqtt(topico, mensagem string) {
	if mqttClientVeiculo != nil && mqttClientVeiculo.IsConnected() {
		token := mqttClientVeiculo.Publish(topico, 0, false, mensagem)
		token.Wait()
		fmt.Printf("[MQTT] ➡️  Mensagem enviada para %s: %s\n", topico, mensagem)
	} else {
		fmt.Println("[MQTT] ⚠️  Cliente MQTT não conectado")
	}
}

// Solicita reserva via MQTT
func solicitarReservaMqtt(placa, ponto string) {
	mensagem := fmt.Sprintf("RESERVA,%s,%s", placa, ponto)
	enviarMensagemMqtt("mensagens/cliente", mensagem)
}

// Limpa o registro de reservas confirmadas (usado no início de nova viagem)
func limparReservasConfirmadas() {
	reservasConfirmadas = make(map[string]bool)
}

// Solicita recarga via MQTT
func solicitarRecargaMqtt(placa, ponto string, valor float64) {
	mensagem := fmt.Sprintf("RECARGA,%s,%s,%.2f", placa, ponto, valor)
	enviarMensagemMqtt("mensagens/cliente", mensagem)
}

// Solicita pagamento via MQTT
func solicitarPagamentoMqtt(placa, ponto string, valor float64) {
	mensagem := fmt.Sprintf("PAGAMENTO,%s,%s,%.2f", placa, ponto, valor)
	enviarMensagemMqtt("mensagens/cliente", mensagem)
}

// Solicita status via MQTT
func solicitarStatusMqtt(placa string) {
	mensagem := fmt.Sprintf("STATUS,%s", placa)
	enviarMensagemMqtt("mensagens/cliente", mensagem)
}

// Aguarda resposta MQTT com timeout
func aguardarRespostaMqtt(timeout time.Duration) string {
	select {
	case mensagem := <-mensagemRecebida:
		return mensagem
	case <-time.After(timeout):
		return ""
	}
}

// Desconecta cliente MQTT
func desconectarMqtt() {
	if mqttClientVeiculo != nil && mqttClientVeiculo.IsConnected() {
		mqttClientVeiculo.Disconnect(250)
		fmt.Println("[MQTT] 🔌 Desconectado do broker")
	}
}

// Verifica se MQTT está conectado
func mqttConectado() bool {
	return mqttClientVeiculo != nil && mqttClientVeiculo.IsConnected()
}
