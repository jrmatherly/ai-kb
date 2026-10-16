# Done criteria for a code change

Run `scripts/validate.sh` (or `--base <rev>`): it runs the areas below that the change touches, then the English-only, em dash and version gates. CI in `.github/workflows/` runs the same commands. Per area:

1. Root Go: `go vet` + `go test` over `go list ./... | grep -v /docreader/`; `go test -tags desktop ./internal/container` if `internal/container` changed; `golangci-lint run` clean; `go build ./cmd/server`.
2. Model metadata (`internal/models/catalog/data/**`, `scripts/model-catalog/**`): `make model-catalog-check` exits 0, else `make model-catalog-generate` and commit the output.
3. `cli/**`: `go -C cli test -race ./... && go -C cli vet ./...` + the two `cli/scripts/check-*.sh`. `client/**`: `go -C client test ./...`.
4. `frontend/**`: `npm --prefix frontend test && npm --prefix frontend run type-check` (build only when bundling is in doubt). i18n key change: `check-i18n` passes, and a `datasource.syncError.*` change also needs `go test ./internal/datasource/syncreason/` (validate runs the Go area for `en-US.ts`).
5. `docreader/**`: unittest discover under `uv run --project docreader`; `go test ./docreader/client ./docreader/proto` if proto or client changed.
6. Swagger annotations changed: `make docs`, commit `docs/docs.go` + `swagger.{json,yaml}`.
7. `scripts/check-english-only.sh <changed files>` and `scripts/check-emdash.sh` exit 0; changed Markdown passes markdownlint.
8. Commit locally only when asked, conventional-commit subject; never push.
