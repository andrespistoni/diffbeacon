# Checklist de release v0.1

Este documento separa evidencia local de cobertura configurada en CI. No declara
una release hasta que los jobs remotos también estén verdes.

## Verificación automatizada

- [x] Local: `make test`.
- [x] Local: `make test-e2e` sin caché.
- [x] Local: `make test-race`.
- [x] Local: `make vet`.
- [x] Local: `govulncheck ./...`.
- [x] Local: `make build`.
- [x] Local: `make build-all` para Linux/macOS/Windows amd64/arm64 e inspección con
  `go version -m`.
- [x] Local: matriz completa con Git 2.31.0 y 2.53.0 en Linux amd64.
- [x] Local: benchmark reproducible con p95 de 66.490 ms al arranque y
  184.206 ms al refresh, ambos bajo 500 ms.
- [ ] CI: job Linux verde en el commit candidato.
- [ ] CI: job macOS verde en el commit candidato.
- [ ] CI: job Git mínimo 2.31.0 verde en el commit candidato.
- [ ] CI: seis cross-builds y seis paquetes distribuibles descargados e inspeccionados.
- [ ] CI: reporte JSON de performance revisado; cualquier incumplimiento se
  registra como riesgo, aunque el job no falle por el umbral.
- [ ] CI: `SHA256SUMS`, ambos SBOM y procedencia/attestación descargados y
  verificados contra el commit candidato.

## Cobertura de aceptación

| Criterios | Evidencia principal |
|---|---|
| AC-1–AC-7 | integración real de discovery, status y contenido en `internal/git` |
| AC-8–AC-12 | unitarias/goldens de diff, highlighting y render |
| AC-13–AC-16 | integración viva de watcher, coordinator, UI y generaciones |
| AC-17–AC-23 | allowlist central, snapshots read-only y observación de cambios externos |
| AC-24–AC-30 | E2E de lock, paths, procesos, privacidad y terminal |
| AC-31 | builds sin cgo, un binario por plataforma, inspección de metadata |
| AC-32 | check de arranque y matriz Git 2.31.0/actual |
| AC-33 | `test/performance` y reporte reproducible p95 |

## Alcance y seguridad

- [x] Git es la única dependencia externa declarada de ejecución.
- [x] No se requiere red, cuenta, servicio remoto, telemetría ni configuración.
- [x] No existe creación de commits ni edición de mensajes.
- [x] No existen fetch, pull, push, merge, rebase o cherry-pick.
- [x] No existe descarte, checkout de paths, borrado de archivos/locks ni editor.
- [x] No existe resolución de conflictos ni gestión interna de submodules.
- [x] No existe stage/unstage de archivo, hunk o conjunto, ni keybindings `s/u/S/U`.
- [x] Hunks son exclusivamente navegación y no generan patches aplicables.
- [x] La allowlist rechaza antes de `exec` todo comando u opción no aprobada.
- [x] Fsmonitor/helpers están neutralizados y los filtros no se ejecutan durante consultas.
- [x] E2E compara índice, working tree, refs, configuración y metadata tras todas las interacciones.
- [x] Partial clone real cubre blobs ausentes de tree/index, remote instrumentado,
  bypass de protocolo local, error no fatal y snapshots separados de Git dir,
  objects y packs sin transporte ni mutación.
- [x] Repositorio sin `HEAD` observa stage/unstage externo de archivo, hunk y
  conjunto sin mutación compensatoria.
- [x] `:(glob)*.txt` y `*.txt` están cubiertos como paths read-only; F-VER-01 no conserva superficie mutante.
- [x] Diagnósticos pre-TUI neutralizan C0/DEL/C1/ESC/CSI/OSC.
- [x] Status, recorridos y watches tienen presupuestos y degradación explícita.
- [x] Acciones fijadas por SHA; tarball mínimo autenticado; checksums/SBOM/
  procedencia configurados.

## Revisión manual pendiente de candidata

- [ ] Probar una terminal clara y una oscura en Linux y macOS.
- [ ] Probar resize continuo, terminal compacta y salida por interrupción real.
- [ ] Confirmar checksums/tamaños de artefactos publicados.
- [ ] Revisar resultados CI contra [compatibility.md](compatibility.md) y
  [performance.md](performance.md), sin copiar versiones flotantes como evidencia
  permanente.

Limitación real: esta sesión no dispuso de host macOS ni ejecutó GitHub Actions.
La compilación cruzada de Darwin fue exitosa, pero no equivale a ejecución nativa.
