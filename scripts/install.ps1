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

$bundledBinary = Join-Path $PSScriptRoot "diffbeacon.exe"
$go = Get-Command go -ErrorAction SilentlyContinue
if (-not (Test-Path -LiteralPath $bundledBinary -PathType Leaf) -and $null -eq $go) {
    throw "Bundled diffbeacon.exe was not found and required command is unavailable: go"
}

$tempDir = Join-Path ([System.IO.Path]::GetTempPath()) ("diffbeacon-install-" + [guid]::NewGuid().ToString("N"))
$tempBinary = Join-Path $tempDir "diffbeacon.exe"
$target = Join-Path $InstallDir "diffbeacon.exe"
$pathMarker = Join-Path $InstallDir ".diffbeacon-path-added"

New-Item -ItemType Directory -Path $tempDir -Force | Out-Null
try {
    if (Test-Path -LiteralPath $bundledBinary -PathType Leaf) {
        Write-Host "Installing bundled DiffBeacon..."
        Copy-Item -LiteralPath $bundledBinary -Destination $tempBinary
    } else {
        Write-Host "Building DiffBeacon..."
        $repoRoot = Split-Path -Parent $PSScriptRoot
        $version = (Get-Content -LiteralPath (Join-Path $repoRoot "VERSION") -Raw).Trim()
        Push-Location $repoRoot
        $previousCgo = $env:CGO_ENABLED
        try {
            $env:CGO_ENABLED = "0"
            & $go.Source build -trimpath -ldflags "-X main.version=$version -s -w" -o $tempBinary ./cmd/diffbeacon
            if ($LASTEXITCODE -ne 0) {
                throw "go build failed with exit code $LASTEXITCODE"
            }
        } finally {
            $env:CGO_ENABLED = $previousCgo
            Pop-Location
        }
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
