# Compatibilidad de DiffBeacon v0.1

## Versión mínima de Git

La versión mínima publicada es **Git 2.31.0**. Este valor es un piso de
validación: no afirma que una versión anterior sea necesariamente incapaz de
ejecutar el programa. Se eligió 2.31.0 porque es la versión más antigua contra
la que se ejecutó la suite normativa completa, incluidos repositorios sin
`HEAD`, worktrees, status porcelain v2, contenido por scope, observación de
stage/unstage externo de archivo, hunk y conjunto, partial clones con blobs
prometidos ausentes, locks, paths especiales, E2E read-only y ciclo de vida de la
TUI.

El arranque ejecuta `git --version` antes de descubrir el repositorio y abrir la
TUI. Una versión menor, o una salida que no pueda interpretarse, termina con un
diagnóstico explícito. Las pruebas unitarias cubren exactamente 2.31.0, una
versión menor y una salida inválida. Esta comprobación evita presentar como
compatible una instalación que no participó en el piso validado.

`scripts/test-git-matrix.sh` acepta uno o más ejecutables Git y ejecuta
`go test ./... -count=1` con cada uno como primer `git` de `PATH`. Sin argumentos
prueba el Git local y avisa si no recibió 2.31.0; CI compila 2.31.0 desde el
tarball oficial de kernel.org y prueba además el Git del runner. El workflow fija
el SHA-256 de `git-2.31.0.tar.xz`, verifica la firma del tar descomprimido con el
subkey OpenPGP `E1F036B1FEE7221FC778ECEFB0B5E88696AFE6CB` y solo entonces compila.

## Evidencia y matriz

| Entorno | Git | Cobertura | Estado |
|---|---:|---|---|
| Linux amd64, WSL2, ejecución local del 2026-07-26 | 2.31.0 compilado localmente | `go test ./... -count=1` completo | Ejecutado correctamente |
| Linux amd64, WSL2, ejecución local del 2026-07-26 | 2.53.0 del sistema | `go test ./... -count=1` completo | Ejecutado correctamente |
| `ubuntu-latest` | 2.31.0 compilado + Git del runner | misma matriz mediante script | Cubierto por workflow; no ejecutado desde esta sesión |
| `ubuntu-latest`, `macos-latest` | Git del runner | vet, tests, E2E, race y build nativo | Cubierto por workflow; no ejecutado desde esta sesión |

La evidencia local no sustituye el resultado de GitHub Actions. Las versiones
del Git preinstalado y la arquitectura exacta de los runners hospedados pueden
cambiar; sus logs son la evidencia para cada ejecución.

La protección promisor común a ambas versiones es `GIT_ALLOW_PROTOCOL` con una
allowlist vacía, reforzada por `GIT_PROTOCOL_FROM_USER=0` y
`protocol.allow=never`. La reproducción directa mostró que
`GIT_NO_LAZY_FETCH=1` sí evita el intento en Git 2.53.0 pero Git 2.31.0 la ignora
y llega a buscar `remote-http`; por eso DiffBeacon no depende ni establece esa
variable. La integración configura deliberadamente `protocol.allow=always` y
`protocol.probe.allow=always`: el control de entorno sigue evitando el proceso de
transporte y conserva byte a byte Git dir, objects, packs, refs, config, índice y
working tree.

La carga de contenido añade una barrera independiente de versión: antes de
`cat-file`, comprueba el OID en loose objects, pack indexes y alternates locales.
Un OID ausente no inicia siquiera ese proceso Git. La denegación de protocolos
permanece como defensa para una eliminación concurrente posterior a la
comprobación.

Para la ejecución local, Git 2.31.0 se compiló sin curl, OpenSSL, expat, gettext
ni Tk y contra zlib 1.3.1 compilado en `/tmp`, porque la sesión no tenía permisos
para instalar headers del sistema. La prueba de helper de transporte personalizado
no depende de curl y demuestra el bloqueo de proceso en esa build; la prueba HTTP
con Git actual añade observación directa de conexión. El job CI instala las
dependencias de build y compila la distribución normal de 2.31.0 (solo omite Tk).

## Plataformas y artefactos iniciales

La matriz validada de producto continúa siendo Linux y macOS. `make build-all`
genera con `CGO_ENABLED=0` seis binarios raw en `build/release/`:

- `diffbeacon_linux_amd64`
- `diffbeacon_linux_arm64`
- `diffbeacon_darwin_amd64`
- `diffbeacon_darwin_arm64`
- `diffbeacon_windows_amd64.exe`
- `diffbeacon_windows_arm64.exe`

La compilación cruzada comprueba cada artefacto mediante `go version -m`.
`make release-metadata` añade checksums, SBOM SPDX 2.3 y procedencia in-toto a
los builds raw. `make dist` genera cuatro `tar.gz` Unix y dos ZIP Windows en
`dist/`, con checksums sobre los paquetes finales. CI añade Syft y attestación
OIDC. La CI ejecuta la suite en hosts Linux y macOS. En esta sesión se generaron
y verificaron los seis cross-builds en Linux; no se
ejecutó localmente un binario macOS. Windows no forma parte de la matriz validada
de v0.1. Los cross-builds Windows amd64/arm64 compilan y se incluyen scripts PowerShell
para evaluación. CI valida su ciclo de instalación/desinstalación en
`windows-latest`, pero no se declara soporte oficial hasta ejecutar la suite y la
TUI completa en un host Windows.

## Dependencias y límites de la afirmación

Los artefactos se construyen sin cgo. Git es la única dependencia externa de
ejecución declarada. No se requiere shell, red, cuenta, servicio remoto ni
archivo de configuración para usar DiffBeacon. Los scripts de desarrollo y el
workflow sí usan shell; no forman parte del binario ni de su ejecución.

El producto usa únicamente operaciones locales de consulta read-only. Que Git
2.31.0 supere esta matriz no constituye soporte para operaciones remotas ni para
comandos fuera del alcance de DiffBeacon.
