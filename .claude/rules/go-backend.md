---
paths:
  - "internal/**/*.go"
  - "cmd/**/*.go"
---

# Go backend

- Layers: `internal/router` registers routes, `internal/handler` binds and validates the request, `internal/application/service` holds business logic, `internal/application/repository` talks to the database, `internal/types` holds the models. Keep logic out of handlers.
- HTTP status comes from the error type. `internal/middleware/error_handler.go` turns an `*apperrors.AppError` into its `HTTPCode` and anything else into a 500. The body is `errors.ErrorResponse` (`{"success": false, "error": {...}}`). A service that returns `fmt.Errorf` for a caller mistake ships a 500 where the API promises a 400 or 404. Return `apperrors.NewBadRequestError`, `NewNotFoundError`, `NewValidationError` and friends (`internal/errors/errors.go`) for client-caused failures.
- Formatting is gofumpt plus gofmt, lines at most 120 (`.golangci.yml`); the pre-commit hook runs `golangci-lint run --new-from-rev=HEAD`.
- Changing a handler's swagger annotations means regenerating `docs/` with `make docs`; `docs/swagger_contract_test.go` checks it.
- New strings are English: log messages, errors, prompts and comments. No CJK text remains (`scripts/check-english-only.sh --summary` is empty).
- Go modules: `github.com/jrmatherly/ai-kb` (root), `.../cli` and `.../client` (their own modules; cli replaces client with `../client`).
