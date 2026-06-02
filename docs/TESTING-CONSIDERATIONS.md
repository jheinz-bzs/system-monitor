# Testing Considerations

## What we test

- **Unit — pure logic (primary).** Ring-buffer behavior (capacity, wraparound,
  ordering, ~1 min window), metric aggregation, and any parsing/formatting
  helpers. Fast, deterministic, no system calls. This is where most test value
  lives.
- **Integration — collectors.** The `internal/monitor` collectors that wrap
  gopsutil, exercised against the real OS. Keep these tolerant: assert on shape
  and invariants (non-negative, sane ranges, expected fields populated), not on
  exact machine-specific values. Skip or guard platform-specific paths with
  build tags / `t.Skip` so the suite stays green on Windows, macOS, and Linux.
- **Static gate — `go vet` + `gofmt`.** `make vet` must be clean and `make fmt`
  must produce no diff. This is the cheap baseline every change clears.

## The bar

- **Meaningful logic gets tests; plumbing doesn't.** New pure logic (ring
  buffers, parsing, math, shared cross-tab state) ships with a unit test. Thin
  gopsutil pass-throughs and Fyne widget wiring don't require tests.
- Test files live beside the code as `_test.go`, same package for white-box
  unit tests.
- Run the full suite with `go test ./...`.

## What we skip

- **UI rendering / Fyne widget tests.** Verified manually — run the app and
  confirm tabs render and update live.
- **End-to-end / golden-image tests.** Out of scope for a ~1-month exploration.
- Asserting exact live metric values (they change every second by nature).
