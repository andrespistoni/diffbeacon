# Distribución manual

Este flujo permite compartir DiffBeacon sin exigir Go en la máquina destino. Git
2.31.0 o posterior sigue siendo una dependencia de ejecución.

## Generar paquetes

`VERSION` contiene la versión `MAJOR.MINOR.PATCH` sin prefijo `v`. Para construir
los seis binarios, empaquetarlos y verificar sus checksums:

```sh
make version
make dist
```

`dist/` contiene cuatro `tar.gz` para Linux/macOS, dos ZIP para Windows,
`SHA256SUMS`, SBOM y procedencia local. Los binarios raw usados para inspección
quedan en `build/release/`. Cada paquete incluye también el texto de la licencia
MIT.

## Verificar

Linux:

```sh
cd dist
sha256sum --check SHA256SUMS
```

macOS:

```sh
cd dist
shasum -a 256 -c SHA256SUMS
```

Windows puede comparar cada archivo con:

```powershell
Get-FileHash .\diffbeacon_0.1.1_windows_amd64.zip -Algorithm SHA256
```

## Instalar un paquete Unix

```sh
tar -xzf diffbeacon_0.1.1_linux_amd64.tar.gz
sh install.sh
diffbeacon --version
```

El instalador usa `~/.local/bin` salvo que se indique
`DIFFBEACON_INSTALL_DIR` o `--install-dir DIR`.

## Instalar un paquete Windows

```powershell
Expand-Archive .\diffbeacon_0.1.1_windows_amd64.zip -DestinationPath .\diffbeacon
.\diffbeacon\install.ps1
diffbeacon --version
```

El instalador usa `%LOCALAPPDATA%\DiffBeacon\bin`. `-InstallDir` cambia el destino
y `-NoPathUpdate` evita modificar el `PATH`.

## Compartir

Los archivos de `dist/` pueden adjuntarse manualmente a una GitHub Release o
compartirse por un canal interno. No deben añadirse al historial Git. Para una
release manual con GitHub CLI, después de crear y publicar el tag correspondiente:

```sh
gh release create v0.1.1 dist/* --title "DiffBeacon v0.1.1" --generate-notes
```

Windows continúa siendo experimental hasta validar la TUI completa en un host
Windows. Los binarios Windows y macOS no están firmados, por lo que SmartScreen o
Gatekeeper pueden mostrar advertencias al distribuirlos fuera de un entorno de
confianza.
