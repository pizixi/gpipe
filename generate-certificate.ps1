[CmdletBinding()]
param(
    [string[]]$DnsName = @("localhost"),
    [string[]]$IpAddress = @("127.0.0.1", "::1"),
    [int]$CertDays = 825,
    [int]$CaDays = 3650,
    [switch]$Force
)

$ErrorActionPreference = "Stop"

$outputDir = Join-Path $PSScriptRoot "certs"

function New-SerialBytes {
    param([int]$Length = 16)

    $bytes = New-Object byte[] $Length
    $rng = [System.Security.Cryptography.RandomNumberGenerator]::Create()
    try {
        $rng.GetBytes($bytes)
    }
    finally {
        $rng.Dispose()
    }
    if ($bytes[0] -eq 0) {
        $bytes[0] = 1
    }
    return $bytes
}

function ConvertTo-Pem {
    param(
        [Parameter(Mandatory = $true)]
        [string]$Label,
        [Parameter(Mandatory = $true)]
        [byte[]]$Bytes
    )

    $base64 = [Convert]::ToBase64String($Bytes)
    $lines = [System.Collections.Generic.List[string]]::new()
    $lines.Add("-----BEGIN $Label-----") | Out-Null
    for ($offset = 0; $offset -lt $base64.Length; $offset += 64) {
        $count = [Math]::Min(64, $base64.Length - $offset)
        $lines.Add($base64.Substring($offset, $count)) | Out-Null
    }
    $lines.Add("-----END $Label-----") | Out-Null
    return ($lines -join [Environment]::NewLine) + [Environment]::NewLine
}

function Write-PemFile {
    param(
        [Parameter(Mandatory = $true)]
        [string]$Path,
        [Parameter(Mandatory = $true)]
        [string]$Label,
        [Parameter(Mandatory = $true)]
        [byte[]]$Bytes
    )

    Set-Content -LiteralPath $Path -Value (ConvertTo-Pem -Label $Label -Bytes $Bytes) -Encoding ascii
}

function Export-PrivateKeyPemData {
    param(
        [Parameter(Mandatory = $true)]
        [System.Security.Cryptography.RSA]$Key
    )

    if ($Key.PSObject.Methods.Name -contains "ExportPkcs8PrivateKey") {
        return [pscustomobject]@{
            Label = "PRIVATE KEY"
            Bytes = $Key.ExportPkcs8PrivateKey()
        }
    }

    if ($Key -is [System.Security.Cryptography.RSACng]) {
        return [pscustomobject]@{
            Label = "PRIVATE KEY"
            Bytes = $Key.Key.Export([System.Security.Cryptography.CngKeyBlobFormat]::Pkcs8PrivateBlob)
        }
    }

    if ($Key.PSObject.Methods.Name -contains "ExportRSAPrivateKey") {
        return [pscustomobject]@{
            Label = "RSA PRIVATE KEY"
            Bytes = $Key.ExportRSAPrivateKey()
        }
    }

    throw "This PowerShell/.NET runtime cannot export RSA private keys as PEM."
}

$rootCaKey = Join-Path $outputDir "root-ca.key.pem"
$rootCaCert = Join-Path $outputDir "root-ca.pem"
$serverKey = Join-Path $outputDir "server.key.pem"
$serverCert = Join-Path $outputDir "cert.pem"

New-Item -ItemType Directory -Path $outputDir -Force | Out-Null

$dnsNames = @($DnsName | Where-Object { $_ -and $_.Trim() -ne "" } | ForEach-Object { $_.Trim() })
if ($dnsNames.Count -eq 0) {
    throw "At least one DNS SAN is required."
}
$ipAddresses = @($IpAddress | Where-Object { $_ -and $_.Trim() -ne "" } | ForEach-Object { $_.Trim() })

if (-not $Force) {
    foreach ($path in @($rootCaKey, $rootCaCert, $serverKey, $serverCert)) {
        if (Test-Path -LiteralPath $path) {
            throw "Refusing to overwrite existing file: $path. Re-run with -Force to replace the generated certificate set."
        }
    }
}

$hashAlgorithm = [System.Security.Cryptography.HashAlgorithmName]::SHA256
$signaturePadding = [System.Security.Cryptography.RSASignaturePadding]::Pkcs1
$notBefore = [System.DateTimeOffset]::UtcNow.AddMinutes(-5)

$caKey = [System.Security.Cryptography.RSA]::Create(2048)
$caDn = [System.Security.Cryptography.X509Certificates.X500DistinguishedName]::new("CN=gpipe Dev Root CA, O=gpipe Dev Root CA, S=Local, C=CN")
$caRequest = [System.Security.Cryptography.X509Certificates.CertificateRequest]::new($caDn, $caKey, $hashAlgorithm, $signaturePadding)
$caRequest.CertificateExtensions.Add([System.Security.Cryptography.X509Certificates.X509BasicConstraintsExtension]::new($true, $false, 0, $true))
$caRequest.CertificateExtensions.Add([System.Security.Cryptography.X509Certificates.X509KeyUsageExtension]::new([System.Security.Cryptography.X509Certificates.X509KeyUsageFlags]::KeyCertSign -bor [System.Security.Cryptography.X509Certificates.X509KeyUsageFlags]::CrlSign -bor [System.Security.Cryptography.X509Certificates.X509KeyUsageFlags]::DigitalSignature, $true))
$caRequest.CertificateExtensions.Add([System.Security.Cryptography.X509Certificates.X509SubjectKeyIdentifierExtension]::new($caRequest.PublicKey, $false))
$caCertificate = $caRequest.CreateSelfSigned($notBefore, $notBefore.AddDays($CaDays))

$serverKeyObj = [System.Security.Cryptography.RSA]::Create(2048)
$serverDn = [System.Security.Cryptography.X509Certificates.X500DistinguishedName]::new("CN=$($dnsNames[0]), O=gpipe Dev Server, S=Local, C=CN")
$serverRequest = [System.Security.Cryptography.X509Certificates.CertificateRequest]::new($serverDn, $serverKeyObj, $hashAlgorithm, $signaturePadding)
$serverRequest.CertificateExtensions.Add([System.Security.Cryptography.X509Certificates.X509BasicConstraintsExtension]::new($false, $false, 0, $true))
$serverRequest.CertificateExtensions.Add([System.Security.Cryptography.X509Certificates.X509KeyUsageExtension]::new([System.Security.Cryptography.X509Certificates.X509KeyUsageFlags]::DigitalSignature -bor [System.Security.Cryptography.X509Certificates.X509KeyUsageFlags]::KeyEncipherment, $true))
$serverEku = [System.Security.Cryptography.OidCollection]::new()
$null = $serverEku.Add([System.Security.Cryptography.Oid]::new("1.3.6.1.5.5.7.3.1", "Server Authentication"))
$serverRequest.CertificateExtensions.Add([System.Security.Cryptography.X509Certificates.X509EnhancedKeyUsageExtension]::new($serverEku, $true))
$sanBuilder = [System.Security.Cryptography.X509Certificates.SubjectAlternativeNameBuilder]::new()
foreach ($dns in $dnsNames) {
    $sanBuilder.AddDnsName($dns)
}
foreach ($ip in $ipAddresses) {
    $sanBuilder.AddIpAddress([System.Net.IPAddress]::Parse($ip))
}
$serverRequest.CertificateExtensions.Add($sanBuilder.Build($false))
$serverRequest.CertificateExtensions.Add([System.Security.Cryptography.X509Certificates.X509SubjectKeyIdentifierExtension]::new($serverRequest.PublicKey, $false))
$serverCertificateNoKey = $serverRequest.Create($caCertificate, $notBefore, $notBefore.AddDays($CertDays), (New-SerialBytes))
$rootKeyPem = Export-PrivateKeyPemData -Key $caKey
$serverKeyPem = Export-PrivateKeyPemData -Key $serverKeyObj

Write-Host "Writing root CA certificate: $rootCaCert"
Write-PemFile -Path $rootCaCert -Label "CERTIFICATE" -Bytes $caCertificate.Export([System.Security.Cryptography.X509Certificates.X509ContentType]::Cert)

Write-Host "Writing root CA private key: $rootCaKey"
Write-PemFile -Path $rootCaKey -Label $rootKeyPem.Label -Bytes $rootKeyPem.Bytes

Write-Host "Writing server certificate: $serverCert"
Write-PemFile -Path $serverCert -Label "CERTIFICATE" -Bytes $serverCertificateNoKey.Export([System.Security.Cryptography.X509Certificates.X509ContentType]::Cert)

Write-Host "Writing server private key: $serverKey"
Write-PemFile -Path $serverKey -Label $serverKeyPem.Label -Bytes $serverKeyPem.Bytes

$caCertificate.Dispose()
$serverCertificateNoKey.Dispose()
$caKey.Dispose()
$serverKeyObj.Dispose()

Write-Host ""
Write-Host "Done."
Write-Host "Output directory: $outputDir"
Write-Host "Generated files:"
Write-Host "  - $rootCaCert      (CA certificate)"
Write-Host "  - $rootCaKey       (CA private key, keep private)"
Write-Host "  - $serverCert      (server certificate, use as tls_cert)"
Write-Host "  - $serverKey       (server private key, use as tls_key)"
Write-Host ""
Write-Host "Server config:"
Write-Host '  "enable_tls": true'
Write-Host '  "tls_cert": "./certs/cert.pem"'
Write-Host '  "tls_key": "./certs/server.key.pem"'
Write-Host ""
Write-Host "Covered DNS SANs: $($dnsNames -join ',')"
Write-Host "Covered IP SANs:  $($ipAddresses -join ',')"
