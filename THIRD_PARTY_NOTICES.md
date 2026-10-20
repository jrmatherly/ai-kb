# Third-party notices and corresponding source

This file supplements the third-party notices in `LICENSE`. The MIT license
for AI KB's own code does not replace the licenses of third-party components.
Keep this file, `LICENSE`, and the `licenses/` directory with redistributed
backend and desktop packages. `scripts/copy-licenses.sh` adds checksum-verified
source archives at packaging time; they are not stored in this Git repository.
The app image includes the complete bundle in `/app`. The Lite archive and the
Linux desktop archive include it at their top level, and the macOS application in
`Contents/Resources` and the Windows installer next to the installed executable.
The ui, docreader and sandbox images carry no bundle; the sections below say what
they ship.

## Go MySQL Driver

- Component: `github.com/go-sql-driver/mysql`, version `v1.10.1`.
- Used in backend and desktop binaries, including the Doris MySQL protocol driver.
- License: Mozilla Public License 2.0, reproduced in
  [`licenses/go-sql-driver-mysql-MPL-2.0.txt`](licenses/go-sql-driver-mysql-MPL-2.0.txt).
- Copyright: The Go-MySQL-Driver Authors; the original per-file notices and
  `AUTHORS` are preserved in the accompanying source archive.
- Modifications by AI KB: none. Dialer configuration is in separate AI KB files.
- Corresponding source, available under MPL-2.0, is included in the app image, the
  Lite and Linux desktop archives, the macOS application and the Windows installer as
  `licenses/sources/mysql-v1.10.1.zip`. Everyone can obtain it from the
  [Go module proxy](https://proxy.golang.org/github.com/go-sql-driver/mysql/@v/v1.10.1.zip)
  and [upstream repository](https://github.com/go-sql-driver/mysql/tree/v1.10.1).

## OpenCC dictionary data

- Files: `internal/textconv/data/TSPhrases.txt` and `TSCharacters.txt`, copied
  unchanged from `github.com/longbridgeapp/opencc` version `v0.3.13`.
- Attribution: the OpenCC and longbridge/opencc contributors.
- License: Apache-2.0, reproduced in
  [`licenses/OpenCC-Apache-2.0.txt`](licenses/OpenCC-Apache-2.0.txt).
- Source: [versioned dictionary directory](https://github.com/longbridgeapp/opencc/tree/v0.3.13/dictionary)
  and the two text files distributed with AI KB's source.
- Only dictionary data is retained. AI KB uses its own standard-library lookup
  implementation; the upstream Go converter, `liuzl/da`, and GPL-licensed
  `cedar-go` code are not included.

## noVNC

- Component: `@novnc/novnc`, version `1.7.0` (`frontend/package-lock.json`).
- Used by the sandbox desktop viewer (`frontend/src/composables/useSandboxDesktop.ts`).
  It ships, bundled and minified, in the frontend build: the ui image, the Lite
  archive's `web/` directory and the desktop applications.
- License: Mozilla Public License 2.0, the same text as
  [`licenses/go-sql-driver-mysql-MPL-2.0.txt`](licenses/go-sql-driver-mysql-MPL-2.0.txt).
- Copyright: the noVNC authors (`AUTHORS` in the source).
- Modifications by AI KB: none. The package is used as published on npm.
- Corresponding source, available under MPL-2.0: the
  [npm package](https://registry.npmjs.org/@novnc/novnc/-/novnc-1.7.0.tgz) and the
  [upstream repository](https://github.com/novnc/noVNC/tree/v1.7.0).

## elkjs

- Component: `elkjs`, version `0.9.3` (`frontend/package-lock.json`).
- Used by mermaid's ELK layout. It ships as a lazily loaded chunk of the frontend
  build: the ui image, the Lite archive's `web/` directory and the desktop applications.
- License: Eclipse Public License 2.0, reproduced in
  [`licenses/elkjs-EPL-2.0.txt`](licenses/elkjs-EPL-2.0.txt).
- Copyright: the Eclipse Layout Kernel contributors.
- Modifications by AI KB: none. The package is used as published on npm.
- Corresponding source, available under EPL-2.0: the
  [npm package](https://registry.npmjs.org/elkjs/-/elkjs-0.9.3.tgz) and the
  [upstream repository](https://github.com/kieler/elkjs/tree/v0.9.3).

Frontend packages published under a choice of licenses (dompurify, jszip) are used
under their permissive option and are not listed. khroma `2.1.0`, a mermaid
dependency, declares no license in its package metadata; its
[upstream repository](https://github.com/fabiospampinato/khroma) is MIT-licensed.

## docreader Python packages

The docreader image installs its locked Python environment (`docreader/uv.lock`)
into `/app/.venv`. Each package's license text ships with it, in that package's
`*.dist-info` directory under `site-packages`. Packages with copyleft or
weak-copyleft licenses:

- certifi `2026.7.22`, Mozilla Public License 2.0. Unmodified; source on
  [PyPI](https://pypi.org/project/certifi/2026.7.22/) and in the
  [upstream repository](https://github.com/certifi/python-certifi).
- tld `0.13.2`, MPL-1.1 OR GPL-2.0-only OR LGPL-2.1-or-later, a dependency of
  `courlan`. Unmodified; source on [PyPI](https://pypi.org/project/tld/0.13.2/)
  and in the [upstream repository](https://github.com/barseghyanartur/tld).

## Build-only component

The Windows installer template in `cmd/desktop/build/windows/installer/project.nsi`
is based on Wails v2.12.0's MIT-licensed default template (the build uses Wails
v2.16.0), with local changes to
install this notice/source bundle. Its license and copyright are reproduced in
[`licenses/Wails-MIT.txt`](licenses/Wails-MIT.txt).

`cbindgen` 0.29.4 (MPL-2.0) generates the AnyDoc C ABI header during the Rust
build. Its code/tool executable is not copied into the runtime image or release
packages by the current build recipes. Generating the header does not copy the
tool's implementation into the product. If distributing build environments or
the tool itself, retain its license and provide its corresponding source too.

## Maintaining this bundle

When upgrading an MPL module, update its version, license, and SHA-256 pin in
`licenses/sources/modules.tsv` together. Archives are unmodified Go module proxy
ZIPs, including their original copyright notices. Verify them with
`go mod download -json` and the module's `go.sum` entry before updating.
`scripts/check-license-bundle.sh` checks the pins and notices without downloading
dependencies. `licenses/components.tsv` lists the reviewed npm and Python components;
the check requires this file to name each one's version from `frontend/package-lock.json`
or `docreader/uv.lock`, fails on any other frontend production package whose lockfile
license is copyleft or missing, and does the same for the docreader runtime set when
`docreader/.venv` is installed. `scripts/copy-licenses.sh DESTINATION` obtains the pinned archives
through Go's configured `GOPROXY` and module cache, verifies their SHA-256 hashes,
and includes them in the destination bundle. Offline packaging requires those
module archives to be present in the Go module cache beforehand. To verify a
packaged source directory, run `scripts/check-license-bundle.sh PATH/TO/licenses/sources`.
