# Standard: Type Safety

## Principle

Let the type system carry the invariants. The narrowest type that fits a value
makes illegal states unrepresentable and turns a class of runtime bugs into
compile errors. Reserve `interface{}`/`any` for when you genuinely don't know the
type — which is almost never here.

## Rules

- **Use the narrowest type that works** for parameters and struct fields. If a
  count can't be negative and the API is internal, that's still usually a Go
  `int` — but don't widen to `int64`/`float64` "just in case," and don't pass a
  whole struct where one field is used.
- **Prefer named types and typed constants** (`iota` enums) over bare
  `int`/`string` **where the set of values is known**. A `type connState string`
  with a fixed set of consts beats passing raw `"LISTEN"`/`"ESTABLISHED"` strings
  around (this is also where type safety meets
  [no-string-literals](no-string-literals.md)).
- **Don't pass `interface{}`/`any` where a real type fits.** An `any` parameter
  pushes type errors to runtime and erases documentation.

## How it applies in this project

- The chart uses a **typed numeric constraint** rather than `any`:
  [`linechart.go`](../../internal/ui/linechart.go) defines a `numeric` interface
  and `sourceFrom[T numeric](...)` so ring buffers of `int`/`float64` adapt
  type-safely — no `interface{}` round-tripping of samples.
- Fyne's own typed names are used as named types, not raw strings:
  `fyne.ThemeColorName` / `fyne.ThemeSizeName` back `colorNameTextSecondary` and
  the `sizeName` entries ([`theme.go`](../../internal/ui/theme.go)).
- **Future — connection/port domains.** When the Connections and Ports tabs land,
  model states and protocols as typed constant sets (`type connState string` +
  consts, or an `iota` enum), not bare strings/ints. The known value set is the
  trigger for a named type.
- Sizes coerce cleanly because the spacing scale is declared as untyped numeric
  consts (`baseUnit`-derived) — typed *enough* to be safe, untyped *enough* to
  avoid casts at each Fyne call site (see
  [no-magic-numbers](no-magic-numbers.md)).

## Checklist

- [ ] Is this the narrowest type that fits (not a needless widening)?
- [ ] Is the set of values known? → named type + typed constants (`iota`).
- [ ] Am I about to use `any`/`interface{}` where a concrete or generic-constrained type fits? → don't.
- [ ] Am I passing a whole struct where the callee needs one field?

## Related

- [no-string-literals.md](no-string-literals.md) — typed-constant sets replace stringly-typed domains.
- [idiomatic-go.md](idiomatic-go.md) — accept interfaces, return concrete types.
- [naming.md](naming.md) — naming named types and their constants.
