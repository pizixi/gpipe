$ErrorActionPreference = "Stop"

if ($env:OS -ne "Windows_NT") {
  throw "upgrade-smoke.ps1 currently validates the Windows file-locking path and must run on Windows"
}

$root = Split-Path -Parent $PSScriptRoot
$testDir = Join-Path ([System.IO.Path]::GetTempPath()) ("gpipe-upgrade-smoke-" + [Guid]::NewGuid().ToString("N"))
$logs = Join-Path $testDir "logs"
$cache = Join-Path $testDir "client-cache"
$templateDir = Join-Path $testDir "client-templates"
$serverExe = Join-Path $testDir "gpipe-server.exe"
$clientExe = Join-Path $testDir "gpipe-client.exe"
$configPath = Join-Path $testDir "gpipe.json"
$dbPath = Join-Path $testDir "gpipe.db"
$basePort = Get-Random -Minimum 30000 -Maximum 50000
$clientPort = $basePort
$webPort = $basePort + 1
$playerKey = "upgradeSmokeKey"

New-Item -ItemType Directory -Force -Path $testDir, $logs, $cache, $templateDir | Out-Null

Push-Location $root
try {
  & go build -trimpath -buildvcs=false -o $serverExe .\cmd\server
  if ($LASTEXITCODE -ne 0) { throw "build server failed" }
  & go build -trimpath -buildvcs=false -ldflags "-s -w -X main.clientVersion=1.0.0 -X main.clientPlatform=windows-amd64" -o $clientExe .\cmd\client
  if ($LASTEXITCODE -ne 0) { throw "build old client failed" }
  $placeholder = ((& go run .\scripts\print-client-placeholder) -join "").Trim()
  if ($LASTEXITCODE -ne 0) { throw "read client placeholder failed" }
  $templateName = "gpipe-client-template-windows-amd64.exe"
  $templatePath = Join-Path $templateDir $templateName
  & go build -trimpath -buildvcs=false -ldflags "-s -w -X main.embeddedClientConfig=$placeholder -X main.clientVersion=1.1.0 -X main.clientPlatform=windows-amd64" -o $templatePath .\cmd\client
  if ($LASTEXITCODE -ne 0) { throw "build upgrade template failed" }
  $templateInfo = Get-Item -LiteralPath $templatePath
  $manifest = [ordered]@{
    version = "1.1.0"
    updater_protocol = 1
    artifacts = @([ordered]@{
      target = "windows-amd64"
      file = $templateName
      size = $templateInfo.Length
      sha256 = (Get-FileHash -LiteralPath $templatePath -Algorithm SHA256).Hash.ToLowerInvariant()
    })
  }
  [System.IO.File]::WriteAllText((Join-Path $templateDir "manifest.json"), ($manifest | ConvertTo-Json -Depth 10), (New-Object System.Text.UTF8Encoding($false)))
} finally {
  Pop-Location
}

$oldHash = (Get-FileHash -LiteralPath $clientExe -Algorithm SHA256).Hash
$config = [ordered]@{
  database_url = "sqlite://$dbPath`?mode=rwc"
  listen_addr = "tcp://127.0.0.1:$clientPort"
  illegal_traffic_forward = ""
  enable_tls = $false
  tls_cert = ""
  tls_key = ""
  web_base_dir = ""
  web_addr = "127.0.0.1:$webPort"
  web_username = "admin"
  web_password = "admin@1234"
  client_template_dir = $templateDir
  client_artifact_cache_dir = $cache
  client_latest_version = "1.1.0"
  quiet = $true
  log_dir = $logs
}
[System.IO.File]::WriteAllText($configPath, ($config | ConvertTo-Json -Depth 10), (New-Object System.Text.UTF8Encoding($false)))

$server = $null
$client = $null
try {
  $server = Start-Process -FilePath $serverExe -ArgumentList "-config-file", $configPath -WorkingDirectory $root -WindowStyle Hidden -PassThru
  Start-Sleep -Seconds 2
  if ($server.HasExited) { throw "server exited before upgrade smoke test" }

  $webSession = New-Object Microsoft.PowerShell.Commands.WebRequestSession
  $apiBase = "http://127.0.0.1:$webPort/api"
  Invoke-RestMethod -Uri "$apiBase/login" -Method Post -Body (@{ username = "admin"; password = "admin@1234" } | ConvertTo-Json) -ContentType "application/json" -WebSession $webSession | Out-Null
  Invoke-RestMethod -Uri "$apiBase/add_player" -Method Post -Body (@{ remark = "upgrade-smoke"; key = $playerKey } | ConvertTo-Json) -ContentType "application/json" -WebSession $webSession | Out-Null
  Invoke-RestMethod -Uri "$apiBase/update_client_build_settings" -Method Post -Body (@{
    server = "tcp://127.0.0.1:$clientPort"
    enable_tls = $false
    tls_server_name = ""
    use_shadowsocks = $false
    ss_server = ""
    ss_method = "chacha20-ietf-poly1305"
    ss_password = ""
  } | ConvertTo-Json) -ContentType "application/json" -WebSession $webSession | Out-Null

  $client = Start-Process -FilePath $clientExe -ArgumentList "run", "--server", "tcp://127.0.0.1:$clientPort", "--key", $playerKey, "--log-dir", $logs -WorkingDirectory $testDir -WindowStyle Hidden -PassThru

  $player = $null
  $deadline = [DateTime]::UtcNow.AddSeconds(30)
  while ([DateTime]::UtcNow -lt $deadline) {
    $list = Invoke-RestMethod -Uri "$apiBase/player_list" -Method Post -Body (@{ page_number = 0; page_size = 0 } | ConvertTo-Json) -ContentType "application/json" -WebSession $webSession
    $player = $list.players | Select-Object -First 1
    if ($player.online -and $player.client_version -eq "1.0.0" -and $player.can_upgrade) { break }
    Start-Sleep -Milliseconds 500
  }
  if (-not $player.can_upgrade) { throw "old client did not report remote-upgrade capability" }

  $result = Invoke-RestMethod -Uri "$apiBase/upgrade_client" -Method Post -Body (@{ player_id = $player.id } | ConvertTo-Json) -ContentType "application/json" -WebSession $webSession
  if ($result.code -ne 0) { throw "upgrade request failed: $($result.msg)" }

  $deadline = [DateTime]::UtcNow.AddSeconds(150)
  while ([DateTime]::UtcNow -lt $deadline) {
    $list = Invoke-RestMethod -Uri "$apiBase/player_list" -Method Post -Body (@{ page_number = 0; page_size = 0 } | ConvertTo-Json) -ContentType "application/json" -WebSession $webSession
    $player = $list.players | Select-Object -First 1
    if ($player.online -and $player.client_version -eq "1.1.0" -and $player.is_latest -and $player.upgrade_status -eq "succeeded") { break }
    if ($player.upgrade_status -in @("failed", "rolled_back")) { throw "upgrade failed: $($player.upgrade_error)" }
    Start-Sleep -Seconds 1
  }
  if ($player.client_version -ne "1.1.0" -or -not $player.online) { throw "upgraded client did not reconnect successfully" }
  $newHash = (Get-FileHash -LiteralPath $clientExe -Algorithm SHA256).Hash
  if ($newHash -eq $oldHash) { throw "client executable was not replaced" }
  Start-Sleep -Seconds 7
  if (Test-Path (Join-Path $testDir ".gpipe-update\pending.json")) { throw "completed upgrade left a pending state file" }
  if (Get-ChildItem (Join-Path $testDir ".gpipe-update") -Filter "candidate-*" -ErrorAction SilentlyContinue) { throw "completed upgrade left a stale candidate" }
  if (Get-ChildItem $testDir -Filter "gpipe-client.exe.backup-*" -ErrorAction SilentlyContinue) { throw "completed upgrade left a stale backup" }
  Write-Host "Windows remote upgrade smoke test passed: 1.0.0 -> 1.1.0"
} finally {
  if ($client -and -not $client.HasExited) { Stop-Process -Id $client.Id -Force -ErrorAction SilentlyContinue }
  Get-CimInstance Win32_Process -ErrorAction SilentlyContinue |
    Where-Object { $_.ExecutablePath -eq $clientExe } |
    ForEach-Object { Stop-Process -Id $_.ProcessId -Force -ErrorAction SilentlyContinue }
  if ($server -and -not $server.HasExited) { Stop-Process -Id $server.Id -Force -ErrorAction SilentlyContinue }
  Write-Host "Upgrade smoke artifacts: $testDir"
}
