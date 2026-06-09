# Standard: No String Literals

> One of the cross-cutting standards. Pairs with
> [no-magic-numbers](no-magic-numbers.md); the namespaced-dictionary mechanic it
> relies on is owned by [ui-conventions](ui-conventions.md).

## Principle

A string that **carries meaning, identity, or is used in more than one place**
must live in a named constant or a namespaced dictionary — never inline as a bare
literal. The literal text then has one authoritative spelling, one place to
change, and a name that says what it *is*.

This is a Go codebase, so the rule is scoped by intent, not by syntax. We are
eliminating *load-bearing* strings, not the letter sequence.

## What counts as a violation

- **Identity / key strings** used to look something up or match: theme names,
  map keys, widget IDs, tab labels, font/icon resource paths.
- **The same literal appearing twice or more** anywhere in the package.
- **Domain vocabulary**: metric names, connection states (`"LISTEN"`,
  `"ESTABLISHED"`), protocol names (`"tcp"`, `"udp"`), unit suffixes shown in the
  UI.
- **Design-system tokens expressed as strings** (color hex as text, custom theme
  size/color names).

## Allowed — not violations

- **Error and log messages**, including `fmt.Errorf("collect cpu: %w", err)`.
  Wrapping context strings are idiomatic Go and stay inline. (Don't *reuse* the
  same message in two places — at that point extract it.)
- **`fmt` format strings** (`"%5.1f%%"`) at their single use site.
- **Struct tags** and other compile-time-only string syntax.
- **Single-use, self-evident UI prose** at one call site (a one-off window title
  whose text is its own meaning).

When unsure: *Is this string an identifier the program reasons about, or is it
just text the program emits once?* The former is a violation; the latter is fine.

## How it applies in this project

The codebase already models the target well — follow these patterns:

- **Custom theme names are consts, not inline strings.** In
  [`theme.go`](../../internal/ui/theme.go) the secondary-text color name is:

  ```go
  const colorNameTextSecondary fyne.ThemeColorName = "monitor.textSecondary"
  ```

  and the size names live in the `sizeName` struct var
  (`MetricValue: "monitor.metricValue"`, …). Call sites reference
  `sizeName.MetricValue`, never the raw `"monitor.metricValue"`. **Do the same
  for any new theme name.**

- **Namespaced dictionaries over scattered string globals.** The project standard
  (owned by [ui-conventions](ui-conventions.md)) is to group related resources
  into one struct var: `font.SansRegular`, `icon.Overview`, `palette.Accent`,
  `status.Healthy`. New string-keyed resources join the relevant dictionary; they
  don't become a new bare `const fooName = "..."`.

- **Resource paths go through one loader.** Assets load via
  `resource("fonts/…")` / `resource("icons/…")`. The path prefixes are
  centralized in the loader, not retyped at each call.

- **Future: connection states & protocols.** The Connections and Ports tabs
  (not yet built) will deal with `"LISTEN"`, `"tcp"`, etc. Define these as a
  typed-constant set (`type connState string` + `iota`-style consts or a small
  dictionary) the moment they appear — see
  [type-safety](type-safety.md): "prefer named types and typed constants where
  the set of values is known."

## Checklist

- [ ] Is this string an identifier/key/domain term? → named const or dictionary.
- [ ] Does this exact literal already exist elsewhere? → extract and share.
- [ ] Is it a new theme/resource name? → add to the existing namespaced dictionary, not a new loose global.
- [ ] Is it just a one-off error/log/format/prose string? → inline is fine.

## Related

- [no-magic-numbers.md](no-magic-numbers.md) — the numeric twin of this rule.
- [dry.md](dry.md) — a repeated literal is a DRY violation too.
