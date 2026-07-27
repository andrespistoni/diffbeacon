[CmdletBinding()]
param(
    [string]$InstallDir = "",
    [switch]$KeepPath
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

$target = Join-Path $InstallDir "diffbeacon.exe"
$pathMarker = Join-Path $InstallDir ".diffbeacon-path-added"
if (Test-Path -LiteralPath $target -PathType Container) {
    throw "Refusing to remove directory: $target"
}
if (Test-Path -LiteralPath $target) {
    Remove-Item -LiteralPath $target -Force
    Write-Host "Removed $target"
} else {
    Write-Host "DiffBeacon is not installed at $target"
}

if (-not $KeepPath -and (Test-Path -LiteralPath $pathMarker -PathType Leaf)) {
    $userPath = [Environment]::GetEnvironmentVariable("Path", "User")
    $entries = @($userPath -split ";" | Where-Object {
        -not [string]::IsNullOrWhiteSpace($_) -and
        -not [string]::Equals($_.TrimEnd("\"), $InstallDir.TrimEnd("\"), [StringComparison]::OrdinalIgnoreCase)
    })
    [Environment]::SetEnvironmentVariable("Path", ($entries -join ";"), "User")

    $processEntries = @($env:Path -split ";" | Where-Object {
        -not [string]::IsNullOrWhiteSpace($_) -and
        -not [string]::Equals($_.TrimEnd("\"), $InstallDir.TrimEnd("\"), [StringComparison]::OrdinalIgnoreCase)
    })
    $env:Path = $processEntries -join ";"

    Write-Host "Removed installer-managed PATH entry for $InstallDir"
}

if (Test-Path -LiteralPath $pathMarker -PathType Leaf) {
    Remove-Item -LiteralPath $pathMarker -Force
}

if ((Test-Path -LiteralPath $InstallDir -PathType Container) -and
    $null -eq (Get-ChildItem -LiteralPath $InstallDir -Force | Select-Object -First 1)) {
    Remove-Item -LiteralPath $InstallDir -Force
}
