# The docs moved to website-docs

Product, deployment, API and development docs live in [website-docs](../website-docs/README.md). The old hand-written docs that were migrated, duplicated or out of date are deleted; their content is in the Git history.

- [FAQ and upgrade troubleshooting](../website-docs/01-getting-started/05-troubleshooting.md)
- [API overview](../website-docs/04-api/01-api-overview.md)
- [Development guide](../website-docs/06-development/01-dev-guide.md)
- [Migration map and remaining dependencies](../website-docs/MIGRATION.md)

This directory keeps only these engineering resources and this page:

- `docs.go`, `swagger.json`, `swagger.yaml` and the contract test: part of the backend build and tests. The generated files are still updated by `make docs`.
- `LITE.md`: copied into the Lite release package as its offline README.
- `images/`: the README hero and architecture art (`images/readme/`), the brand sources (`images/brand/`) and `images/logo.png`, which `scripts/brand/generate.mjs` writes.
- `poc/docker-sandbox/`: a standalone Go experiment module. Its source and run notes stay; it is not a current product guide.

These resources are still used by the build, the release or past experiments, so the directory cannot simply be deleted. Do not add new product docs here.
