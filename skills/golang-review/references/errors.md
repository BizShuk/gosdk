# Errors, context, DI

- Return errors; `panic` only for programmer bugs in init.
- Wrap at the boundary: `fmt.Errorf("fetch user %s: %w", id, err)`. Sentinel `Err*` in the owning package; typed `*Error` when callers need fields.
- Do not log **and** return — log once at the handler. Check `err` on the next line; never `_ = err` without a comment.
- `ctx` is the first I/O param; never store it on a struct; never pass `nil` (`context.TODO()` + comment). `context.Value` is request-scoped only, typed keys. Honor `ctx.Done()`.
- DI: ≤4 deps as params, 5+ as options struct, many knobs as `...Option` (`golang-dev` layers chapter).
