# docreader/ (Python gRPC document parser)

- `uv`-managed project (`pyproject.toml`, `uv.lock`), Python >= 3.14. Entry `python -m docreader.main` (gRPC server on 50051, health checking enabled). `container_entrypoint.py` for the image, `Dockerfile` beside it.
- Layout: `parser/` (format parsers: docx, epub, excel, html, pdf via markitdown/pypdf/pypdfium2/opendataloader, playwright for browser fixtures), `splitter/`, `models/`, `proto/` + `client/` (Go packages in the root module: `go test ./docreader/client ./docreader/proto`), `utils/`, `tests/` (`unittest`, fixtures in `tests/fixtures/`), `scripts/generate_proto.sh` via `make -C docreader proto`.
- Root Go CI excludes `/docreader/` packages from the general `go test` list and tests them in the docreader workflow after starting the Python server.
- Tests: `uv run --project docreader python -m unittest discover -s docreader/tests -p "test_*.py" -v`; some need `python -m playwright install --with-deps webkit`.
