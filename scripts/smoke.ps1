$ErrorActionPreference = "Stop"

if ($PSVersionTable.PSVersion.Major -lt 7) {
  $pwsh = Get-Command pwsh -ErrorAction SilentlyContinue
  if (-not $pwsh) {
    throw "pwsh is required to run this smoke test"
  }
  & $pwsh.Source -NoProfile -ExecutionPolicy Bypass -File $PSCommandPath
  exit $LASTEXITCODE
}

$root = Split-Path -Parent $PSScriptRoot
$bin = Join-Path $root "bin"
$serverExe = Join-Path $bin "gpipe-server.exe"
$clientExe = Join-Path $bin "gpipe-client.exe"
$tmp = Join-Path $root ".smoke"
$dbPath = Join-Path $tmp "gpipe.db"
$config = Join-Path $tmp "gpipe.json"
$logs = Join-Path $tmp "logs"

New-Item -ItemType Directory -Force $tmp, $logs | Out-Null
Remove-Item -Force $dbPath -ErrorAction SilentlyContinue

function Escape-JsonString([string]$value) {
  return $value.Replace('\', '\\')
}

# 手工生成最小配置，确保 Windows 路径和 sqlite URL 都按 JSON 正确转义。
$databaseUrl = "sqlite://" + $dbPath + "?mode=rwc"
$databaseUrlJson = Escape-JsonString $databaseUrl
$logsJson = Escape-JsonString $logs
$json = @'
{{
  "database_url": "{0}",
  "listen_addr": "tcp://127.0.0.1:28118",
  "illegal_traffic_forward": "",
  "enable_tls": false,
  "tls_cert": "./cert.pem",
  "tls_key": "./server.key.pem",
  "web_base_dir": "",
  "web_addr": "127.0.0.1:28120",
  "web_username": "admin",
  "web_password": "admin@1234",
  "quiet": false,
  "log_dir": "{1}"
}}
'@ -f $databaseUrlJson, $logsJson
[System.IO.File]::WriteAllText($config, $json, (New-Object System.Text.UTF8Encoding($false)))

$server = Start-Process -FilePath $serverExe -ArgumentList "-config-file", $config -PassThru
Start-Sleep -Seconds 2

if ($server.HasExited) {
  throw "server exited unexpectedly before smoke requests"
}

try {
  $session = New-Object Microsoft.PowerShell.Commands.WebRequestSession
  $index = Invoke-WebRequest -Uri "http://127.0.0.1:28120/" -Method Get -WebSession $session
  if ($index.Content -notmatch "<!doctype html>") {
    throw "embedded index.html was not served"
  }
  Invoke-RestMethod -Uri "http://127.0.0.1:28120/api/login" -Method Post -Body (@{ username = "admin"; password = "admin@1234" } | ConvertTo-Json) -ContentType "application/json" -WebSession $session | Out-Null
  Invoke-RestMethod -Uri "http://127.0.0.1:28120/api/add_player" -Method Post -Body (@{ remark = "smoke"; key = "smoke" } | ConvertTo-Json) -ContentType "application/json" -WebSession $session | Out-Null

  $client = Start-Process -FilePath $clientExe -ArgumentList "run", "--server", "tcp://127.0.0.1:28118", "--key", "smoke", "--log-dir", $logs -PassThru
  Start-Sleep -Seconds 5
  if (-not $client.HasExited) {
    Stop-Process -Id $client.Id -Force
  }

  $clientLog = Join-Path $logs "client.log"
  if (-not (Test-Path $clientLog)) {
    throw "client log not found"
  }
  $content = Get-Content $clientLog -Raw
  if ($content -notmatch "login successful") {
    throw "smoke test failed: client did not login successfully"
  }
} finally {
  if ($server -and -not $server.HasExited) {
    Stop-Process -Id $server.Id -Force
  }
}
