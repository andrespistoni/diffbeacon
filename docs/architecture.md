# Arquitectura de DiffBeacon v0.1

## Flujo principal

```text
cmd/diffbeacon
  ├─ comprueba Git y descubre root/Git dir
  ├─ crea watcher y coordinator
  └─ ejecuta Bubble Tea en alternate screen
          │
          ├─ internal/ui        input, layout y render read-only
          ├─ internal/app       estado, generaciones y contexto
          ├─ internal/watch     fsnotify, debounce y reconciliación
          ├─ internal/git       consultas allowlisted y contenido
          ├─ internal/diff      documentos, hunks, alineación y límites
          └─ internal/highlight Chroma acotado con fallback
```

La UI no ejecuta Git ni contiene intents de escritura. `app.Coordinator` inicia
cada refresh en una goroutine con generación monotónica, cancela el trabajo
reemplazado y entrega resultados que el reducer descarta si son antiguos. Las
únicas interacciones son navegación, selección, filtros, vistas, ayuda, refresh y
salida. Un hunk activo solo identifica una posición visual.

## Frontera Git Read-Only

`internal/git.Runner` es la única frontera de procesos Git de producción. Usa
`exec.CommandContext`, programa y argumentos separados, locale estable,
prompts/pagers inertes, entorno Git aislado y overrides para neutralizar
fsmonitor, hooks, external diff y credenciales. Todas las invocaciones, incluida
`cat-file blob`, reciben `GIT_ALLOW_PROTOCOL` con allowlist vacía,
`GIT_PROTOCOL_FROM_USER=0` y `protocol.allow=never`. El control de entorno
prevalece sobre `protocol.<tipo>.allow=always` en configuración local, por lo que
ningún transporte built-in o helper personalizado queda habilitado.

Antes de crear el proceso, `policy.go` valida la forma completa contra una
allowlist cerrada. v0.1 admite exclusivamente las consultas concretas basadas en:

- `git --version`;
- `git rev-parse` para discovery;
- `git status --porcelain=v2 -z` con opciones fijas;
- `git ls-tree`, `git ls-files` y `git cat-file blob` para contenido;
- `git diff --no-index` sobre snapshots temporales privados para hunks textuales.

No basta con añadir un caller: una forma nueva debe incorporarse explícitamente a
la política y demostrar que es read-only. `add`, `reset`, `rm`, `read-tree`,
`apply`, `checkout`, `restore`, comandos de refs/config y toda forma no enumerada
se rechazan antes de `exec`. Las consultas por path usan `--literal-pathspecs` y
`--` para conservar nombres como `:(glob)*.txt` y `*.txt` sin convertirlos en una
operación. Status conserva su presupuesto de 8 MiB, 100 000 entradas y 2 s.

Contenido Staged compara `HEAD` o árbol vacío con índice; Unstaged compara índice
con working tree; Untracked carga el archivo completo. Para tracked, DiffBeacon
materializa snapshots raw acotados en un directorio temporal privado y pide a
`git diff --no-index --no-ext-diff --no-textconv` un patch Myers. Esto evita
drivers, textconv y filtros `process` configurados por el repositorio. No existe
API de stage/unstage, índice temporal Git, lock propio ni escritura directa bajo
el root o Git dir. Antes de `cat-file`, DiffBeacon comprueba directamente
loose objects, índices de pack y alternates del object store local, incluido
`commondir` en worktrees. Si un `ls-tree`/`ls-files` resuelve un OID promisor
ausente, devuelve el error local sin iniciar otro proceso Git. El coordinator
conserva el snapshot, expone un detalle de error y mantiene activa la TUI, sin
crear objetos, packs ni metadata. La denegación de transportes protege además la
carrera entre comprobación y lectura.

## Observación y consistencia

fsnotify es una señal, no una fuente de verdad. Se observa el árbol de trabajo y,
en Git dir/common dir, solo la raíz administrativa y `refs`; no se recorren
`objects`, logs u otros árboles. No se siguen symlinks. Cada señal debounced pide
un refresh completo; la reconciliación de 3 s recupera eventos perdidos y nuevos
directorios.

Cada recorrido queda limitado a 250 000 entradas, 50 000 watches y 250 ms. Al
agotarse, el error visible declara cobertura parcial y la reconciliación periódica
permanece activa. Stage/unstage o cambios de baseline hechos por otro proceso solo
causan una reconsulta; nunca una mutación compensatoria.

## Contenido y presentación

`internal/diff` parsea el patch de Git y produce un modelo único para inline,
side-by-side, changes-only y full-file. Los hunks no requieren retener los
contenidos completos: si se supera el presupuesto full-file, changes-only sigue
disponible. Inputs, patch y filas completas tienen presupuestos independientes.
`internal/highlight` aplica Chroma por filename con un único slot de trabajo y
timeout; el estilo es opcional y nunca cambia la semántica. Controles ANSI del
contenido se convierten en datos visibles.

El render es función del estado, dimensiones, foco y viewport. Terminales pequeñas
usan un plan compacto determinista. Scope, estado, foco, adición, eliminación y
errores poseen texto o símbolos además de color.

## Privacidad y dependencias

El binario no contiene cliente de red, telemetría ni configuración persistente.
Git es la única dependencia externa de ejecución. Los controles E2E instrumentan
procesos y proxies y comparan snapshots de índice, working tree, refs,
configuración y metadata para todas las interacciones disponibles.
La cobertura promisor separa además snapshots del Git dir completo, objects y
packs, usa un remote instrumentado y fuerza configuración local hostil de
protocolos para probar que no hay bypass.
