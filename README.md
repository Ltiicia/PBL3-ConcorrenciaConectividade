<h2 align="center">Concorrência, Conectividade e Blockchain para Recarga de Veículos Elétricos</h2>
<h4 align="center">Projeto da disciplina TEC502 - Concorrência e Conectividade</h4>

<p align="center">
Este projeto simula um sistema distribuído para recarga de veículos elétricos, integrando empresas e veículos em uma rede robusta baseada em Blockchain, MQTT e API REST. O sistema evolui os modelos anteriores <a href="https://github.com/brendatrindade/recarga-inteligente">Projeto 1</a> e <a href="https://github.com/naylane/pbl2">Projeto 2</a>, trazendo como principal inovação a utilização de blockchain para garantia de integridade, consenso e rastreabilidade de transações como recarga e pagamento entre múltiplas empresas.
</p>

## Sumário
- [Introdução](#introdução)
- [Arquitetura do Sistema](#arquitetura-do-sistema)
  - [Broker MQTT](#broker-mqtt)
  - [Empresas (Servidores)](#empresas-servidores)
  - [API REST](#api-rest)
  - [Veículo](#veículo)
  - [Blockchain](#blockchain)
  - [Fluxo de Comunicação](#fluxo-de-comunicação)
- [Protocolo de Comunicação](#protocolo-de-comunicação)
- [Concorrência e Consenso](#concorrência-e-consenso)
  - [Consenso entre as empresas](#consenso-entre-as-empresas)
- [Execução com Docker](#execução-com-docker)
- [Como Executar](#como-executar)
- [Tecnologias Utilizadas](#tecnologias-utilizadas)
- [Conclusão](#conclusão)
- [Desenvolvedoras](#desenvolvedoras)
- [Referências](#referências)

[Relatório]()

## Introdução
O sistema distribuído simula operações entre empresas de recarga e veículos elétricos, avançando em relação aos modelos anteriores, sobretudo o projeto 2, ao incorporar uma solução baseada em **Blockchain** para registrar todas as transações de forma imutável, auditável e segura, além de manter a comunicação eficiente via MQTT e REST. O objetivo é permitir que veículos planejem viagens, reservem pontos de recarga, realizem recargas e pagamentos, com garantia de integridade, consenso e concorrência segura entre múltiplos participantes.

## Arquitetura do Sistema
O sistema é composto por múltiplos serviços utilizando MQTT para comunicação assíncrona e API REST para coordenação entre servidores, os dados são persistidos em arquivos JSON montados como volumes nos containers. Todos orquestrados via Docker Compose.

### Broker MQTT
- Utiliza Eclipse Mosquitto.
- Permite comunicação assíncrona e em tempo real entre empresas e veículos.
- Exposto na porta 1883.

### Empresas (Servidores)
- Cada empresa representa uma região e expõe uma API REST.
- Mantém sua cópia local da blockchain, sincronizada com as demais empresas.
- Gerencia pontos de recarga, reservas, recargas e pagamentos.
- Valida e propaga blocos para consenso entre as empresas.
- Persistência de dados em arquivos JSON.

### API REST
- Usada para coordenação de reservas, recargas, pagamentos e sincronização de blockchain entre empresas.
- Endpoints: `/blockchain`, `/bloco`, `/reserva`, `/recarga`, `/pagamento`, `/sincronizar`, `/api/status`, `/api/historico`...

### Veículo
- Interface de terminal para o usuário simular viagens, reservas, recargas e pagamentos.
- Consulta status, histórico e verifica integridade das transações via hash.
- Comunica-se via MQTT e HTTP.
- Mantém histórico local de viagens e transações.

### Blockchain
- Cada empresa mantém uma blockchain local, com blocos assinados digitalmente (RSA).
- Cada transação é registrada como um bloco.
- Consenso: novos blocos só são aceitos se todas as empresas validarem e propagarem o bloco.
- Permite rastreabilidade, integridade e auditoria de todas as operações.

### Fluxo de Comunicação
1. Veículo solicita reserva/recarga/pagamento via MQTT ou HTTP.
2. Empresa valida, registra na blockchain, propaga para consenso.
3. Empresas sincronizam blockchains e garantem integridade.
4. Veículo recebe confirmação e pode consultar histórico e status.

## Protocolo de Comunicação
- MQTT: Comunicação assíncrona e em tempo real para reservas, recargas, notificações e status.
- REST: Coordenação de operações críticas, sincronização de blockchain e fallback.
- JSON: Todas as mensagens e dados persistidos são estruturados em JSON.

## Concorrência e Consenso
- Uso de mutexes para garantir exclusão mútua em operações críticas.
- Canal para processar blocos em sequência.
- Consenso entre empresas: um bloco só é aceito se todas as empresas validarem.
- Recuperação automática em caso de corrupção da blockchain.
- Cancelamento automático de reservas em pontos desconectados.

### Consenso entre as empresas
O mecanismo de consenso é fundamental para garantir a integridade e a confiança do sistema. Para que um novo bloco (transação) seja aceito na blockchain de todas as empresas, o seguinte processo ocorre:

1. **Criação do Bloco**
   - A empresa que inicia a transação monta um novo bloco com todos os dados.
   - O bloco é assinado digitalmente com a chave privada da empresa autora.

2. **Validação Local**
   - Antes de propagar, a empresa verifica se o bloco é válido localmente: sequência correta, hash anterior correto, hash do bloco correto, assinatura digital válida e sem duplicidade.

3. **Propagação para Consenso**
   - O bloco é enviado via HTTP para as APIs REST das demais empresas.
   - Cada empresa processa o bloco em sequência para garantir ordenação e evitar concorrência.

4. **Validação em Cada Empresa**
     - **Sequência:** O índice do bloco deve ser o próximo da cadeia local.
     - **Hash Anterior:** O hash do bloco anterior deve bater com o último bloco local.
     - **Hash do Bloco:** O hash calculado deve ser igual ao informado.
     - **Assinatura Digital:** A assinatura é válidada usando a chave pública da empresa autora.
     - **Duplicidade:** Não pode haver índice ou hash já existente na cadeia local.
          
Se todas as validações passarem, o bloco é adicionado à blockchain local.
Se qualquer validação falhar, o bloco é rejeitado.

5. **Confirmação de Consenso**
   - O bloco só é considerado aceito se **todas as empresas** responderem positivamente à propagação.
   - Se alguma empresa rejeitar, o bloco não é adicionado nem na empresa de origem.

6. **Sincronização e Recuperação**
   - Se uma empresa detectar corrupção ou desatualização, pode buscar a blockchain válida das outras empresas e sincronizar.

Esse modelo garante integridade, auditabilidade, resiliência e confiança distribuída entre todos os participantes do sistema.

## Execução com Docker
- O sistema é simulado com Docker Compose.
- Volumes mapeiam arquivos de dados para persistência.

## Como Executar
### Pré-requisitos
- [Docker](https://www.docker.com/)
- [Docker Compose](https://docs.docker.com/compose/)
- [Go](https://go.dev/) (opcional, para testes locais)

### Passo a passo
1. Clone o repositório:
   ```bash
   git clone https://github.com/usuario/nome-do-repositorio.git
   cd nome-do-repositorio
   ```
2. Inicie todos os serviços:
   ```bash
   docker-compose up --build -d
   ```
3. Para acessar a interface do veículo:
   ```bash
   docker-compose exec veiculo sh
   ./veiculo
   ```
4. Para visualizar logs em tempo real de algum serviço:
   ```bash
   docker compose logs -f <nome-do-serviço>
   ```
5. Para encerrar:
   ```bash
   docker-compose down
   ```

## Tecnologias Utilizadas
- Go (Golang)
- MQTT (Eclipse Mosquitto)
- REST (API HTTP)
- Docker e Docker Compose
- Blockchain (Crypto)
- JSON para persistência de dados

## Conclusão
O sistema demonstra na prática conceitos de sistemas distribuídos, blockchain, comunicação em tempo real e concorrência. O uso de blockchain garante integridade, rastreabilidade e consenso nas operações, enquanto MQTT e REST proporcionam flexibilidade e robustez na comunicação. O controle de concorrência com mutexes e canais assegura integridade mesmo com múltiplos veículos e empresas atuando simultaneamente.

## Desenvolvedoras
<table>
  <tr>
    <td align="center"><img style="" src="https://avatars.githubusercontent.com/u/142849685?v=4" width="100px;" alt=""/><br /><sub><b> Brenda Araújo </b></sub></a><br />👨‍💻</a></td>
    <td align="center"><img style="" src="https://avatars.githubusercontent.com/u/89545660?v=4" width="100px;" alt=""/><br /><sub><b> Naylane Ribeiro </b></sub></a><br />👨‍💻</a></td>
    <td align="center"><img style="" src="https://avatars.githubusercontent.com/u/124190885?v=4" width="100px;" alt=""/><br /><sub><b> Letícia Gonçalves </b></sub></a><br />👨‍💻</a></td>    
  </tr>
</table>


## Referências
Donovan, A. A. and Kernighan, B. W. (2016). The Go Programming Language. Addison-Wesley.   
Merkel, D. (2014). Docker: lightweight Linux containers for consistent development and deployment. Linux Journal, 2014(239), 2.    
Silberschatz, A., Galvin, P. B., and Gagne, G. (2018). Operating System Concepts (10th ed.). Wiley.   
Stevens, W. R. (1998). UNIX Network Programming, Volume 1: The Sockets Networking API (2nd ed.). Prentice Hall.    
Tanenbaum, A. S. and Van Steen, M. (2007). Distributed Systems: Principles and Paradigms (2nd ed.). Pearson Prentice Hall.
Nakamoto, S. (2008). Bitcoin: A Peer-to-Peer Electronic Cash System.
