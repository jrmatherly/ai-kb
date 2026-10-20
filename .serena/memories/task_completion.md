# Done criteria for a code change

Run `scripts/validate.sh` (or `--base <rev>`): it runs the areas below that the change touches, then the English-only, em dash and version gates. CI in `.github/workflows/` runs the same commands. Per area:

1. Root Go: `go vet` + `go test` over `go list ./... | grep -v /docreader/`; `go test -tags desktop ./internal/container` if `internal/container` changed; `golangci-lint run` clean; `go build ./cmd/server`; `scripts/check-govulncheck.sh` (symbol-level findings against `scripts/govulncheck-allowlist.txt`, needs govulncheck and jq) and `bash scripts/test-check-govulncheck.sh`.
2. Model metadata (`internal/models/catalog/data/**`, `scripts/model-catalog/**`): `make model-catalog-check` exits 0, else `make model-catalog-generate` and commit the output.
3. `cli/**`: `go -C cli test -race ./... && go -C cli vet ./...` + the two `cli/scripts/check-*.sh`; validate also cross-compiles darwin, linux and windows on amd64 and arm64 with `CGO_ENABLED=0`. `client/**`: `go -C client test ./...`. `scripts/update_tap.py`, its test or `release-lite.yml`: `scripts/validate.sh --area tap`. `third_party/anydoc-go/**`, `scripts/build-anydoc-lib.sh` or `internal/infrastructure/docparser/**`: `--area anydoc` (builds the patched crate with its zip 9.0.5 pin and runs `cargo audit`).
4. `frontend/**`: `scripts/verify_frontend_pr.sh` (test, type-check, build, then fails if `src/components.d.ts` changed). A template that adds a TDesign component needs the build and the regenerated `components.d.ts` committed. i18n key change: `check-i18n` passes, and a `datasource.syncError.*` change also needs `go test ./internal/datasource/syncreason/` (validate runs the Go area for `en-US.ts`).
5. `dataset/**`: `uv run --project dataset --locked python -m unittest discover -s dataset/tests` (`scripts/validate.sh --area dataset`).
6. `docreader/**`: unittest discover under `uv run --project docreader`; `go test ./docreader/client ./docreader/proto` if proto or client changed.
7. Swagger annotations changed: `make docs`, commit `docs/docs.go` + `swagger.{json,yaml}`.
8. `scripts/check-english-only.sh <changed files>` and `scripts/check-emdash.sh` exit 0; changed Markdown passes markdownlint.
9. Commit locally only when asked, conventional-commit subject; never push.
