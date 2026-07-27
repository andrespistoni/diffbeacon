# Checklist para publicar DiffBeacon como open source

El repositorio está técnicamente bastante completo, pero antes de publicarlo en
GitHub conviene completar y revisar los siguientes puntos.

## Imprescindible

### 1. Agregar `CHANGELOG.md` en la raíz

El archivo `.btpaitool/docs/changelog.md` está ignorado y además es documentación
interna autogenerada. Conviene crear un `CHANGELOG.md` público, por ejemplo usando
el formato [Keep a Changelog](https://keepachangelog.com/), con una sección inicial
para la versión `0.1.1`.

### 2. Corregir el módulo Go

Actualmente `go.mod` declara:

```go
module diffbeacon
```

Para que el proyecto pueda instalarse mediante `go install`, debería utilizar la
URL definitiva del repositorio:

```go
module github.com/USUARIO/diffbeacon
```

También habrá que actualizar los imports `diffbeacon/internal/...`. Esto debe
hacerse después de decidir el usuario u organización y la URL definitiva del
repositorio.

### 3. Versionar los archivos del proyecto

Actualmente prácticamente todo el proyecto está sin seguimiento en Git:
`README.md`, `LICENSE`, `go.mod`, código fuente, tests, documentación y archivos de
`.github/`. Git solamente tiene registrados `.gitignore` y la especificación
inicial.

Antes de publicar hay que revisar y agregar todos los archivos que deban formar
parte del repositorio, evitando incluir `bin/`, `build/`, `dist/` y archivos de
herramientas locales.

### 4. Agregar `SECURITY.md`

Es especialmente importante porque DiffBeacon hace afirmaciones fuertes de
seguridad y privacidad. El documento debería indicar:

- Qué versiones reciben correcciones de seguridad.
- Cómo reportar una vulnerabilidad de manera privada.
- Qué información incluir en el reporte.
- Que las vulnerabilidades no deben publicarse inicialmente como un issue normal.

### 5. Agregar `CONTRIBUTING.md`

Debería explicar como mínimo:

- Requisitos de desarrollo.
- Cómo compilar el proyecto.
- Cómo ejecutar `make test`, `make vet` y las demás verificaciones.
- Convenciones básicas para los cambios.
- Proceso esperado para issues y pull requests.

## Muy recomendable

- Agregar `CODE_OF_CONDUCT.md`.
- Agregar `.github/PULL_REQUEST_TEMPLATE.md`.
- Agregar formularios de issues para bugs y propuestas de funcionalidades.
- Incluir una captura o GIF de la TUI en `README.md`.
- Agregar badges de CI, licencia y última release.
- Incluir una instrucción corta de instalación desde GitHub Releases.
- Decidir si la comunidad objetivo será hispanohablante o internacional. Para un
  alcance global convendría un README en inglés o bilingüe.
- Configurar en GitHub la descripción, topics, protección de la rama principal,
  secret scanning y private vulnerability reporting.
- Evaluar GitHub Discussions si se desea separar preguntas de los reportes de bugs.

## Elementos que ya están bien resueltos

- Licencia MIT válida en `LICENSE`.
- `README.md` completo.
- `.gitignore` para binarios, builds y herramientas locales.
- CI para Linux, macOS y Windows.
- Dependabot para módulos Go y GitHub Actions.
- Tests, race detector, `govulncheck`, SBOM, checksums y procedencia.
- Documentación de arquitectura, compatibilidad, distribución y releases.
- No se encontraron credenciales evidentes en los archivos revisados.

## Detalle sobre `.github/`

El `.gitignore` bloquea actualmente cualquier archivo nuevo dentro de `.github/`,
excepto `.github/workflows/ci.yml` y `.github/dependabot.yml`. Si se agregan
plantillas de issues o pull requests dentro de `.github/`, habrá que modificar esas
reglas para permitir que Git las registre.

## Archivos que no son necesarios inicialmente

Para una primera versión distribuida bajo MIT no es obligatorio agregar:

- `NOTICE`
- `AUTHORS.md`
- `SUPPORT.md`
- `GOVERNANCE.md`

Pueden incorporarse más adelante si el proyecto o su comunidad lo requieren.

## Mínimo recomendado antes de publicar

- [ ] Crear `CHANGELOG.md`.
- [ ] Crear `SECURITY.md`.
- [ ] Crear `CONTRIBUTING.md`.
- [ ] Definir la URL definitiva de GitHub.
- [ ] Actualizar el módulo Go y sus imports.
- [ ] Agregar una captura o GIF del programa.
- [ ] Revisar todos los archivos sin seguimiento.
- [ ] Confirmar que no se incluirán secretos ni artefactos locales.
- [ ] Ejecutar `make release-check`.
- [ ] Crear el primer commit completo.
- [ ] Configurar las opciones de seguridad y protección del repositorio en GitHub.
