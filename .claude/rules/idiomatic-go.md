# Rule: Idiomatic Go

Full doc: [docs/conventions/idiomatic-go.md](../../docs/conventions/idiomatic-go.md)

**MUST**
- Run `make fmt`; keep `make vet` clean.
- Handle every error; wrap with `%w`. Return errors from library code.
- Accept interfaces, return concrete types; define interfaces at the consumer.
- Prefer the standard library before adding a dependency.
- Group imports stdlib → third-party → local (blank line between); remove unused.

**MUST NOT**
- `_ =` a meaningful error; `panic` in library code (only unrecoverable startup).
- Introduce a circular import (extract a seam package instead).
- Hand-format instead of `gofmt`.

Note: build/vet go through the Makefile (Fyne needs CGO + `make generate`);
a bare `go vet` on a fresh clone fails until assets are generated.

Self-check: *fmt/vet clean? Every error handled? Interfaces in, concretes out?*
