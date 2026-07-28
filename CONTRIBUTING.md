# Contribuir a DiffBeacon

Gracias por contribuir. Antes de empezar, revisá `AGENTS.md` y mantené el
contrato estrictamente read-only del producto.

## Preparar el entorno

Necesitás Go según `go.mod`, Git 2.31.0 o posterior, `make`, `tar` y `zip`.

```sh
git clone https://github.com/andrespistoni/diffbeacon.git
cd diffbeacon
go mod download
make build
```

## Verificar cambios

Para cambios normales:

```sh
make test
make vet
make test-e2e
```

Para cambios de concurrencia, filesystem, watcher o proceso Git:

```sh
make test-race
```

Antes de proponer una release:

```sh
make release-check VERSION=0.1.1
```

## Pull requests

- Abrí un issue primero para cambios grandes o incompatibles.
- Mantené cada PR enfocado e incluí tests cuando cambie comportamiento.
- Actualizá documentación y `CHANGELOG.md` si el cambio es visible.
- Usá Conventional Commits según `AGENTS.md`.
- No incluyas secretos, repositorios de terceros ni artefactos de `bin/`,
  `build/` o `dist/`.

Al enviar un PR aceptás que tu contribución se distribuya bajo la licencia MIT
del proyecto.
