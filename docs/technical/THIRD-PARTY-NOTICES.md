# Third-party notices

The application embeds HTMX and generated Tailwind CSS assets. Their full license texts are included beside the assets under `web/static/` and are embedded in the executable.

| Component | Pin | Use | License notice |
|---|---:|---|---|
| HTMX | 2.0.11 | Local progressive-enhancement JavaScript | [`web/static/js/htmx.LICENSE.txt`](../../web/static/js/htmx.LICENSE.txt) (0BSD) |
| Tailwind CSS and CLI | 4.3.0 | Build-time CSS compilation only | [`web/static/css/tailwind.LICENSE.txt`](../../web/static/css/tailwind.LICENSE.txt) (MIT) |
| `github.com/dustin/go-humanize` | v1.0.1 | Human-readable operational values | MIT |
| `github.com/go-chi/chi/v5` | v5.5.0 | HTTP routing | MIT |
| `github.com/google/uuid` | v1.6.0 | Opaque identifiers and idempotency keys | BSD-3-Clause |
| `github.com/remyoudompheng/bigfft` | 2023-01-30 revision | SQLite arithmetic support | BSD-3-Clause |
| `golang.org/x/crypto` | v0.58.0 | Argon2id password hashing | BSD-3-Clause |
| `golang.org/x/net` | v0.61.0 | Bounded HTML5 document parsing in the extraction package | BSD-3-Clause |
| `golang.org/x/sys` | v0.49.0 | Platform support used by dependencies | BSD-3-Clause |
| `golang.org/x/term` | v0.47.0 | Password reset terminal input | BSD-3-Clause |
| `golang.org/x/text` | v0.43.0 | Unicode normalization in product matching | BSD-3-Clause |
| `modernc.org/libc` | v1.77.1 | SQLite driver support | MIT |
| `modernc.org/mathutil` | v1.7.1 | SQLite driver support | BSD-3-Clause (manually reviewed module `LICENSE`; `go-licenses` reports Unknown) |
| `modernc.org/memory` | v1.12.1 | SQLite driver support | BSD-3-Clause |
| `modernc.org/sqlite` | v1.60.1 | SQLite driver | BSD-3-Clause |

The Go module dependency report is generated with `make license-report`. Scanner output currently reports `modernc.org/mathutil` as Unknown; its module `LICENSE` was manually inspected and is BSD-3-Clause. The repository license for leanPrice itself has not been selected; `go-licenses` therefore reports first-party packages as Unknown. Do not infer a project license from dependency licenses or the product's open-source-first positioning.
