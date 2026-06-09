# Rule: DRY (Don't Repeat Yourself)

Full doc: [docs/conventions/dry.md](../../docs/conventions/dry.md)

**MUST**
- Give each piece of knowledge (value, rule, calculation, layout decision) one
  authoritative home.
- Reference existing tokens/helpers (`palette`, `space*`, `metrics` math,
  `valueToY`/`niceNum`/`formatCompact`) instead of re-typing or reimplementing.
- Put shared cross-collector math in `internal/metrics`; shared chart math in the
  chart helpers — not copy-pasted per collector/tab.
- Apply the **rule of three**: extract only on the third real occurrence.

**MUST NOT**
- Copy a named value (color, size, threshold, duration, key) to a second site.
- Abstract on occurrence #2, or couple two snippets that merely *look* alike but
  change for different reasons.

**Allowed (do not flag)**
- Coincidental structural similarity; explicit per-site Go error handling; clear
  explicit test setup.

Self-check: *Is this the third occurrence of the same knowledge, or just
similar-looking code with different reasons to change?*
