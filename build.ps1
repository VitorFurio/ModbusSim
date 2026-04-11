# build.ps1 — Script de build nativo para Windows
# Uso: .\build.ps1 [-Arch amd64|arm64] [-Firewall]
# Requer: Go 1.22+, Node.js 18+, npm

param(
    [ValidateSet("amd64", "arm64")]
    [string]$Arch = "amd64",

    # Adiciona regra no Windows Firewall para a porta HTTP do simulador (requer admin)
    [switch]$Firewall
)

$ErrorActionPreference = "Stop"
$Binary  = "modbussim.exe"
$WebDir  = "web"
$DistDir = "internal\frontend\dist"
$HttpPort = 7070

function Write-Step($msg) { Write-Host "`n>> $msg" -ForegroundColor Cyan }
function Write-Ok($msg)   { Write-Host "   [OK] $msg" -ForegroundColor Green }
function Write-Fail($msg) { Write-Host "`n[ERRO] $msg" -ForegroundColor Red; exit 1 }

# ── Verificar dependencias ─────────────────────────────────────────────────────

Write-Step "Verificando dependencias..."

if (-not (Get-Command go -ErrorAction SilentlyContinue)) {
    Write-Fail "Go nao encontrado. Instale em: https://go.dev/dl/"
}
$goVer = (go version) -replace "go version go", "" -replace " .*", ""
Write-Ok "Go $goVer"

if (-not (Get-Command npm -ErrorAction SilentlyContinue)) {
    Write-Fail "npm nao encontrado. Instale o Node.js em: https://nodejs.org/"
}
$npmVer = npm --version
Write-Ok "npm $npmVer"

# ── Build do frontend ──────────────────────────────────────────────────────────

Write-Step "Compilando frontend (React + Vite)..."

Push-Location $WebDir
try {
    npm install --silent
    if ($LASTEXITCODE -ne 0) { Write-Fail "npm install falhou" }

    npm run build
    if ($LASTEXITCODE -ne 0) { Write-Fail "npm run build falhou" }
} finally {
    Pop-Location
}
Write-Ok "Frontend compilado em $WebDir\dist"

# ── Copiar dist para embed ─────────────────────────────────────────────────────

Write-Step "Copiando dist para $DistDir..."

if (Test-Path $DistDir) {
    Remove-Item -Recurse -Force $DistDir
}
Copy-Item -Recurse "$WebDir\dist" $DistDir
Write-Ok "Copiado para $DistDir"

# ── Build do binario Go ────────────────────────────────────────────────────────

Write-Step "Compilando binario Go (windows/$Arch)..."

$env:GOOS   = "windows"
$env:GOARCH = $Arch

go build -o $Binary .\cmd\modbussim\
$buildExit = $LASTEXITCODE

# Limpar variaveis de ambiente para nao afetar o restante da sessao
Remove-Item Env:GOOS   -ErrorAction SilentlyContinue
Remove-Item Env:GOARCH -ErrorAction SilentlyContinue

if ($buildExit -ne 0) { Write-Fail "go build falhou" }

$sizeMB = [math]::Round((Get-Item $Binary).Length / 1MB, 1)
Write-Ok "$Binary ($sizeMB MB)"

# ── Regra de Firewall (opcional) ───────────────────────────────────────────────

if ($Firewall) {
    Write-Step "Configurando Windows Firewall para porta $HttpPort..."

    $isAdmin = ([Security.Principal.WindowsPrincipal][Security.Principal.WindowsIdentity]::GetCurrent()).IsInRole(
        [Security.Principal.WindowsBuiltInRole]::Administrator)

    if (-not $isAdmin) {
        Write-Host "   [AVISO] Necessario executar como Administrador para criar regra de firewall." -ForegroundColor Yellow
        Write-Host "   Execute: .\build.ps1 -Firewall  (como Administrador)" -ForegroundColor Yellow
    } else {
        $ruleName = "ModbusSim HTTP $HttpPort"
        $existing = Get-NetFirewallRule -DisplayName $ruleName -ErrorAction SilentlyContinue
        if ($existing) {
            Write-Ok "Regra de firewall ja existe: $ruleName"
        } else {
            New-NetFirewallRule `
                -DisplayName $ruleName `
                -Direction Inbound `
                -Protocol TCP `
                -LocalPort $HttpPort `
                -Action Allow | Out-Null
            Write-Ok "Regra criada: $ruleName (TCP porta $HttpPort)"
        }
    }
}

# ── Resumo ─────────────────────────────────────────────────────────────────────

Write-Host ""
Write-Host "Build concluido! Execute com:" -ForegroundColor White
Write-Host "  .\$Binary" -ForegroundColor Yellow
Write-Host ""
Write-Host "Se o browser nao conectar, adicione a regra de firewall:" -ForegroundColor DarkGray
Write-Host "  .\build.ps1 -Firewall  (como Administrador)" -ForegroundColor DarkGray
Write-Host ""
