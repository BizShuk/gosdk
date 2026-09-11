# Registry + init()

Use when one interface has N implementations and adding one means adding one file. Not for 1–2 impls.

```tree
registry/          # imports no implementation
impl/<name>/       # <name>.go + register.go (one init())
impl/all/all.go    # optional blank-imports
```

`Register` panics on duplicate. `init()` does no I/O. Factory adapts flat `Options` to the impl's own options. Blank-import in `main` only. Inject `LookupEnv func(string) string` — registry must not import viper.

Full sample (package skeleton, guard tests, pitfalls): [registry-pattern.md](registry-pattern.md).
