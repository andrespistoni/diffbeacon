# Política de dependencias y cadena de suministro

## Actualizaciones

- Dependabot revisa mensualmente módulos Go y GitHub Actions. Cada cambio se
  integra únicamente tras `go mod verify`, tests, race, vet, E2E,
  `govulncheck`, release metadata y builds de plataforma.
- Vulnerabilidades alcanzables bloquean la release. Una vulnerabilidad no
  alcanzable se documenta con plataforma, versión corregida y fecha objetivo;
  no se acepta automáticamente por estar fuera de la matriz actual.
- Las acciones se fijan por SHA completa obtenida del repositorio oficial. El
  comentario conserva la versión revisada para facilitar auditoría y updates.
- Las licencias nuevas se revisan en el SBOM SPDX/Syft y contra las fuentes del
  módulo antes de integrar; `NOASSERTION` exige revisión humana, no equivale a
  aprobación.

## Evidencia de release

`make release-metadata` genera checksums y metadata para los seis binarios raw en
`build/release/`. `make dist` copia `diffbeacon.spdx.json` (SPDX 2.3) y
`provenance.intoto.json` (in-toto/SLSA) junto a los paquetes finales y crea un
`dist/SHA256SUMS` nuevo sobre los assets distribuibles. CI añade un SBOM independiente con
Syft y, fuera de pull requests, una attestación de procedencia firmada por
GitHub OIDC. Los metadatos locales no firmados son evidencia inspeccionable; no
sustituyen la attestación.

La fuente Git 2.31.0 usada en la matriz se acepta solo si coinciden el SHA-256
publicado y la firma OpenPGP del tar sin comprimir con el subkey
`E1F036B1FEE7221FC778ECEFB0B5E88696AFE6CB`. El piso de compatibilidad no es una
recomendación de ejecutar una versión sin parches de seguridad.
