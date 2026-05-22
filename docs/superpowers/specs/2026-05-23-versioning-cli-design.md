# Versioning CLI Tool Design

## Overview

Create a `versioning` CLI tool that manages semver version in a `version` file with `major`, `minor`, `patch` subcommands.

## Architecture

```
cmd/versioning/
├── main.go          # Entry point, calls cmd.Execute()
└── cmd/
    ├── root.go      # Root command, reads/writes version file
    ├── major.go     # major subcommand
    ├── minor.go     # minor subcommand
    └── patch.go     # patch subcommand
```

## Version File Format

- Plain text file named `version`
- Content: `<major>.<minor>.<patch>` (e.g., `1.2.3`)
- Default to `0.0.0` if file does not exist

## Core Logic

1. **Read version file**: Parse from `<major>.<minor>.<patch>` format
2. **Increment corresponding version**:
   - `major`: major + 1, reset minor and patch to 0
   - `minor`: minor + 1, reset patch to 0
   - `patch`: patch + 1
3. **Write back to version file**

## Error Handling

- Invalid version file format → print error and exit 1
- Write failure → print error and exit 1

## Usage

```bash
go install github.com/bizshuk/gosdk/cmd/versioning@latest

versioning major   # 1.2.3 → 2.0.0
versioning minor   # 1.2.3 → 1.3.0
versioning patch   # 1.2.3 → 1.2.4
```

## Implementation Notes

- Use `spf13/cobra` for CLI framework (same as `gotmpl`)
- Version file path resolved relative to current working directory
- No external dependencies beyond cobra and standard library