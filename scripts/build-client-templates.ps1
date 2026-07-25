param(
    [string]$OutputDir = ".\client-templates",
    [string]$Version = "1.0.0"
)

$ErrorActionPreference = "Stop"
$repoRoot = Split-Path -Parent $PSScriptRoot
if ($Version -notmatch '^(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)(?:-[0-9A-Za-z.-]+)?(?:\+[0-9A-Za-z.-]+)?$') {
    throw "Version must be a semantic version, got: $Version"
}
$resolvedOutputDir = if ([System.IO.Path]::IsPathRooted($OutputDir)) {
    [System.IO.Path]::GetFullPath($OutputDir)
} else {
    [System.IO.Path]::GetFullPath((Join-Path (Get-Location) $OutputDir))
}

$targets = @(
    @{ Id = "windows-amd64"; GOOS = "windows"; GOARCH = "amd64"; Output = "gpipe-client-template-windows-amd64.exe" },
    @{ Id = "windows-arm64"; GOOS = "windows"; GOARCH = "arm64"; Output = "gpipe-client-template-windows-arm64.exe" },
    @{ Id = "linux-amd64"; GOOS = "linux"; GOARCH = "amd64"; Output = "gpipe-client-template-linux-amd64" },
    @{ Id = "linux-arm64"; GOOS = "linux"; GOARCH = "arm64"; Output = "gpipe-client-template-linux-arm64" },
    @{ Id = "linux-armv7"; GOOS = "linux"; GOARCH = "arm"; GOARM = "7"; Output = "gpipe-client-template-linux-armv7" }
)

function Get-EmbeddedConfigPlaceholder() {
    $value = (& go run ./scripts/print-client-placeholder) -join ""
    if ($LASTEXITCODE -ne 0) {
        throw "go run ./scripts/print-client-placeholder failed"
    }
    return $value.Trim()
}

New-Item -ItemType Directory -Path $resolvedOutputDir -Force | Out-Null

$oldGoOS = $env:GOOS
$oldGoArch = $env:GOARCH
$oldGoArm = $env:GOARM
$oldCGO = $env:CGO_ENABLED

Push-Location $repoRoot
try {
    $placeholder = Get-EmbeddedConfigPlaceholder
    foreach ($target in $targets) {
        $outputPath = Join-Path $resolvedOutputDir $target.Output
        $ldflags = "-s -w -X main.embeddedClientConfig=$placeholder -X main.clientVersion=$Version -X main.clientPlatform=$($target.Id)"

        Write-Host "Building client template $($target.Id) -> $outputPath"
        $env:CGO_ENABLED = "0"
        $env:GOOS = $target.GOOS
        $env:GOARCH = $target.GOARCH
        if ($null -ne $target.GOARM -and -not [string]::IsNullOrWhiteSpace($target.GOARM)) {
            $env:GOARM = $target.GOARM
        } else {
            Remove-Item Env:\GOARM -ErrorAction SilentlyContinue
        }
        go build -trimpath -buildvcs=false -ldflags $ldflags -o $outputPath .\cmd\client
        if ($LASTEXITCODE -ne 0) {
            throw "go build failed for $($target.Id)"
        }
    }
} finally {
    if ($null -ne $oldGoOS) { $env:GOOS = $oldGoOS } else { Remove-Item Env:\GOOS -ErrorAction SilentlyContinue }
    if ($null -ne $oldGoArch) { $env:GOARCH = $oldGoArch } else { Remove-Item Env:\GOARCH -ErrorAction SilentlyContinue }
    if ($null -ne $oldGoArm) { $env:GOARM = $oldGoArm } else { Remove-Item Env:\GOARM -ErrorAction SilentlyContinue }
    if ($null -ne $oldCGO) { $env:CGO_ENABLED = $oldCGO } else { Remove-Item Env:\CGO_ENABLED -ErrorAction SilentlyContinue }
    Pop-Location
}

$artifacts = @()
foreach ($target in $targets) {
    $artifactPath = Join-Path $resolvedOutputDir $target.Output
    $fileInfo = Get-Item -LiteralPath $artifactPath
    $artifacts += [ordered]@{
        target = $target.Id
        file = $target.Output
        size = $fileInfo.Length
        sha256 = (Get-FileHash -LiteralPath $artifactPath -Algorithm SHA256).Hash.ToLowerInvariant()
    }
}
$manifest = [ordered]@{
    version = $Version
    updater_protocol = 1
    generated_at = [DateTime]::UtcNow.ToString("o")
    artifacts = $artifacts
}
$manifestPath = Join-Path $resolvedOutputDir "manifest.json"
[System.IO.File]::WriteAllText($manifestPath, ($manifest | ConvertTo-Json -Depth 10), (New-Object System.Text.UTF8Encoding($false)))

Write-Host "Client templates are ready in $resolvedOutputDir"
