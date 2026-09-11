# Measure

Empirical only. Builds go in `mktemp -d` and get deleted.

```bash
go build -o "$BUILD_DIR/app" ./cmd/app && ls -lh "$BUILD_DIR/app"
# macOS Peak RSS:  /usr/bin/time -l "$BUILD_DIR/app" … | grep "maximum resident set size"
# Linux Peak RSS:  /usr/bin/time -v "$BUILD_DIR/app" … | grep "Maximum resident set size"
go test -bench=. -benchmem -run=^$ ./...
go tool pprof http://localhost:6060/debug/pprof/heap      # retained
go tool pprof http://localhost:6060/debug/pprof/allocs
go tool pprof http://localhost:6060/debug/pprof/profile?seconds=30
```

`import _ "net/http/pprof"` on a dedicated port. Heap = still held; allocs = churn.

Lifecycle `MemStats` (GC + `FreeOSMemory` first): startup → config/registry → construct → workload → post-GC. Report `HeapAlloc`, `HeapInuse`, `Sys`, `HeapObjects`, `NumGC`.

| Component | Binary | Peak RSS | Flags |
| --------- | ------ | -------- | ----- |
| app | x MB | y MB | … |
