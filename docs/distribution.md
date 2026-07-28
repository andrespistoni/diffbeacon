# Distribución manual

Este flujo permite compartir DiffBeacon sin exigir Go en la máquina destino. Git
2.31.0 o posterior sigue siendo una dependencia de ejecución.

## Generar paquetes

La versión se recibe sin prefijo `v`. Para construir localmente los seis
binarios, empaquetarlos y verificar sus checksums:

```sh
make dist VERSION=0.1.1
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

## Publicar

El workflow `.github/workflows/release.yml` publica una GitHub Release cuando se
envía un tag `vMAJOR.MINOR.PATCH`. El tag es la fuente de verdad de la versión y
debe coincidir con una sección fechada de `CHANGELOG.md`; el workflow ejecuta
`make release-check`, genera procedencia verificable y adjunta todo `dist/`.

Ejemplo, sólo después de completar la checklist de release:

```sh
git tag v0.1.1
git push origin v0.1.1
```

No agregues `dist/` al historial Git ni publiques una release manual en paralelo
con el workflow.

Windows continúa siendo experimental hasta validar la TUI completa en un host
Windows. Los binarios Windows y macOS no están firmados, por lo que SmartScreen o
Gatekeeper pueden mostrar advertencias al distribuirlos fuera de un entorno de
confianza.
