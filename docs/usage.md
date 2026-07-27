# Uso de DiffBeacon v0.1

## Instalación desde el código fuente

Requisitos:

- Go 1.26 para compilar;
- Git 2.31.0 o posterior disponible en `PATH` para ejecutar;
- Linux o macOS.

```sh
make build
./bin/diffbeacon
```

Instalación y desinstalación para el usuario actual en Linux/macOS:

```sh
./scripts/install.sh
./scripts/uninstall.sh
```

Por defecto se usa `~/.local/bin`. Puede cambiarse con
`DIFFBEACON_INSTALL_DIR=/otro/directorio` o `--install-dir DIR`. El instalador no
usa `sudo` ni modifica la configuración del shell; avisa cuando el directorio no
está en `PATH`.

Los mismos scripts detectan cuándo están dentro de un paquete precompilado. En
ese caso instalan el binario incluido y no requieren Go:

```sh
tar -xzf diffbeacon_0.1.1_linux_amd64.tar.gz
sh install.sh
diffbeacon --version
```

También puede instalarse desde este checkout con `go install ./cmd/diffbeacon`.
No se requiere red durante la ejecución, cuenta, telemetría ni archivo de
configuración. La descarga inicial de módulos Go sí es una actividad de build, no
del producto en ejecución.

### Windows experimental

Los cross-builds `windows/amd64` y `windows/arm64` compilan, pero Windows todavía no forma parte de la
matriz de ejecución validada para v0.1. Para probarlo desde PowerShell con Go y
Git disponibles en `PATH`:

```powershell
.\scripts\install.ps1
.\scripts\uninstall.ps1
```

El instalador usa `%LOCALAPPDATA%\DiffBeacon\bin` y lo añade al `PATH` del usuario.
Puede indicarse otro directorio con `-InstallDir`. Use `-NoPathUpdate` para no
modificar `PATH`; la desinstalación sólo retira una entrada que haya sido añadida
por el propio instalador. `-KeepPath` permite conservar incluso esa entrada.

Desde un paquete Windows precompilado:

```powershell
Expand-Archive .\diffbeacon_0.1.1_windows_amd64.zip -DestinationPath .\diffbeacon
.\diffbeacon\install.ps1
diffbeacon --version
```

## Inicio

```sh
diffbeacon             # repositorio asociado al cwd
diffbeacon path/to/repo
diffbeacon path/to/repo/subdirectory
```

Solo se admite cero o un path. Puede ser el root, un subdirectorio o un archivo
dentro del repositorio. Repositorios bare, Git ausente/incompatible y paths fuera
de un repositorio terminan antes de abrir la TUI con un diagnóstico.

## Teclado

| Teclas | Acción |
|---|---|
| `j`/`↓`, `k`/`↑` | mover selección o scroll según el foco |
| `Tab`, `Enter`, `Esc` | cambiar foco, entrar al contenido o volver a la lista |
| `[` / `]` | hunk anterior/siguiente |
| `v` | alternar inline/side-by-side |
| `f` | alternar changes-only/full-file |
| `1`, `2`, `3`, `4` | All, Staged, Changes, Untracked |
| `r` | refresh manual |
| `e` | abrir/cerrar detalle del error |
| `?` | ayuda compacta/completa |
| `q` / `Ctrl+C` | salir |

La barra de estado indica scope, layout, densidad, branch, repositorio, hunk,
progreso y errores sin depender solo del color. `s`, `u`, `S` y `U` no tienen
binding ni acción.

## Vistas y degradaciones

| Tipo | Changes only | Full file | Inline | Side-by-side |
|---|---|---|---|---|
| Staged/Unstaged texto | hunks | comparación completa | sí | sí |
| Untracked | archivo completo | archivo completo | completo | before vacío/after completo |
| Deleted | diff | contenido anterior | sí | after vacío |
| Binary | resumen | no textual | resumen | resumen |
| Conflicto | informativa | working tree legible | limitada | no garantizada |
| Submodule | resumen | resumen | resumen | resumen |

Los archivos tracked que exceden el presupuesto full-file conservan Changes only
si su input y patch están dentro de sus límites independientes; `f` no cambia a
una representación incompleta y la pantalla explica la restricción.

Una terminal angosta fuerza layout compacto o inline y muestra la razón. Binarios,
conflictos, submodules, líneas/archivos excesivos y fallos de highlighting se
degradan explícitamente. No se ofrecen hunks cuando su semántica no es segura.

## Seguridad, privacidad y alcance

Git continúa siendo la fuente de verdad después de cada señal o refresh. Los
paths se pasan como argumentos separados, no mediante un shell, y las consultas
por path usan semántica literal. DiffBeacon nunca elimina `index.lock` ni escribe
el índice, working tree, refs, configuración o metadata Git.

La configuración Git global/system y variables `GIT_*`/`SSH_*` heredadas no
participan. Los overrides desactivan fsmonitor, hooks, external diff, credenciales
y todo protocolo de transporte. La frontera del runner admite solo las formas
exactas de `--version`, `rev-parse`, `status`, `ls-tree`, `ls-files` y `cat-file`
usadas por la aplicación, además del `diff --no-index` acotado sobre snapshots
temporales privados; cualquier otra forma se rechaza antes de iniciar Git.
En partial clones, un blob prometido pero ausente no se descarga: su detalle se
degrada a un error local no fatal y la TUI sigue respondiendo.

Status y watcher poseen presupuestos globales. Cuando se exceden, la TUI conserva
el último snapshot seguro, muestra el error y continúa la reconciliación periódica;
no presenta una lista truncada como completa.

v0.1 no hace stage/unstage, no crea commits, no edita archivos o mensajes, no descarta cambios y no
ejecuta fetch, pull, push, merge, rebase, cherry-pick ni resolución de conflictos.
No atribuye cambios, no revisa pull requests y no envía código, paths, diffs,
métricas ni telemetría.

Consulte [compatibilidad](compatibility.md), [rendimiento y límites](performance.md)
y [arquitectura](architecture.md) para el contrato técnico completo.
