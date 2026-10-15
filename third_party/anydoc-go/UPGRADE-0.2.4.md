# AnyDoc 0.2.4 upgrade review and load test

Review date: 2026-09-29. Project baseline: `9114e4e4f905be71976a77c684d3f92731e6f9a6`.

## Conclusion

We follow upstream from 0.1.9 to 0.2.4 and keep maintaining the vendored Go binding. The upgrade is mainly about document completeness: formulas, checkboxes and the scanned pages of mixed PDFs are no longer dropped silently.

- [Go binding PR #30](https://github.com/firecrawl/anydoc/pull/30) is still open, `merged=false`, `mergeable=false`, head `1a7a6c04ff8f2689bf8ebbd1e0d9d61d2d109164`, last updated 2026-08-08. There is no `go/v*` release tag.
- The latest release tag is [v0.2.4](https://github.com/firecrawl/anydoc/tree/v0.2.4), released 2026-08-27, commit `42bf1c5ecdde9eb0d96d6bd75a9e6698cf93b14c`. The three commits on main after that tag change only the README.
- The downloaded crate source was checked file by file against the v0.2.4 tag. Only the existing renderer re-export patch is kept.
- We reviewed the [full v0.1.9...v0.2.4 source diff](https://github.com/firecrawl/anydoc/compare/v0.1.9...v0.2.4), including the model, serialization, the PDF path, the Excel parser and the bindings, not just the release notes.

## Core changes and how this project handles them

| Upstream change | Impact and handling |
| --- | --- |
| 0.2.0: calamine removed, XLS/XLSX/XLSB parsed in-house; better number display, merged cells and hidden content handling | The existing format routing stays; every container is regression tested. The upstream XLSB sample fails on the old version and succeeds on the new one; this does not mean the application gained an XLSB upload path. |
| 0.2.0: fixes for DOCX run edge spaces, duplicate EPUB spine entries, Markdown separators and pipes in table code | The 66 upstream snapshots are compared for Markdown and error details through the real Go ABI. |
| 0.2.1-0.2.2: OMML/MathML/RTF formulas converted to LaTeX, dollar-sign escaping fixed | New block/inline math ABI tags, encoding and decoding; the image path accepts formula nodes. DOCX/PPTX/ODT/EPUB/RTF regression tested. |
| 0.2.3: checkboxes become inline nodes, Rust `ListItem.checked` removed | New checkbox tag and bool field. The old wire slot stays as nil; the old Go field is marked deprecated. Tested with titled checked and unchecked controls. |
| 0.2.4: mixed or scanned PDFs return `NeedsOcr`, and the error names the pages | New dedicated error code and `NeedsOCR()`, recognizing wrapped errors through `errors.As`; real Reader tests verify the fallback and the failure without OCR. The page numbers stay in Detail; no structured page-number API was added. |
| Node/Python/CLI can choose Firecrawl hosted OCR | Rust does not include it; this project keeps its existing builtin/docreader fallback. |

Local thread binding, panic capture, the decode allocation cap and the image link API are all kept.
This upgrade also removes the duplicate parse on the image extraction path: the new `ToDocumentWithAssetLinks` parses once and returns both the raw document tree and the image Markdown, reusing the single-buffer allocate/free protocol. It encodes the raw asset IDs first and then rewrites the image links, so image positions and section information are not lost. The upstream corpus and the 13 built-in non-PDF samples all confirm that the merged interface gives the same output as the two separate interfaces. The local version cannot be deleted as soon as the PR merges: we also need to confirm that upstream has these capabilities. See the [README](README.md) for the detailed differences.

The dependency lock keeps `pdf-inspector=1.14.2` and `lopdf=0.42.0` and removes calamine and its five dedicated dependencies. The audit found that the existing `chacha20=0.10.1` was yanked; it is updated to 0.10.2 per [upstream fix #580](https://github.com/RustCrypto/stream-ciphers/pull/580), which fixes the SSE2 backend wrongly using SSE4.1 instructions. No other transitive dependency versions changed.

## Verification done

- `scripts/build-anydoc-lib.sh`: the release archive builds with `--locked`; the C header is regenerated in sync.
- `ANYDOC_UPSTREAM_DIR=... go test -tags anydoc -count=1 ./internal/infrastructure/docparser/...`: passes, including the 66 upstream snapshots, Go model decoding of normal documents and the image Markdown path. Empty input keeps the Go binding's existing up-front error, asserted separately.
- `go test -race -tags anydoc -count=1 ./internal/infrastructure/docparser/...`: passes. The Go race detector does not cover Rust memory access.
- `go test -count=1 ./internal/infrastructure/docparser/...`: the path without the engine linked passes.
- `go vet -tags anydoc ./internal/infrastructure/docparser/...`: passes.
- `go build -tags anydoc -o /tmp/ai-kb-anydoc-server ./cmd/server`: the full server compiles.
- The existing regressions (4,000 concurrent error details, deeply nested malicious PDFs, TJ lookback): pass.
- `cargo audit --file third_party/anydoc-go/Cargo.lock`: no known vulnerabilities; the unmaintained warning `RUSTSEC-2026-0192` for `ttf-parser 0.25.1` remains.

## How performance was measured

Apple M4 Pro, 64 GiB RAM, macOS arm64; Go 1.26.3, Rust 1.97.1, Rust release + thin LTO.
The same Go benchmark and the same fixed upstream samples were linked against 0.1.9 and 0.2.4 in turn, excluding file reads and compile time.
`WithAssets=true` covers the production Reader's default path: Rust parsing, Go document decoding, image collection and the official Markdown output. The old and the straight-upgrade versions parse twice; the final version parses once. PDF still takes the text path.
Each item runs 500 ms × 3 times, `GOMAXPROCS=1,4`; the tables give the median of the three. The four-way ns/op is total time divided by total runs, a throughput measure, not single-request latency. `B/op` counts only the Go heap and says nothing about Rust memory.
Measured sequentially after compiling and the other tests finished. This is a small-sample comparison on one machine; it claims no statistical significance and is not equivalent to production end-to-end performance.

### Same machine: single concurrency

Unit: µs/op, lower is better; change is relative to 0.1.9.

| Format | 0.1.9 | 0.2.4 straight upgrade | 0.2.4 final | Final change |
| --- | ---: | ---: | ---: | ---: |
| doc | 228.7 | 237.7 | 147.2 | -35.6% |
| docx | 1040.3 | 1082.4 | 559.8 | -46.2% |
| ppt | 60.0 | 62.7 | 37.0 | -38.4% |
| pptx | 887.2 | 895.4 | 450.3 | -49.2% |
| odt | 719.5 | 727.5 | 390.7 | -45.7% |
| ods | 478.2 | 483.2 | 259.3 | -45.8% |
| odp | 1157.2 | 1167.6 | 588.2 | -49.2% |
| rtf | 742.8 | 757.4 | 407.2 | -45.2% |
| epub | 313.4 | 314.5 | 174.9 | -44.2% |
| xls | 69.8 | 127.9 | 76.1 | +9.0% |
| xlsx | 217.7 | 500.3 | 261.2 | +20.0% |
| xlsb | conversion failed | 84.3 | 48.7 | newly working sample |
| csv | 155.9 | 161.2 | 90.1 | -42.2% |
| pdf | 1941.6 | 3605.4 | 3631.9 | +87.1% |

### Four-way concurrent throughput

Unit: µs/op (wall-clock time per conversion, not request latency).

| Format | 0.1.9 | Final | Change |
| --- | ---: | ---: | ---: |
| doc | 53.8 | 36.6 | -32.0% |
| docx | 268.3 | 142.5 | -46.9% |
| ppt | 15.9 | 9.8 | -38.6% |
| pptx | 226.5 | 115.0 | -49.2% |
| odt | 179.6 | 99.8 | -44.4% |
| ods | 120.5 | 64.1 | -46.8% |
| odp | 297.9 | 150.2 | -49.6% |
| rtf | 187.9 | 104.3 | -44.5% |
| epub | 78.7 | 43.6 | -44.6% |
| xls | 19.1 | 19.8 | +3.5% |
| xlsx | 52.3 | 65.6 | +25.4% |
| xlsb | conversion failed | 12.5 | newly working sample |
| csv | 38.3 | 22.6 | -41.0% |
| pdf | 577.5 | 1060.4 | +83.6% |

### Performance assessment

- With the duplicate parse gone, single-concurrency sample times for DOC/DOCX/PPT/PPTX/ODT/ODS/ODP/RTF/EPUB drop by about 36%-49%, and CSV by about 42%.
- XLS goes from 69.8 µs to 76.1 µs (about +9%), XLSX from 217.7 µs to 261.2 µs (about +20%). The new parser's number, layout and control handling still costs extra; we cannot claim every format got faster.
- PDF goes from 1.94 ms to 3.63 ms (about +87%). The source adds a second extraction check on pages flagged as scanned, to avoid dropped pages and false detections. This path does not use the document model, so the merged-parse optimization does not apply to it. The completeness check stays; the application's default PDF engine is still builtin, and only the PDF path that selects AnyDoc is affected.
- The Go model gains one checkbox pointer field, so Go allocated bytes rise slightly; the raw output keeps B/op and allocs/op.

### Sustained mixed load

8 workers, rotating through 14 formats, 30 seconds, checking Markdown and image bytes and names on every run.

| Metric | Straight upgrade | Final |
| --- | ---: | ---: |
| Conversions completed | 231,291 | 306,252 |
| Throughput | 7,708.5 docs/s | 10,207.1 docs/s |
| p50 | 0.644 ms | 0.359 ms |
| p95 | 5.320 ms | 5.491 ms |
| p99 | 6.684 ms | 6.843 ms |
| Peak process RSS (Rust included) | 61.1 MiB | 63.3 MiB |

Both runs had zero errors and no output or image mismatches. Final throughput is about 32% higher than the straight upgrade; tail latency did not improve, because PDF takes the longer execution time. RSS includes the latency samples the test keeps, so it cannot simply be attributed to the parser. The test rotates evenly through small documents and cannot be extrapolated to production capacity for large documents.

### Input size tests

| Input | Median time | Go B/op |
| --- | ---: | ---: |
| csv_100_rows | 0.398 ms | 129,360 |
| csv_10000_rows | 32.133 ms | 12,581,816 |
| pdf_40000_operators | 1.074 ms | 152 |
| pdf_200000_operators | 4.346 ms | 152 |
| xlsx_100_rows | 0.592 ms | 82,624 |
| xlsx_10000_rows | 46.186 ms | 7,946,442 |

CSV has three columns and XLSX two; each format grows with the same row structure, and generating and compressing the input is not counted in the conversion time. A 10,000-row XLSX takes about 46.2 ms, and the last row was checked to be present. With bare PDF TJ tokens going from 40,000 to 200,000 (5x the input), the time grows about 4x; the historical quadratic regression did not reappear. Go allocations exclude the Rust native heap. Raw XLSX results: [scaling load test](benchmarks/2026-09-29/xlsx-scaling.txt).

Raw data: [0.1.9](benchmarks/2026-09-29/baseline.txt), [straight upgrade](benchmarks/2026-09-29/upgrade.txt), [final](benchmarks/2026-09-29/optimized.txt), [straight upgrade stress](benchmarks/2026-09-29/upgrade-stress.txt), [final stress](benchmarks/2026-09-29/optimized-stress.txt), [scaling](benchmarks/2026-09-29/scaling.txt), [audit](benchmarks/2026-09-29/audit.txt).

## Reproducing

Functional checks, from the repository root:

```bash
scripts/build-anydoc-lib.sh
git clone --branch v0.2.4 --depth 1 https://github.com/firecrawl/anydoc.git /tmp/anydoc-v0.2.4
ANYDOC_UPSTREAM_DIR=/tmp/anydoc-v0.2.4 go test -tags anydoc -count=1 ./internal/infrastructure/docparser/...
go test -race -tags anydoc -count=1 ./internal/infrastructure/docparser/...
go vet -tags anydoc ./internal/infrastructure/docparser/...
cargo audit --file third_party/anydoc-go/Cargo.lock
```

Performance and mixed load:

```bash
go test -tags anydoc -run '^$' -bench '^BenchmarkConvert$' -benchmem -benchtime=500ms -count=3 -cpu=1,4 ./internal/infrastructure/docparser/anydoc
go test -tags anydoc -run '^$' -bench '^BenchmarkConvertScaling$' -benchmem -benchtime=500ms -count=3 -cpu=1 ./internal/infrastructure/docparser/anydoc
go test -tags anydoc -c -o /tmp/anydoc.test ./internal/infrastructure/docparser/anydoc
cd internal/infrastructure/docparser/anydoc
ANYDOC_STRESS_DURATION=30s /usr/bin/time -l /tmp/anydoc.test -test.run '^TestConcurrentConversionStress$' -test.v
```

On Linux, record RSS with `/usr/bin/time -v`. The test binary must run from the package directory to read the samples. The baseline comparison needs the same benchmark and samples compiled separately against the old version, then the two binaries run one after the other; the old version fails to convert the XLSB sample, so exclude it from the baseline rather than counting the failure in throughput.

## Verification limits

This run was on macOS arm64; Linux glibc/musl and Windows cross builds were not run on this machine. The mixed PDF test uses a stub to verify the fallback request and result; no external OCR service, storage or retrieval index was called. The load test covers the parsing layer only, not upload queuing, network or OCR time. Peak RSS and a short stress run cannot prove there is no long-term native memory leak.
