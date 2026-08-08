---
name: golang-runtime-profiling
description: Use when measuring, verifying, or auditing runtime memory usage (HeapAlloc, HeapInuse, Sys, HeapObjects, GC count), peak RSS, or compiled binary artifact sizes across Go modules and workspace executables.
allowed-tools: Bash, Read, Write, Grep, Glob
context: fork
user-invocable: true
disable-model-invocation: false
---

# Go Runtime Memory & Binary Profiling Skill

A specialized workflow for measuring, auditing, and verifying Go runtime memory consumption (`runtime.MemStats`), Peak RSS (Resident Set Size), and compiled binary sizes across Go applications and multi-module `go.work` workspaces.

---

## 1. Overview & Objectives

This skill provides a standardized, empirical methodology to:
1. **In-Process Heap Profiling (`runtime.MemStats`)**: Track `HeapAlloc`, `HeapInuse`, `Sys`, `HeapObjects`, and `NumGC` across process execution stages.
2. **OS Process Peak RSS**: Measure real Peak Resident Set Size using `/usr/bin/time -l` (macOS) or `/usr/bin/time -v` (Linux).
3. **Binary Footprint Analysis**: Build target packages into temporary locations and record binary artifact file sizes.
4. **Dependency & Workspace Overhead Audit**: Compare builds with/without `go.work` (`GOWORK=off`) or isolate dependency tree impact.

---

## 2. Verification Protocol

### Step 1: Workspace & Target Discovery
- Identify project structure (single `go.mod` vs multi-module `go.work`).
- Discover all executable packages (`main.go`, `cmd/*`, `sample/*`, CLI entry points).

### Step 2: Build & Binary Footprint Audit
- Build binaries into a temporary directory using `mktemp -d`:
  ```bash
  BUILD_DIR="$(mktemp -d)"
  go build -o "${BUILD_DIR}/app" ./cmd/app
  ```
- Record binary file size:
  ```bash
  ls -lh "${BUILD_DIR}/app"
  ```

### Step 3: Peak RSS Measurement (OS Level)
- Run target executable under OS memory profiler:
  - **macOS**:
    ```bash
    /usr/bin/time -l "${BUILD_DIR}/app" [args...] 2>&1 | grep "maximum resident set size"
    ```
  - **Linux**:
    ```bash
    /usr/bin/time -v "${BUILD_DIR}/app" [args...] 2>&1 | grep "Maximum resident set size"
    ```

### Step 4: In-Process Heap Profiling (`runtime.MemStats`)
For granular in-process heap tracking, construct a temporary measurement script or inline helper using Go's `runtime.MemStats`:

```go
package main

import (
	"fmt"
	"runtime"
	"runtime/debug"
)

type MemReport struct {
	Step        string
	HeapAllocMB float64
	HeapInuseMB float64
	SysMB       float64
	HeapObjects uint64
	NumGC       uint32
}

func MeasureMem(step string) MemReport {
	runtime.GC()
	debug.FreeOSMemory()
	var m runtime.MemStats
	runtime.ReadMemStats(&m)

	report := MemReport{
		Step:        step,
		HeapAllocMB: float64(m.HeapAlloc) / 1024 / 1024,
		HeapInuseMB: float64(m.HeapInuse) / 1024 / 1024,
		SysMB:       float64(m.Sys) / 1024 / 1024,
		HeapObjects: m.HeapObjects,
		NumGC:       m.NumGC,
	}
	fmt.Printf("| %-35s | %10.2f MB | %10.2f MB | %8.2f MB | %11d | %5d |\n",
		report.Step, report.HeapAllocMB, report.HeapInuseMB, report.SysMB, report.HeapObjects, report.NumGC)
	return report
}
```

#### Lifecycle Inspection Points:
1. **Baseline**: Immediately after process startup.
2. **Registry / Config Load**: After initializing registries, options, or configs.
3. **Instance Construction**: After constructing core domain engines or state structures.
4. **Execution / Runtime Loop**: After executing workload rounds or turns.
5. **Post-GC**: After `runtime.GC()` and `debug.FreeOSMemory()`.

---

## 3. Report Output Format

Always format findings into standard Markdown tables:

```markdown
# Go Runtime Memory & Profiling Report — <Project Name>

## 1. Compiled Binary Footprint
| Component / Target | Binary Size | Command / Flags | Peak RSS (OS Memory) |
|--------------------|-------------|-----------------|----------------------|
| <app>              | X.XX MB     | <args>          | Y.YY MB              |

## 2. In-Process Heap Allocation (`runtime.MemStats`)
| Step                                | HeapAlloc   | HeapInuse   | Sys      | HeapObjects | GC #  |
|-------------------------------------|-------------|-------------|----------|-------------|-------|
| 0. Process Startup Baseline         | X.XX MB     | X.XX MB     | X.XX MB  | N           | N     |
| 1. Registry & Dependencies Loaded   | X.XX MB     | X.XX MB     | X.XX MB  | N           | N     |
| 2. Instance Constructed             | X.XX MB     | X.XX MB     | X.XX MB  | N           | N     |
| 3. Execution Workload               | X.XX MB     | X.XX MB     | X.XX MB  | N           | N     |

## 3. Analysis & Key Insights
- **Heap Efficiency**: Summary of HeapAlloc vs HeapInuse.
- **Sys Overhead**: Analysis of OS virtual memory reserved by runtime.
- **Peak RSS Drivers**: Major dependencies contributing to binary size and Peak RSS.
```

---

## 4. Operating Rules

- **Non-Destructive**: Build outputs must always use temporary directories (`mktemp -d`) and be cleaned up.
- **Empirical Measurements Only**: Do not estimate memory usage; execute tests or profilers to gather exact metrics.
- **High-Level Presentation**: Output high-level component summaries and actionable comparisons.
