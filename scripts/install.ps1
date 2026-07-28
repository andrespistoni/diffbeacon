[CmdletBinding()]
param(
    [string]$InstallDir = "",
    [switch]$NoPathUpdate
)

$ErrorActionPreference = "Stop"

if ([string]::IsNullOrWhiteSpace($InstallDir)) {
    if (-not [string]::IsNullOrWhiteSpace($env:DIFFBEACON_INSTALL_DIR)) {
        $InstallDir = $env:DIFFBEACON_INSTALL_DIR
    } elseif (-not [string]::IsNullOrWhiteSpace($env:LOCALAPPDATA)) {
        $InstallDir = Join-Path $env:LOCALAPPDATA "DiffBeacon\bin"
    } else {
        $InstallDir = Join-Path $HOME ".local\bin"
    }
}

if ($null -eq (Get-Command git -ErrorAction SilentlyContinue)) {
    throw "Required command not found: git"
}

$scriptRoot = if ([string]::IsNullOrWhiteSpace($PSScriptRoot)) { $null } else { $PSScriptRoot }
$bundledBinary = if ($null -eq $scriptRoot) { $null } else { Join-Path $scriptRoot "diffbeacon.exe" }
$repoRoot = if ($null -eq $scriptRoot) { $null } else { Split-Path -Parent $scriptRoot }
$sourceTree = if ($null -eq $repoRoot) { $null } else { Join-Path $repoRoot "go.mod" }
$hasBundledBinary = $null -ne $bundledBinary -and (Test-Path -LiteralPath $bundledBinary -PathType Leaf)
$hasSourceTree = $null -ne $sourceTree -and (Test-Path -LiteralPath $sourceTree -PathType Leaf)
$go = Get-Command go -ErrorAction SilentlyContinue
if (-not $hasBundledBinary -and $hasSourceTree -and $null -eq $go) {
    throw "Source checkout detected and required command is unavailable: go"
}

$tempDir = Join-Path ([System.IO.Path]::GetTempPath()) ("diffbeacon-install-" + [guid]::NewGuid().ToString("N"))
$tempBinary = Join-Path $tempDir "diffbeacon.exe"
$target = Join-Path $InstallDir "diffbeacon.exe"
$pathMarker = Join-Path $InstallDir ".diffbeacon-path-added"

New-Item -ItemType Directory -Path $tempDir -Force | Out-Null
try {
    if ($hasBundledBinary) {
        Write-Host "Installing bundled DiffBeacon..."
        Copy-Item -LiteralPath $bundledBinary -Destination $tempBinary
    } elseif ($hasSourceTree) {
        Write-Host "Building DiffBeacon..."
        Push-Location $repoRoot
        $previousCgo = $env:CGO_ENABLED
        try {
            $env:CGO_ENABLED = "0"
            & $go.Source build -trimpath -o $tempBinary ./cmd/diffbeacon
            if ($LASTEXITCODE -ne 0) {
                throw "go build failed with exit code $LASTEXITCODE"
            }
        } finally {
            $env:CGO_ENABLED = $previousCgo
            Pop-Location
        }
    } else {
        $architecture = switch ([System.Runtime.InteropServices.RuntimeInformation]::OSArchitecture.ToString()) {
            "X64" { "amd64" }
            "Arm64" { "arm64" }
            default { throw "Unsupported Windows architecture: $_" }
        }
        $headers = @{
            Accept = "application/vnd.github+json"
            "User-Agent" = "diffbeacon-installer"
        }
        Write-Host "Resolving the latest DiffBeacon release..."
        $release = Invoke-RestMethod `
            -Uri "https://api.github.com/repos/andrespistoni/diffbeacon/releases/latest" `
            -Headers $headers
        $tag = [string]$release.tag_name
        if ($tag -notmatch '^v([0-9]+\.[0-9]+\.[0-9]+)$') {
            throw "Latest release has an unsupported tag: $tag"
        }
        $version = $Matches[1]
        $archive = "diffbeacon_${version}_windows_${architecture}.zip"
        $downloadBase = "https://github.com/andrespistoni/diffbeacon/releases/download/$tag"
        $archivePath = Join-Path $tempDir $archive
        $checksumsPath = Join-Path $tempDir "SHA256SUMS"

        Write-Host "Downloading DiffBeacon $version for windows/$architecture..."
        Invoke-WebRequest -Uri "$downloadBase/$archive" -OutFile $archivePath
        Invoke-WebRequest -Uri "$downloadBase/SHA256SUMS" -OutFile $checksumsPath

        $checksumLine = Get-Content -LiteralPath $checksumsPath |
            Where-Object { $_ -match "^[0-9a-fA-F]{64}\s+$([regex]::Escape($archive))$" } |
            Select-Object -First 1
        if ([string]::IsNullOrWhiteSpace($checksumLine)) {
            throw "Checksum not found for $archive"
        }
        $expected = ($checksumLine -split '\s+')[0].ToLowerInvariant()
        $actual = (Get-FileHash -LiteralPath $archivePath -Algorithm SHA256).Hash.ToLowerInvariant()
        if ($actual -ne $expected) {
            throw "SHA-256 mismatch for $archive"
        }

        $packageDir = Join-Path $tempDir "package"
        Expand-Archive -LiteralPath $archivePath -DestinationPath $packageDir
        Copy-Item -LiteralPath (Join-Path $packageDir "diffbeacon.exe") -Destination $tempBinary
    }

    New-Item -ItemType Directory -Path $InstallDir -Force | Out-Null
    Copy-Item -LiteralPath $tempBinary -Destination $target -Force
} finally {
    Remove-Item -LiteralPath $tempDir -Recurse -Force -ErrorAction SilentlyContinue
}

& $target --version
if ($LASTEXITCODE -ne 0) {
    throw "Installed binary validation failed with exit code $LASTEXITCODE"
}

if (-not $NoPathUpdate) {
    $userPath = [Environment]::GetEnvironmentVariable("Path", "User")
    $entries = @($userPath -split ";" | Where-Object { -not [string]::IsNullOrWhiteSpace($_) })
    $alreadyPresent = $false
    foreach ($entry in $entries) {
        if ([string]::Equals($entry.TrimEnd("\"), $InstallDir.TrimEnd("\"), [StringComparison]::OrdinalIgnoreCase)) {
            $alreadyPresent = $true
            break
        }
    }
    if (-not $alreadyPresent) {
        $newUserPath = if ([string]::IsNullOrWhiteSpace($userPath)) { $InstallDir } else { "$userPath;$InstallDir" }
        [System.IO.File]::WriteAllText($pathMarker, "")
        try {
            [Environment]::SetEnvironmentVariable("Path", $newUserPath, "User")
            $env:Path = "$env:Path;$InstallDir"
        } catch {
            Remove-Item -LiteralPath $pathMarker -Force -ErrorAction SilentlyContinue
            throw
        }
        Write-Host "Added $InstallDir to the user PATH. Open a new terminal to use it everywhere."
    }
}

Write-Host "Installed DiffBeacon at $target"
