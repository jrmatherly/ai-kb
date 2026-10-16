---
paths:
  - "internal/**/*.go"
  - "cmd/**/*.go"
---

# Go backend

- Layers: `internal/router` registers routes, `internal/handler` binds and validates the request, `internal/application/service` holds business logic, `internal/application/repository` talks to the database, `internal/types` holds the models. Keep logic out of handlers.
- HTTP status comes from the error type. `internal/middleware/error_handler.go` turns an `*apperrors.AppError` into its `HTTPCode` and anything else into a 500. The body is `errors.ErrorResponse` (`{"success": false, "error": {...}}`). Every 5xx is scrubbed to a fixed message plus `request_id` (the cause is logged), so a service that returns `fmt.Errorf` for a caller mistake ships an opaque 500 where the API promises a 400 or 404. Return `apperrors.NewBadRequestError`, `NewNotFoundError`, `NewValidationError` and friends (`internal/errors/errors.go`) for client-caused failures, and `NewBadGatewayError` (502, code 1011, cause kept) when an upstream the user configured fails. `AppError` has no `Unwrap`: convert sentinels to AppErrors at the service's exported edge, not deeper.
- Connector errors (`internal/datasource/connector/*`) are classified by sentinels, never by message text: `connectorError` in `internal/application/service/datasource_service.go` maps `datasource.ErrPermissionDenied`, `ErrInvalidCredentials`, `ErrResourceNotFound` and `ErrInvalidConfig` to `details.code` and to 400 (other failures 502). A connector with a typed API error gives it an `Is(target)` that maps its own status and codes to those sentinels; Feishu's table is `core.APIError.Is` in `feishu/core/client.go`. The service imports no connector package. `APIError.Error()` text is kept because `feishuFailure` (`feishu/core/shared.go`) still reads it for sync item reasons.
- Formatting is gofumpt plus gofmt, lines at most 120 (`.golangci.yml`); the pre-commit hook runs `golangci-lint run --new-from-rev=HEAD`.
- Changing a handler's swagger annotations means regenerating `docs/` with `make docs`; `docs/swagger_contract_test.go` checks it.
- New strings are English: log messages, errors, prompts and comments. No CJK text remains (`scripts/check-english-only.sh --summary` is empty).
- Go modules: `github.com/jrmatherly/ai-kb` (root), `.../cli` and `.../client` (their own modules; cli replaces client with `../client`).
