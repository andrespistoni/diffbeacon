# Rendimiento, límites y temporizadores

## Objetivo y método

El objetivo de v0.1 es p95 menor de 500 ms para primera vista útil y actualización
normal en el fixture documentado. Es una meta de release, no una garantía para
monorepos, filesystems remotos, máquinas saturadas o Git configurado con trabajo
adicional.

`test/performance/refresh_test.go` usa el binario real y un repositorio temporal
determinista:

- 200 archivos trackeados de 40 líneas;
- 376 000 bytes de contenido de working tree al crear el fixture;
- 25 archivos modificados, con `file-000.txt` seleccionado;
- 25 muestras de arranque y 25 de actualización por defecto;
- p95 por **nearest rank**, índice `ceil(0.95*n)` en la serie ordenada;
- primera vista: desde `exec` del proceso hasta observar en stdout el marcador
  único renderizado del archivo seleccionado;
- actualización: desde que termina `os.WriteFile` hasta observar el nuevo
  marcador único renderizado por el proceso ya activo.

La actualización incluye fsnotify, debounce, consulta Git, carga, diff, fallback
de highlighting y render TUI. El build del binario y la creación del fixture
quedan fuera de las muestras. Cada espera posee timeout de 8 s para detectar un
flujo roto, pero el test no falla por superar 500 ms: registra `RELEASE_RISK`.
Esto evita un gate flaky por carga del runner. Para una evaluación controlada se
puede activar `DIFFBEACON_PERF_ENFORCE=1`.

El procedimiento no ejecuta staging desde DiffBeacon. La preparación inicial del
fixture usa un helper externo antes de medir; durante las muestras el binario solo
realiza consultas aprobadas por la política read-only.

## Reproducción

```sh
./scripts/benchmark-refresh.sh
# o, con al menos 20 muestras y salida elegida:
DIFFBEACON_PERF_SAMPLES=50 \
DIFFBEACON_PERF_REPORT=/tmp/diffbeacon-performance.json \
./scripts/benchmark-refresh.sh
```

El script registra sistema, Go, Git, filesystem y número de muestras. El JSON
por defecto queda en `build/performance/report.json`, un artefacto ignorado. CI
ejecuta el mismo script y sube ese JSON; un reporte de CI es evidencia de ese
runner concreto, no de todos los entornos.

## Resultado local medido

Ejecución del 2026-07-26 en WSL2/Linux amd64, filesystem informado como
`ext2/ext3`, 8 CPU lógicas de un Intel Core i7-1185G7, Go 1.26.5 y Git 2.53.0:

| Métrica (25 muestras) | p50 | p95 | máximo | objetivo p95 < 500 ms |
|---|---:|---:|---:|---|
| Arranque a primera vista útil | 52.362 ms | 66.490 ms | 72.592 ms | Cumplido |
| Fin del guardado a vista actualizada | 183.046 ms | 184.206 ms | 184.282 ms | Cumplido |

No se extrapola este resultado a macOS ni a los runners de CI. No existe un
riesgo de latencia observado en esta medición; un futuro reporte que exceda el
objetivo debe registrarse como riesgo antes de release aunque el test termine
correctamente.

## Temporizadores efectivos

| Control | Valor | Comportamiento |
|---|---:|---|
| Debounce de eventos | 150 ms | reinicia tras cada evento de la ráfaga |
| Espera máxima de ráfaga | 500 ms | fuerza señal aun con eventos continuos |
| Reconciliación completa | 3 s | recupera eventos perdidos y resincroniza watches |
| Highlighting por documento | 250 ms | cancela/fallback a texto plano |
| Status Git | 2 s | cancela la consulta y conserva el snapshot anterior |
| Patch Git | 5 s | cancela sólo el detalle seleccionado |
| Recorrido de watches | 250 ms | detiene el recorrido y queda reconciliación periódica |

El watcher normaliza valores fuera de los rangos aprobados: debounce 100–250 ms
y reconciliación 2–5 s. Los valores no son configurables por CLI en v0.1.

## Límites efectivos

| Recurso | Límite |
|---|---:|
| Contenido por lado retenido para full-file | 1 MiB |
| Longitud de línea en full-file | 16 KiB |
| Líneas combinadas de full-file | 20 000 |
| Input por lado para changes-only | 64 MiB |
| Patch de changes-only | 8 MiB |
| Longitud de línea del patch | 256 KiB |
| Líneas del patch | 100 000 |
| Contexto por hunk | 3 líneas |
| Entrada combinada al highlighting | 256 KiB |
| Tokens de highlighting | 50 000 |
| stderr retenido por comando Git | 8 KiB |
| stdout de status porcelain v2 | 8 MiB |
| Entradas/cambios de status | 100 000 |
| Entradas por recorrido del watcher | 250 000 |
| Directorios observados | 50 000 |

Superar el presupuesto full-file fuerza changes-only con una explicación, pero
no descarta hunks disponibles. Superar los límites de input o patch sí produce
una vista degradada con causa estable, nunca un recorte presentado como completo.
El highlighting vuelve a texto plano por
tamaño, tokens, timeout, cancelación, lexer ausente o error. stdout de lecturas
de contenido Git también se acota antes de convertirlo en documento; stderr se
sanitiza y marca como truncado. Status nunca parsea un prefijo truncado: devuelve
`ErrStatusBudget`, conserva el snapshot visible anterior y espera el siguiente
refresh/reconcile. El watcher devuelve `ErrWatchBudget`, mantiene los watches ya
instalados y continúa reconciliando cada 3 s.
