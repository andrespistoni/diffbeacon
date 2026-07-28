# Repository instructions

## Product invariants

No cambies estos contratos sin una petición explícita y una entrada destacada
en el changelog:

- DiffBeacon es estrictamente read-only: no agregues stage, unstage, commit,
  checkout, discard, fetch, pull, push ni ninguna mutación del repositorio.
- Todo comando Git debe pasar por la allowlist central antes de ejecutarse.
- No habilites filtros, hooks, pagers, helpers, editores ni transporte remoto
  durante las consultas.
- No registres contenido del repositorio, paths privados, variables de entorno,
  argumentos sensibles ni output sin sanitizar.
- Los límites de status, diff, archivos y render deben degradar de forma segura
  y visible, nunca cargar contenido sin presupuesto.
- Linux, macOS y Windows siguen siendo plataformas soportadas. Un cross-build
  demuestra compilación, no funcionamiento nativo.

## Code navigation

Si existe `.codegraph/`, usa CodeGraph antes de búsquedas textuales o lectura
manual para localizar símbolos, comprender flujos y evaluar el impacto de un
cambio. No crees el índice automáticamente cuando no exista.

## Conventional Commits

Los commits deben seguir:

```text
<type>[optional scope][!]: <descripción>
```

Usa `feat`, `fix`, `docs`, `test`, `refactor`, `perf`, `build`, `ci`, `chore`
o `revert`. La descripción debe ser breve, imperativa, en minúscula y sin punto
final. Marca incompatibilidades con `!` y un footer `BREAKING CHANGE:`.

## Validation

Ejecuta validación proporcional y comunica qué se ejecutó:

- Todo cambio: `git diff --check` y `test -z "$(gofmt -l .)"`.
- Código Go: `go test ./... -count=1` y `go vet ./...`.
- Concurrencia, procesos, filesystem o watcher:
  `go test -race ./... -count=1`.
- Scripts shell: `bash -n scripts/*.sh`.
- Packaging o release: `make dist VERSION=X.Y.Z`.
- Candidata completa: `make release-check VERSION=X.Y.Z`.

No declares verificados Windows o macOS sin evidencia ejecutada en esos hosts.

## Changelog

Mantén `CHANGELOG.md` según Keep a Changelog y SemVer.

- Añade en `Unreleased` todo cambio visible para usuarios.
- Usa sólo `Added`, `Changed`, `Deprecated`, `Removed`, `Fixed` y `Security`.
- No inventes versiones, fechas ni resultados.
- Al publicar, mueve las entradas a `[X.Y.Z] - YYYY-MM-DD`, conserva
  `Unreleased` y actualiza los enlaces.

## Release discipline

- No crees ni publiques tags, releases o assets salvo solicitud explícita.
- El tag debe ser `vX.Y.Z` y existir en `CHANGELOG.md`; el tag es la única
  fuente de verdad para la versión publicada.
- Las Actions deben tener permisos mínimos y referencias fijadas a SHA.
- Nunca publiques desde un árbol sucio ni ignores una comprobación fallida.
