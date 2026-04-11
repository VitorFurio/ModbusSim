# ModbusSim

Simulador de dispositivo Modbus TCP com interface web para gerenciamento de registradores e visualização de sinais em tempo real.

---

## Sumário

- [Requisitos](#requisitos)
- [Instalação](#instalação)
- [Como Rodar](#como-rodar)
- [Build para Windows](#build-para-windows)
- [Configuração](#configuração)
- [Tipos de Registradores](#tipos-de-registradores)
- [Tipos de Sinal](#tipos-de-sinal)
- [API REST](#api-rest)
- [Como Testar](#como-testar)

---

## Requisitos

| Ferramenta | Versão mínima |
|---|---|
| Go | 1.22 |
| Node.js | 18+ |
| npm | 9+ |

---

## Instalação

```bash
git clone https://github.com/VitorFurio/ModbusSim.git
cd ModbusSim
```

---

## Como Rodar

### Modo produção (frontend embutido no binário)

```bash
make build   # compila o frontend React e o binário Go
./modbussim  # inicia o simulador
```

Ou em um único comando:

```bash
make run
```

Saída esperada:

```
  ┌─────────────────────────────────────────────────────────┐
  │             ModbusSim — Modbus TCP Simulator            │
  └─────────────────────────────────────────────────────────┘
  Modbus TCP : :5020
  Admin HTTP : http://localhost:7070

  Press Ctrl+C to stop.
```

Acesse a interface web em: **http://localhost:7070**

---

### Modo desenvolvimento (hot reload)

Em dois terminais separados:

```bash
# Terminal 1 — backend Go
go run ./cmd/modbussim/

# Terminal 2 — frontend Vite (proxy para :7070)
cd web && npm install && npm run dev
```

A interface de desenvolvimento estará em **http://localhost:5173**.

---

### Com arquivo de configuração personalizado

```bash
./modbussim -config minha-config.yaml
```

### Diretório de versões salvas

```bash
./modbussim -versions ./minhas-versoes
```

Por padrão as versões são salvas no mesmo diretório do binário, dentro de `configs/`.

---

## Build por Plataforma

### Cross-compile a partir do Linux ou macOS (recomendado)

O `make` compila o frontend **uma única vez** e gera o binário para a plataforma desejada:

| Comando | Plataforma |
|---|---|
| `make build` | Sistema atual |
| `make build-windows` | Windows x86-64 |
| `make build-windows-arm64` | Windows ARM64 (Surface, Snapdragon X) |
| `make build-linux` | Linux x86-64 |
| `make build-linux-arm64` | Linux ARM64 (Raspberry Pi 4/5, servidores ARM) |
| `make build-darwin` | macOS Intel |
| `make build-darwin-arm64` | macOS Apple Silicon (M1/M2/M3/M4) |
| `make build-all` | Todas as plataformas de uma vez |

Exemplo — gerar binário para Windows a partir do macOS ou Linux:

```bash
make build-windows
# → modbussim-windows-amd64.exe
```

Gerar todos os binários de uma vez:

```bash
make build-all
# → modbussim-windows-amd64.exe
# → modbussim-windows-arm64.exe
# → modbussim-linux-amd64
# → modbussim-linux-arm64
# → modbussim-darwin-amd64
# → modbussim-darwin-arm64
```

---

### Build nativo no Windows

No Windows **não há `make`** disponível por padrão. Use o script PowerShell incluído no projeto:

```powershell
.\build.ps1
```

Para ARM64 (Surface, Snapdragon X):

```powershell
.\build.ps1 -Arch arm64
```

O script verifica as dependências, compila o frontend com npm, copia o dist e gera `modbussim.exe`.

> Se a política de execução do PowerShell estiver restrita, execute primeiro:
> ```powershell
> Set-ExecutionPolicy -Scope CurrentUser RemoteSigned
> ```

### Executar no Windows

```powershell
.\modbussim.exe
```

Com opções:

```powershell
.\modbussim.exe -config minha-config.yaml
.\modbussim.exe -versions C:\Users\usuario\modbussim-configs
```

Para parar: **Ctrl+C** no terminal. Se o processo ficar em background:

```powershell
Stop-Process -Name modbussim
```

### Troubleshooting Windows — browser não conecta (ERR_CONNECTION_RESET)

#### Passo 1 — Confirmar que o servidor esta ouvindo

Antes de qualquer outra coisa, confirme que o processo está rodando e aceitando TCP:

```powershell
# Deve retornar TcpTestSucceeded: True
Test-NetConnection -ComputerName localhost -Port 7070
```

Se retornar `False`, verifique se o executável está rodando (`Get-Process modbussim`).

#### Passo 2 — Regra no Windows Firewall

O Windows Firewall pode bloquear a porta 7070. Solução mais simples — rode o build com a flag `-Firewall` **como Administrador**:

```powershell
# Abra o PowerShell como Administrador, depois:
.\build.ps1 -Firewall
```

Isso cria automaticamente uma regra de entrada para a porta TCP 7070.

Alternativamente, adicione a regra manualmente:

```powershell
# PowerShell como Administrador
New-NetFirewallRule -DisplayName "ModbusSim HTTP 7070" `
    -Direction Inbound -Protocol TCP -LocalPort 7070 -Action Allow
```

Ou via painel do Windows: **Windows Defender Firewall → Regras de Entrada → Nova Regra → Porta → TCP 7070 → Permitir**.

#### Passo 3 — Antivírus / Software de segurança corporativo (EDR)

Se `Test-NetConnection` retornar `True` mas o browser ainda mostrar ERR_CONNECTION_RESET, o problema é quase certamente um **software de segurança corporativo** (Windows Defender, CrowdStrike, SentinelOne, Symantec, etc.) que intercepta conexões TCP no nível do driver antes de entregá-las ao processo Go.

Sintoma característico: o log do ModbusSim mostra `msg="api server started"` mas **nunca aparece** `msg=conn` nem `msg=http` — ou seja, o servidor aceita a porta mas nenhuma conexão chega ao Go.

**Soluções (escolha uma):**

1. **Adicionar exclusão no Windows Defender** (se for o AV ativo):
   - Abra **Segurança do Windows → Proteção contra vírus e ameaças → Configurações → Exclusões → Adicionar exclusão**
   - Adicione o caminho completo do executável `modbussim.exe`

2. **Solicitar ao TI corporativo** uma exclusão ou permissão para o executável e as portas 5020 e 7070

3. **Testar em uma máquina pessoal** (fora do domínio corporativo) para confirmar se é bloqueio de política

4. **Usar uma porta diferente** — portas como 8080 ou 3000 costumam ter menos restrições em políticas corporativas:
   ```yaml
   # config.yaml
   admin_addr: ":8080"
   ```
   ```powershell
   .\modbussim.exe -config config.yaml
   ```

---

## Configuração

O simulador aceita um arquivo YAML com a seguinte estrutura:

```yaml
version: "1"
name: minha-planta
description: Exemplo de configuração
modbus_addr: ":5020"   # endereço do servidor Modbus TCP
admin_addr: ":7070"    # endereço da interface web

registers:
  - id: temperature
    name: Temperatura
    description: Sensor de temperatura
    address: 0          # endereço Modbus (0-based)
    data_type: float32  # uint16 | int16 | uint32 | int32 | float32 | bool
    unit: "°C"
    signal:
      kind: sine
      amplitude: 5
      period: 30
      offset: 25
      min: 15
      max: 35

  - id: status
    name: Status
    address: 2
    data_type: uint16
    signal:
      kind: constant
      value: 1
```

Se nenhum arquivo for informado, o simulador inicia com três registradores de exemplo (temperatura, pressão e umidade).

---

## Tipos de Registradores

| `data_type` | Tamanho | Descrição |
|---|---|---|
| `uint16` | 1 word | Inteiro sem sinal de 16 bits |
| `int16` | 1 word | Inteiro com sinal de 16 bits |
| `bool` | 1 word | Booleano (0 ou 1) |
| `uint32` | 2 words | Inteiro sem sinal de 32 bits |
| `int32` | 2 words | Inteiro com sinal de 32 bits |
| `float32` | 2 words | Ponto flutuante de 32 bits |

---

## Tipos de Sinal

| `kind` | Descrição | Parâmetros relevantes |
|---|---|---|
| `constant` | Valor fixo | `value` |
| `sine` | Onda senoidal | `amplitude`, `period`, `offset`, `min`, `max` |
| `ramp` | Rampa linear com wrap | `rate`, `min`, `max` |
| `step` | Alterna entre dois valores | `low`, `high`, `period` |
| `random_walk` | Caminhada aleatória | `step_max_walk`, `min`, `max` |
| `counter` | Contador incremental | `step_min`, `interval_ms`, `min`, `max` |
| `counter_random` | Contador com incremento aleatório | `step_min`, `step_max`, `interval_ms`, `min`, `max` |

---

## Function Codes Modbus Suportados

| FC | Nome | Descrição |
|---|---|---|
| `0x01` | Read Coils | Lê bits (word != 0 = ON) |
| `0x02` | Read Discrete Inputs | Idem ao FC01 |
| `0x03` | Read Holding Registers | Lê registradores de 16 bits |
| `0x04` | Read Input Registers | Idem ao FC03 |

---

## API REST

A interface web consome uma API HTTP disponível em `http://localhost:7070/api`.

### Registradores

```
GET    /api/registers          Lista todos os registradores
POST   /api/registers          Cria um novo registrador
PUT    /api/registers/{id}     Atualiza um registrador
DELETE /api/registers/{id}     Remove um registrador
```

### Configuração

```
GET    /api/config             Retorna a configuração atual
POST   /api/config/save        Salva a configuração como nova versão
```

### Versões

```
GET    /api/versions           Lista versões salvas
POST   /api/versions/load      Carrega uma versão salva  (body: {"path": "..."})
GET    /api/versions/export    Exporta a config atual como YAML
POST   /api/versions/import    Importa uma config via YAML no body
```

### WebSocket

```
WS     /ws                     Stream de snapshots dos registradores (200ms)
```

Exemplo de mensagem recebida:

```json
{
  "type": "snapshot",
  "registers": [
    { "id": "temperature", "value": 24.3, "updated_at": 1712700000000, "history": [...] }
  ]
}
```

---

## Testando o Servidor em Execução

Formas de verificar se o servidor está funcionando corretamente **sem abrir o browser** e sem olhar o terminal.

### Testar a API HTTP via PowerShell (Windows)

```powershell
# Verificar se a porta HTTP esta ouvindo
Test-NetConnection -ComputerName localhost -Port 7070

# Listar registradores
Invoke-WebRequest -Uri http://localhost:7070/api/registers -UseBasicParsing

# Listar registradores com saida formatada
(Invoke-WebRequest -Uri http://localhost:7070/api/registers -UseBasicParsing).Content | ConvertFrom-Json

# Verificar configuracao
(Invoke-WebRequest -Uri http://localhost:7070/api/config -UseBasicParsing).Content | ConvertFrom-Json
```

### Testar a API HTTP via curl (Windows 10+, Linux, macOS)

```bash
# Listar registradores
curl http://localhost:7070/api/registers

# Listar registradores com saida formatada (requer jq instalado)
curl http://localhost:7070/api/registers | jq .

# Verificar configuracao
curl http://localhost:7070/api/config
```

### Testar a porta Modbus TCP

```powershell
# Windows — verificar se a porta Modbus esta ouvindo
Test-NetConnection -ComputerName localhost -Port 5020
```

```bash
# Linux/macOS
nc -zv localhost 5020
```

### Verificar processos em execucao

```powershell
# Windows — ver se o processo esta rodando e em quais portas
netstat -ano | findstr "7070"
netstat -ano | findstr "5020"
Get-Process modbussim
```

```bash
# Linux/macOS
ss -tlnp | grep -E '7070|5020'
```

### Saida esperada de `Test-NetConnection`

```
ComputerName     : localhost
RemoteAddress    : ::1
RemotePort       : 7070
InterfaceAlias   : Loopback Pseudo-Interface 1
SourceAddress    : ::1
TcpTestSucceeded : True
```

Se `TcpTestSucceeded: True` — o servidor esta ouvindo e aceita conexoes TCP.
Se `TcpTestSucceeded: False` — o servidor nao esta rodando ou a porta esta bloqueada.

---

## Como Testar

### Executar todos os testes

```bash
go test ./...
```

### Executar com saída detalhada

```bash
go test ./... -v
```

### Executar testes de um pacote específico

```bash
go test ./internal/register/... -v
go test ./internal/config/... -v
go test ./internal/modbus/... -v
go test ./internal/api/... -v
```

### Cobertura de testes

```bash
go test ./... -cover
```

Para gerar um relatório HTML de cobertura:

```bash
go test ./... -coverprofile=coverage.out
go tool cover -html=coverage.out
```

### Estrutura dos testes

| Pacote | Arquivo de teste | O que é testado |
|---|---|---|
| `internal/register` | `register_test.go` | DataType, WordAddresses |
| `internal/register` | `engine_test.go` | Engine CRUD, sinais, contador, WordAt, encode |
| `internal/config` | `config_test.go` | Load, Save, ListVersions, Export, Import |
| `internal/modbus` | `server_test.go` | FC01–FC04, códigos de erro, Stop |
| `internal/api` | `server_test.go` | Todos os handlers HTTP, broadcastLoop |

> **Nota:** Os testes não dependem do frontend React. O arquivo `internal/frontend/dist/index.html` é um stub mínimo que permite compilar o binário sem executar `npm run build`.

---

## Estrutura do Projeto

```
ModbusSim/
├── cmd/modbussim/        # Entrypoint principal
├── internal/
│   ├── api/              # Servidor HTTP + WebSocket
│   ├── config/           # Leitura/escrita de configuração YAML
│   ├── frontend/         # Frontend embutido (embed.go + dist/)
│   ├── modbus/           # Servidor Modbus TCP
│   └── register/         # Engine de simulação de registradores
├── web/                  # Código-fonte do frontend (React + Vite)
├── Makefile
└── go.mod
```
