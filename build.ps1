# build.ps1 — Script de build nativo para Windows
# Uso: .\build.ps1 [-Arch amd64|arm64]
# Requer: Go 1.22+, Node.js 18+, npm

param(
    [ValidateSet("amd64", "arm64")]
    [string]$Arch = "amd64"
)

$ErrorActionPreference = "Stop"
$Binary = "modbussim.exe"
$WebDir = "web"
$DistDir = "internal\frontend\dist"

function Step($msg) { Write-Host "`n→ $msg" -ForegroundColor Cyan }
function OK($msg)   { Write-Host "  ✓ $msg" -ForegroundColor Green }
function Fail($msg) { Write-Host "`n✗ $msg" -ForegroundColor Red; exit 1 }

# ── Verificar dependências ────────────────────────────────────────────────────

Step "Checking dependencies..."

if (-not (Get-Command go -ErrorAction SilentlyContinue)) {
    Fail "Go not found. Install from https://go.dev/dl/"
}
$goVersion = (go version) -replace "go version go", "" -replace " .*", ""
OK "Go $goVersion"

if (-not (Get-Command npm -ErrorAction SilentlyContinue)) {
    Fail "npm not found. Install Node.js from https://nodejs.org/"
}
$npmVersion = npm --version
OK "npm $npmVersion"

# ── Build do frontend ─────────────────────────────────────────────────────────

Step "Building frontend (React + Vite)..."

Push-Location $WebDir
try {
    npm install --silent
    if ($LASTEXITCODE -ne 0) { Fail "npm install failed" }

    npm run build
    if ($LASTEXITCODE -ne 0) { Fail "npm run build failed" }
} finally {
    Pop-Location
}
OK "Frontend built in $WebDir\dist"

# ── Copiar dist para o embed ──────────────────────────────────────────────────

Step "Copying dist to $DistDir..."

if (Test-Path $DistDir) {
    Remove-Item -Recurse -Force $DistDir
}
Copy-Item -Recurse "$WebDir\dist" $DistDir
OK "Copied to $DistDir"

# ── Build do binário Go ───────────────────────────────────────────────────────

Step "Compiling Go binary (windows/$Arch)..."

$env:GOOS   = "windows"
$env:GOARCH = $Arch

go build -o $Binary .\cmd\modbussim\
if ($LASTEXITCODE -ne 0) { Fail "go build failed" }

OK "Binary: $Binary  ($([math]::Round((Get-Item $Binary).Length / 1MB, 1)) MB)"

# ── Resumo ────────────────────────────────────────────────────────────────────

Write-Host ""
Write-Host "Build complete! Run with:" -ForegroundColor White
Write-Host "  .\$Binary" -ForegroundColor Yellow
Write-Host ""
