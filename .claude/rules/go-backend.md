---
paths:
  - "internal/**/*.go"
  - "cmd/**/*.go"
---

# Go backend

- Layers: `internal/router` registers routes, `internal/handler` binds and validates the request, `internal/application/service` holds business logic, `internal/application/repository` talks to the database, `internal/types` holds the models. Keep logic out of handlers.
- HTTP status comes from the error type. `internal/middleware/error_handler.go` turns an `*apperrors.AppError` into its `HTTPCode` and anything else into a 500. A service that returns `fmt.Errorf` for a caller mistake ships a 500 where the API promises a 400 or 404; two such bugs are logged (wiki page slug, chunk edit). Return `apperrors.NewBadRequestError`, `NewNotFoundError`, `NewValidationError` and friends (`internal/errors/errors.go`) for client-caused failures.
- Formatting is gofumpt plus gofmt, lines at most 120 (`.golangci.yml`); the pre-commit hook runs `golangci-lint run --new-from-rev=HEAD`.
- Changing a handler's swagger annotations means regenerating `docs/` with `make docs`; `docs/swagger_contract_test.go` checks it.
- New strings are English: log messages, errors, prompts and comments. The seed's Chinese strings are translated in a separate pass, not opportunistically inside an unrelated fix.
- The Go module path is still `github.com/Tencent/WeKnora` until the code rebrand lands; new imports use it.
