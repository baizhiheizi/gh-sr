---
name: efficiency-notes
description: Repo-specific efficiency observations and patterns
metadata:
  type: reference
---

- `name + "-" + strconv.Itoa(j)` wins over strings.Builder for InstanceNames (PR #123, #128, #146, #155, #167).
- `fmt.Sscanf` for one float → `strconv.ParseFloat` (PR #191: 3806→452 ns/op, -88%).
- `fmt.Sprintf("%.<prec>f ...")` → `strconv.FormatFloat` + `strings.Builder`. Better: `strconv.AppendFloat` + `[N]byte` stack buffer.
- `string(AppendFloat(...)) + " GiB"` = 1 alloc/call. FormatBytesHuman: 443.5 → 379.9 ns/op (-14.3%).
- **`strings.Builder.String()` is zero-copy**. Stack buffer + `return string(b)` forces copy. HostMetrics.LoadStr regressed 553 vs 385 ns/op.
- **Closures that don't capture enclosing state are stack-allocated by Go escape analysis** — `formatContainerImageBuild` inner `short` closure: 0 allocs/op.
- **`fmt.Sprintf` with only `%s` + one dynamic arg pays reflection + `[]interface{}` alloc** for negligible readability gain over `+` concat when format is static. FooterMain: 11→10 allocs/op.
- **Pre-built `const` strings + simple `if m.loading` branch** outperforms `Sprintf` for two-state help/footer rendering. Pin prefix relation with a one-line test.
- Dead-code `seen map[*T]bool` keyed on `&slice[i]` is always false. PR #155: 297→5 allocs/op.
- Alloc reduction > time reduction in micro-opts: alloc cost compounds in GC pressure.
- `ContainerImageLayoutRevision` is loop-invariant in `Manager.Status` — hoist once per Status(): -85% time, -89% bytes, -50% allocs (PR #226).
- `[N]byte` stack buffer sizing: max realistic output × 1.