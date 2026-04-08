param(
    [string]$OutputDir = ".\release",
    [string]$ConfigPath = ".\gpipe.json",
    [string]$ServerGOOS = "",
    [string]$ServerGOARCH = "",
    [switch]$SkipTemplates,
    [switch]$SkipCerts,
    [switch]$Clean
)

$ErrorActionPreference = "Stop"

$repoRoot = Split-Path -Parent $PSScriptRoot

function Resolve-RepoPath([string]$Path) {
    if ([string]::IsNullOrWhiteSpace($Path)) {
        return $repoRoot
    }
    if ([System.IO.Path]::IsPathRooted($Path)) {
        return [System.IO.Path]::GetFullPath($Path)
    }
    return [System.IO.Path]::GetFullPath((Join-Path $repoRoot $Path))
}

function Set-JsonProperty($Object, [string]$Name, $Value) {
    $property = $Object.PSObject.Properties[$Name]
    if ($null -eq $property) {
        $Object | Add-Member -NotePropertyName $Name -NotePropertyValue $Value
        return
    }
    $property.Value = $Value
}

function Get-GoEnvValue([string]$Name) {
    $value = & go env $Name
    if ($LASTEXITCODE -ne 0) {
        throw "go env $Name failed"
    }
    return ($value | Out-String).Trim()
}

function Invoke-GoBuild([string]$GoOS, [string]$GoArch, [string]$OutputPath, [string]$PackagePath) {
    $oldGoOS = $env:GOOS
    $oldGoArch = $env:GOARCH
    $oldCGO = $env:CGO_ENABLED
    try {
        $env:CGO_ENABLED = "0"
        if (-not [string]::IsNullOrWhiteSpace($GoOS)) {
            $env:GOOS = $GoOS
        }
        if (-not [string]::IsNullOrWhiteSpace($GoArch)) {
            $env:GOARCH = $GoArch
        }
        & go build -trimpath -buildvcs=false -ldflags "-s -w" -o $OutputPath $PackagePath
        if ($LASTEXITCODE -ne 0) {
            throw "go build failed for $PackagePath"
        }
    } finally {
        if ($null -ne $oldGoOS) { $env:GOOS = $oldGoOS } else { Remove-Item Env:\GOOS -ErrorAction SilentlyContinue }
        if ($null -ne $oldGoArch) { $env:GOARCH = $oldGoArch } else { Remove-Item Env:\GOARCH -ErrorAction SilentlyContinue }
        if ($null -ne $oldCGO) { $env:CGO_ENABLED = $oldCGO } else { Remove-Item Env:\CGO_ENABLED -ErrorAction SilentlyContinue }
    }
}

function Write-Utf8NoBom([string]$Path, [string]$Content) {
    [System.IO.File]::WriteAllText($Path, $Content, (New-Object System.Text.UTF8Encoding($false)))
}

$resolvedOutputDir = Resolve-RepoPath $OutputDir
$resolvedConfigPath = Resolve-RepoPath $ConfigPath

if (-not (Test-Path -LiteralPath $resolvedConfigPath)) {
    throw "config file not found: $resolvedConfigPath"
}

if ($Clean -and (Test-Path -LiteralPath $resolvedOutputDir)) {
    Remove-Item -LiteralPath $resolvedOutputDir -Recurse -Force
}

New-Item -ItemType Directory -Force -Path $resolvedOutputDir | Out-Null

$binDir = Join-Path $resolvedOutputDir ""
$templateDir = Join-Path $resolvedOutputDir "client-templates"
$cacheDir = Join-Path $resolvedOutputDir "client-cache"
$logsDir = Join-Path $resolvedOutputDir "logs"
$certsDir = Join-Path $resolvedOutputDir "certs"

New-Item -ItemType Directory -Force -Path $binDir, $templateDir, $cacheDir, $logsDir | Out-Null

$targetGoOS = if ([string]::IsNullOrWhiteSpace($ServerGOOS)) { Get-GoEnvValue "GOHOSTOS" } else { $ServerGOOS.Trim() }
$targetGoArch = if ([string]::IsNullOrWhiteSpace($ServerGOARCH)) { Get-GoEnvValue "GOHOSTARCH" } else { $ServerGOARCH.Trim() }
$serverName = if ($targetGoOS -eq "windows") { "gpipe-server.exe" } else { "gpipe-server" }
$serverOutputPath = Join-Path $binDir $serverName

Write-Host "Building server $targetGoOS/$targetGoArch -> $serverOutputPath"
Push-Location $repoRoot
try {
    Invoke-GoBuild -GoOS $targetGoOS -GoArch $targetGoArch -OutputPath $serverOutputPath -PackagePath ".\cmd\server"

    if (-not $SkipTemplates) {
        Write-Host "Building client templates -> $templateDir"
        & (Join-Path $PSScriptRoot "build-client-templates.ps1") -OutputDir $templateDir
        if ($LASTEXITCODE -ne 0) {
            throw "build-client-templates.ps1 failed"
        }
    } else {
        Write-Host "Skipping client templates"
    }
} finally {
    Pop-Location
}

$configObject = Get-Content -LiteralPath $resolvedConfigPath -Raw | ConvertFrom-Json
Set-JsonProperty -Object $configObject -Name "database_url" -Value "sqlite://gpipe.db?mode=rwc"
Set-JsonProperty -Object $configObject -Name "client_template_dir" -Value "./client-templates"
Set-JsonProperty -Object $configObject -Name "client_artifact_cache_dir" -Value "./client-cache"
Set-JsonProperty -Object $configObject -Name "log_dir" -Value "logs"
if ($null -eq $configObject.PSObject.Properties["quiet"]) {
    Set-JsonProperty -Object $configObject -Name "quiet" -Value $false
}

$releaseConfigPath = Join-Path $resolvedOutputDir "gpipe.json"
Write-Utf8NoBom -Path $releaseConfigPath -Content ($configObject | ConvertTo-Json -Depth 20)

$dbPath = Join-Path $resolvedOutputDir "gpipe.db"
if (-not (Test-Path -LiteralPath $dbPath)) {
    New-Item -ItemType File -Path $dbPath | Out-Null
}

$sourceCerts = Join-Path $repoRoot "certs"
if (-not $SkipCerts -and (Test-Path -LiteralPath $sourceCerts)) {
    Copy-Item -LiteralPath $sourceCerts -Destination $certsDir -Recurse -Force
    Write-Host "Copied certs -> $certsDir"
} elseif ($SkipCerts) {
    Write-Host "Skipping certs"
} else {
    Write-Host "Certs directory not found, skipping"
}

Write-Host ""
Write-Host "Release package is ready:"
Write-Host "  $resolvedOutputDir"
