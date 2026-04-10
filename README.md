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

## Build para Windows

### Opção 1 — Compilar direto no Windows

Instale os pré-requisitos (Go 1.22+, Node.js 18+, Git) e execute no **PowerShell** ou **Prompt de Comando**:

```powershell
# 1. Build do frontend
cd web
npm install
npm run build
cd ..

# 2. Copia o dist para o embed
Remove-Item -Recurse -Force internal\frontend\dist -ErrorAction SilentlyContinue
Copy-Item -Recurse web\dist internal\frontend\dist

# 3. Build do binário
go build -o modbussim.exe .\cmd\modbussim\

# 4. Executar
.\modbussim.exe
```

### Opção 2 — Cross-compile a partir do Linux ou macOS

```bash
# Build do frontend (uma vez)
cd web && npm install && npm run build && cd ..
rm -rf internal/frontend/dist
cp -r web/dist internal/frontend/dist

# Cross-compile para Windows 64-bit
GOOS=windows GOARCH=amd64 go build -o modbussim.exe ./cmd/modbussim/
```

Para **Windows ARM** (Surface, etc.):

```bash
GOOS=windows GOARCH=arm64 go build -o modbussim-arm64.exe ./cmd/modbussim/
```

### Executar no Windows

```powershell
.\modbussim.exe
```

Com configuração personalizada:

```powershell
.\modbussim.exe -config minha-config.yaml
.\modbussim.exe -versions C:\Users\usuario\modbussim-configs
```

Para parar, pressione **Ctrl+C** na janela do terminal. Se o processo ficar em background:

```powershell
# Encontrar o processo
Get-Process modbussim

# Encerrar
Stop-Process -Name modbussim
```

> **Nota:** O Makefile requer `make` (disponível via [MSYS2](https://www.msys2.org/), [Git Bash](https://gitforwindows.org/) ou [WSL](https://learn.microsoft.com/pt-br/windows/wsl/)). Se preferir não instalar `make`, use os comandos manuais da Opção 1 acima.

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
