# Changelog

Todos los cambios relevantes de DiffBeacon se documentan en este archivo.

El formato está basado en [Keep a Changelog](https://keepachangelog.com/es-ES/1.1.0/)
y el proyecto utiliza [Versionado Semántico](https://semver.org/lang/es/).

## [Unreleased]

## [0.1.1] - 2026-07-28

### Added

- Instalación y desinstalación directas mediante `curl | sh` en Linux/macOS o
  `irm | iex` en PowerShell; la instalación verifica SHA-256.
- TUI read-only para revisar cambios staged, tracked y untracked con vistas
  inline, side-by-side, changes-only y full-file.
- Actualización automática ante cambios externos del working tree, índice y
  refs, con límites explícitos para repositorios y archivos grandes.
- Paquetes para Linux, macOS y Windows en amd64 y arm64, instaladores,
  checksums, SBOM y procedencia.
- CI multiplataforma, matriz con Git 2.31.0, race detector, análisis de
  vulnerabilidades y evidencia de release.

### Security

- Allowlist central de comandos Git estrictamente read-only, con helpers,
  filtros, pagers, editores y transporte remoto neutralizados.
- Sanitización de diagnósticos y cobertura E2E de privacidad, paths y
  preservación del repositorio.

[Unreleased]: https://github.com/andrespistoni/diffbeacon/compare/v0.1.1...HEAD
[0.1.1]: https://github.com/andrespistoni/diffbeacon/releases/tag/v0.1.1
