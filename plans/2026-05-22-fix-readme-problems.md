# Fix README Problems Implementation Plan

> `For agentic workers:` REQUIRED SUB-SKILL: Use `superpowers:subagent-driven-development` (recommended) or `superpowers:executing-plans` to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

`Goal:` Fix multiple architectural, testing, dependency, and CI/CD issues outlined in README.md.

`Architecture:` Consolidate duplicate decode and sleep logic, uniform log format across config files with internal logger, fix sqlite errors, enable helmet middleware, write missing tests for config, time, encode and statsHandler packages, update Dockerfile and implement GitHub actions CI and Makefile.

`Tech Stack:` Go 1.24, Gin, Zap, GORM, GitHub Actions, Docker.

---

### Task 1: Consolidate Duplicate Code

`Files:`

- Modify: [decode.go](file:///Users/shuk/projects/gosdk/encode/io/decode.go)
- Delete: [decode.go](file:///Users/shuk/projects/gosdk/utils/decode.go)
- Delete: [sleep.go](file:///Users/shuk/projects/gosdk/utils/sleep.go)

- [ ] `Step 1: Update encode/io/decode.go to include helper functions`

Modify `encode/io/decode.go` to add `DecodeGBKBytes` and `DecodeBig5Bytes` functions:

```go
package encode

import (
 "bytes"
 "io"

 "golang.org/x/text/encoding/simplifiedchinese"
 "golang.org/x/text/encoding/traditionalchinese"
 "golang.org/x/text/transform"
)

// Decoder defines the interface for character set decoding.
type Decoder interface {
 Decode() io.Reader
}

// DecodeGBKBytes converts GBK bytes to UTF-8 bytes
func DecodeGBKBytes(s []byte) ([]byte, error) {
 r := bytes.NewReader(s)
 tr := transform.NewReader(r, simplifiedchinese.GBK.NewDecoder())
 return io.ReadAll(tr)
}

// DecodeBig5Bytes converts Big5 bytes to UTF-8 bytes
func DecodeBig5Bytes(s []byte) ([]byte, error) {
 r := bytes.NewReader(s)
 tr := transform.NewReader(r, traditionalchinese.Big5.NewDecoder())
 return io.ReadAll(tr)
}
```

- [ ] `Step 2: Delete redundant files`

Remove the duplicate `utils/decode.go` and `utils/sleep.go` files:
Run: `rm utils/decode.go utils/sleep.go`

- [ ] `Step 3: Run vet to verify package compilation`

Run: `go vet ./...`
Expected: PASS

---

### Task 2: Unify Log System

`Files:`

- Modify: [env.go](file:///Users/shuk/projects/gosdk/config/env.go)
- Modify: [yaml.go](file:///Users/shuk/projects/gosdk/config/yaml.go)
- Modify: [db.go](file:///Users/shuk/projects/gosdk/config/db/db.go)
- Modify: [mysql.go](file:///Users/shuk/projects/gosdk/config/db/mysql.go)

- [ ] `Step 1: Refactor config/env.go to use SDK log`

Replace standard library `log` and `fmt` with `github.com/bizshuk/gosdk/log`:

```go
package config

import (
 "github.com/bizshuk/gosdk/log"
 "github.com/spf13/viper"
)

func NewEnvConfig() Config {
 return EnvConfig{}
}

type EnvConfig struct{}

// Add extra config from env, `.env.local` should not commit in git
func (c EnvConfig) Load() *viper.Viper {
 v := viper.New()
 v.AddConfigPath(".")
 v.AddConfigPath("conf")
 v.AddConfigPath(GetConfigDir())

 v.SetConfigName(".env")
 v.SetConfigType("dotenv")
 if err := v.ReadInConfig(); err != nil {
  if _, ok := err.(viper.ConfigFileNotFoundError); ok {
   log.Info("Config file not found. Using defaults and env variables.")
  } else {
   log.Fatalf("Fatal error reading config file: %s", err)
  }
 }

 v.SetConfigName(c.GetConfigName())
 if err := v.MergeInConfig(); err != nil {
  if _, ok := err.(viper.ConfigFileNotFoundError); ok {
   log.Info("Config file not found. Using defaults and env variables.")
  } else {
   log.Fatalf("Fatal error reading config file: %s", err)
  }
 }

 log.Infof("EnvConfig used: %s", v.ConfigFileUsed())
 return v
}

func (c EnvConfig) GetConfigName() string {
 return ".env." + GetProfile()
}
```

- [ ] `Step 2: Refactor config/yaml.go to use SDK log`

```go
package config

import (
 "github.com/bizshuk/gosdk/log"
 "github.com/spf13/viper"
)

type YamlConfig struct{}

func NewYamlConfig() Config {
 return &YamlConfig{}
}

// Load reads the yaml config file and returns a viper instance.
func (c *YamlConfig) Load() *viper.Viper {
 v := viper.New()
 v.AddConfigPath(".")
 v.AddConfigPath("conf")
 v.AddConfigPath(GetConfigDir())

 v.SetConfigName(c.GetConfigName())
 v.SetConfigType("yaml")
 if err := v.ReadInConfig(); err != nil {
  if _, ok := err.(viper.ConfigFileNotFoundError); ok {
   log.Info("Yaml Config file not found. Using defaults and env variables.")
  } else {
   log.Fatalf("Fatal error reading config file: %s", err)
  }
 }

 log.Infof("YamlConfig used: %s", v.ConfigFileUsed())
 return v
}

func (c *YamlConfig) GetConfigName() string {
 profile := GetProfile()
 return "config." + profile
}
```

- [ ] `Step 3: Refactor config/db/db.go to use SDK log`

```go
package db

import (
 "fmt"

 "github.com/bizshuk/gosdk/log"
 "github.com/spf13/viper"
 "gorm.io/gorm"
)

func NewDBConfig(confKey string) DBConfig {
 confKey = "db." + confKey
 dbConfig := DBConfig{}
 if err := viper.UnmarshalKey(confKey, &dbConfig); err != nil {
  log.Fatalf("Unable to unmarshal db key: %v", err)
 }
 log.Infof("Load DBConfig: %+v", dbConfig)
 return dbConfig
}

type DBConfig struct {
 Driver string `mapstructure:"driver"`
 URL    string `mapstructure:"url"`
}

func (d DBConfig) Create() (*gorm.DB, error) {
 return DatabaseFactory(d)
}

func DatabaseFactory(cfg DBConfig) (*gorm.DB, error) {
 switch cfg.Driver {
 case "sqlite":
  return NewSQLite(cfg)
 case "mysql":
  return NewMySQL(cfg)
 default:
  return nil, fmt.Errorf("不支持的資料庫驅動: %s", cfg.Driver)
 }
}
```

- [ ] `Step 4: Refactor config/db/mysql.go to use SDK log`

```go
package db

import (
 "fmt"

 "github.com/bizshuk/gosdk/log"
 "gorm.io/driver/mysql"
 "gorm.io/gorm"
)

func NewMySQL(cfg DBConfig) (*gorm.DB, error) {
 log.Infof("建立 MySQL 連接 (URL: %s)", cfg.URL)

 db, err := gorm.Open(mysql.Open(cfg.URL), &gorm.Config{})
 if err != nil {
  return nil, fmt.Errorf("failed to connect DB: %w", err)
 }

 return db, nil
}
```

- [ ] `Step 5: Run vet to verify unified log package compilation`

Run: `go vet ./...`
Expected: PASS

---

### Task 3: Structuring Configuration

`Files:`

- Modify: [config.go](file:///Users/shuk/projects/gosdk/config/config.go)

- [ ] `Step 1: Define strongly typed config schema and global variable`

Modify `config/config.go` to add `AppConfig` and unmarshal settings:

```go
package config

import (
 "github.com/spf13/viper"
 "go.uber.org/zap"
)

type ServerConfig struct {
 Host string `mapstructure:"host"`
 Port int    `mapstructure:"port"`
}

type DBConnConfig struct {
 Driver string `mapstructure:"driver"`
 URL    string `mapstructure:"url"`
}

type ConfigSchema struct {
 Name     string                  `mapstructure:"name"`
 Version  string                  `mapstructure:"version"`
 Profile  string                  `mapstructure:"profile"`
 LogLevel string                  `mapstructure:"log_level"`
 Server   ServerConfig            `mapstructure:"server"`
 DB       map[string]DBConnConfig `mapstructure:"db"`
}

var GlobalConfig ConfigSchema

type Config interface {
 Load() *viper.Viper
 GetConfigName() string
}

func Default() {
 viper.BindEnv("CONFIG_DIR", "CONFIG_DIR")
 viper.BindEnv("PROFILE", "PROFILE")
 viper.SetDefault("CONFIG_DIR", ".")
 viper.SetDefault("PROFILE", "local")

 zap.L().Info("Load Configure...",
  zap.String("CONFIG_DIR", GetConfigDir()),
 )

 v1 := NewEnvConfig().Load()
 viper.MergeConfigMap(v1.AllSettings())
 v2 := NewYamlConfig().Load()
 viper.MergeConfigMap(v2.AllSettings())

 viper.SetEnvPrefix("APP")
 viper.AutomaticEnv()

 // Unmarshal into strong-typed global configuration schema
 if err := viper.Unmarshal(&GlobalConfig); err != nil {
  zap.L().Fatal("Failed to unmarshal configuration", zap.Error(err))
 }
}

func GetProfile() string {
 profile := viper.GetString("PROFILE")
 if profile != "" {
  return profile
 }
 return "local"
}

func GetConfigDir() string {
 return viper.GetString("CONFIG_DIR")
}
```

- [ ] `Step 2: Run vet to verify強型別設定 compilation`

Run: `go vet ./...`
Expected: PASS

---

### Task 4: Fix SQLite Error Message

`Files:`

- Modify: [sqlite.go](file:///Users/shuk/projects/gosdk/config/db/sqlite.go)

- [ ] `Step 1: Modify SQLite error message`

Update `config/db/sqlite.go`:

```go
package db

import (
 "fmt"

 "github.com/bizshuk/gosdk/log"
 "gorm.io/driver/sqlite"
 "gorm.io/gorm"
)

func NewSQLite(cfg DBConfig) (*gorm.DB, error) {
 log.Infof("Construct SQLite Connection:%s", cfg.URL)

 db, err := gorm.Open(sqlite.Open(cfg.URL), &gorm.Config{})
 if err != nil {
  return nil, fmt.Errorf("SQLite 連接失敗: %w", err)
 }

 return db, nil
}
```

- [ ] `Step 2: Run vet to verify compilation`

Run: `go vet ./...`
Expected: PASS

---

### Task 5: Implement Helmet Middleware

`Files:`

- Modify: [helmet.go](file:///Users/shuk/projects/gosdk/mw/helmet.go)

- [ ] `Step 1: Implement Helmet security headers middleware`

Update `mw/helmet.go`:

```go
package mw

import "github.com/gin-gonic/gin"

// Helmet adds basic security headers to the responses.
func Helmet() gin.HandlerFunc {
 return func(c *gin.Context) {
  c.Header("X-Content-Type-Options", "nosniff")
  c.Header("X-Frame-Options", "DENY")
  c.Header("X-XSS-Protection", "1; mode=block")
  c.Header("Referrer-Policy", "no-referrer")
  c.Header("Content-Security-Policy", "default-src 'self'")
  c.Next()
 }
}
```

- [ ] `Step 2: Run vet to verify compilation`

Run: `go vet ./...`
Expected: PASS

---

### Task 6: Refactor main.go Entrance & CSV Processor Parameterization

`Files:`

- Modify: [main.go](file:///Users/shuk/projects/gosdk/main.go)
- Modify: [processor.go](file:///Users/shuk/projects/gosdk/utils/processor.go)
- Modify: [file.go](file:///Users/shuk/projects/gosdk/utils/file.go)

- [ ] `Step 1: Update main.go to implement startup sequence`

Replace sirupsen/logrus with bizshuk/gosdk/log. Implement setup logic:

```go
package main

import (
 "fmt"

 "github.com/bizshuk/gosdk/config"
 "github.com/bizshuk/gosdk/config/db"
 "github.com/bizshuk/gosdk/log"
 "github.com/bizshuk/gosdk/mw"
 "github.com/bizshuk/gosdk/router"
 "github.com/gin-gonic/gin"
)

func main() {
 // 1. Load Configurations
 config.Default()

 // 2. Re-initialize log systems with configuration level
 log.Init()
 log.Info("Configurations loaded successfully.")

 // 3. Connect DB
 if len(config.GlobalConfig.DB) > 0 {
  _, err := db.NewDBConfig("default").Create()
  if err != nil {
   log.Errorf("Database connection failed: %v", err)
  } else {
   log.Info("Database connected successfully.")
  }
 }

 // 4. Start HTTP Server
 HTTPServer()
}

func HTTPServer() {
 s := gin.Default()
 s.Use(mw.CorrelationID())
 s.Use(mw.Helmet())

 router.Default(s)
 router.HealthRouterGroup(s)
 router.PingRouterGroup(s)

 host := config.GlobalConfig.Server.Host
 port := config.GlobalConfig.Server.Port
 if port == 0 {
  port = 8080
 }
 addr := fmt.Sprintf("%s:%d", host, port)

 log.Infof("Server starting on %s", addr)
 err := s.Run(addr)
 if err != nil {
  log.Fatalf("Server failed to start: %v", err)
 }
}
```

- [ ] `Step 2: Modify CSV Processor parameterization in utils/processor.go`

Add `archive` boolean parameter to `ProcessCSVFile`:

```go
package utils

import (
 "encoding/csv"
 "io"
 "os"

 "go.uber.org/zap"
)

// RecordProcessor is a callback function for processing a single CSV row.
type RecordProcessor func(fname string, row []string) error

// ProcessCSVFile parses a CSV file and iterates over its rows.
// It skips the specified number of header lines and ignores rows with fewer than minCols columns.
// archive parameter determines whether to create .archived mark file.
func ProcessCSVFile(fpath string, archive bool, processor RecordProcessor) error {
 if archive {
  if _, err := os.Stat(fpath + ".archived"); err == nil {
   return nil
  }
 }

 defer func() {
  if archive {
   if _, err := os.Create(fpath + ".archived"); err != nil {
    zap.L().Error("failed to create archived file", zap.Any("file", fpath))
   }
  }
 }()

 f, err := os.Open(fpath)
 if err != nil {
  zap.L().Error("failed to open file", zap.Any("file", fpath), zap.Error(err))
  return err
 }
 defer f.Close()

 r := csv.NewReader(f)
 fname := GetFileName(fpath)

 for i := 0; ; i++ {
  row, err := r.Read()
  if err == io.EOF {
   break
  }
  if i < 1 {
   continue
  }
  if len(row) < 2 {
   continue
  }
  if err := processor(fname, row); err != nil {
   zap.L().Error("process row failed", zap.Error(err))
   continue
  }
 }

 return nil
}
```

- [ ] `Step 3: Update utils/file.go to pass archive parameter`

```go
func NewCSVFilelistCallback(pattern string, rowProcessor RecordProcessor) error {
 fileList, err := filepath.Glob(pattern)
 if err != nil {
  zap.L().Error("file glob failed", zap.Any("pattern", pattern), zap.Error(err))
  return err
 }

 for _, fpath := range fileList {
  // Default to archive=true for backward compatibility in file list callback
  if err := ProcessCSVFile(fpath, true, rowProcessor); err != nil {
   return err
  }
 }
 return nil
}
```

- [ ] `Step 4: Cleanup dependency and run tidy`

Run: `go mod tidy`
Expected: `github.com/sirupsen/logrus` removed from `go.mod`.

---

### Task 7: Comprehensive Unit Testing

`Files:`

- Modify: [statsHandler_test.go](file:///Users/shuk/projects/gosdk/router/statsHandler_test.go)
- Create: [config_test.go](file:///Users/shuk/projects/gosdk/config/config_test.go)
- Create: [sleep_test.go](file:///Users/shuk/projects/gosdk/time/sleep_test.go)
- Create: [roc_test.go](file:///Users/shuk/projects/gosdk/time/roc_test.go)
- Create: [decode_test.go](file:///Users/shuk/projects/gosdk/encode/io/decode_test.go)

- [ ] `Step 1: Write statsHandler_test.go`

```go
package router

import (
 "encoding/json"
 "net/http"
 "net/http/httptest"
 "testing"

 "github.com/bizshuk/gosdk/mw"
 "github.com/gin-gonic/gin"
 "github.com/spf13/viper"
)

func TestStatsHandler(t *testing.T) {
 gin.SetMode(gin.TestMode)

 viper.Set("Version", "1.0.0")
 viper.Set("PROFILE", "test")
 viper.Set("viper.file", "config.test.yaml")

 s := gin.New()
 s.Use(mw.CorrelationID())
 s.GET("/stats", StatsHandler)

 req, _ := http.NewRequest("GET", "/stats", nil)
 w := httptest.NewRecorder()
 s.ServeHTTP(w, req)

 if w.Code != http.StatusOK {
  t.Errorf("Expected status code %d, got %d", http.StatusOK, w.Code)
 }

 var stats Stats
 err := json.Unmarshal(w.Body.Bytes(), &stats)
 if err != nil {
  t.Fatalf("Failed to unmarshal response: %v", err)
 }

 if stats.Version != "1.0.0" {
  t.Errorf("Expected version 1.0.0, got %s", stats.Version)
 }
 if stats.Profile != "test" {
  t.Errorf("Expected profile test, got %s", stats.Profile)
 }
 if stats.ConfigFile != "config.test.yaml" {
  t.Errorf("Expected configFile config.test.yaml, got %s", stats.ConfigFile)
 }
 if stats.Status != "OK" {
  t.Errorf("Expected status OK, got %s", stats.Status)
 }
 if stats.CorrelationId == "" {
  t.Error("Expected CorrelationId to be generated, got empty string")
 }
}
```

- [ ] `Step 2: Create config/config_test.go`

```go
package config

import (
 "os"
 "testing"

 "github.com/spf13/viper"
)

func TestDefaultConfig(t *testing.T) {
 os.Setenv("CONFIG_DIR", ".")
 os.Setenv("PROFILE", "local")
 defer os.Unsetenv("CONFIG_DIR")
 defer os.Unsetenv("PROFILE")

 Default()

 profile := GetProfile()
 if profile != "local" {
  t.Errorf("Expected profile local, got %s", profile)
 }

 dir := GetConfigDir()
 if dir != "." {
  t.Errorf("Expected config dir ., got %s", dir)
 }
}
```

- [ ] `Step 3: Create time/sleep_test.go`

```go
package time

import (
 "testing"
 "time"

 "github.com/spf13/viper"
)

func TestConfigSleep(t *testing.T) {
 viper.Set("test.sleep.delay", 10)
 defer viper.Set("test.sleep.delay", nil)

 start := time.Now()
 ConfigSleep("test.sleep.delay")
 duration := time.Since(start)

 if duration < 10*time.Millisecond {
  t.Errorf("Expected sleep duration at least 10ms, got %v", duration)
 }
}
```

- [ ] `Step 4: Create time/roc_test.go`

```go
package time

import (
 "testing"
)

func TestParseROCDate(t *testing.T) {
 res := ParseROCDate("112/05/20")
 if res.Year() != 2023 || res.Month() != 5 || res.Day() != 20 {
  t.Errorf("Expected 2023-05-20, got %v", res)
 }

 resErr := ParseROCDate("invalid")
 if !resErr.IsZero() {
  t.Errorf("Expected zero time for invalid date, got %v", resErr)
 }
}
```

- [ ] `Step 5: Create encode/io/decode_test.go`

```go
package encode

import (
 "bytes"
 "testing"
)

func TestDecodeGBKBytes(t *testing.T) {
 // GBK bytes for "中文"
 gbkBytes := []byte{0xd6, 0xd0, 0xce, 0xc4}
 res, err := DecodeGBKBytes(gbkBytes)
 if err != nil {
  t.Fatalf("Failed to decode GBK bytes: %v", err)
 }
 if string(res) != "中文" {
  t.Errorf("Expected 中文, got %s", string(res))
 }
}

func TestDecodeBig5Bytes(t *testing.T) {
 // Big5 bytes for "中文"
 big5Bytes := []byte{0xa4, 0xa4, 0xa4, 0xe5}
 res, err := DecodeBig5Bytes(big5Bytes)
 if err != nil {
  t.Fatalf("Failed to decode Big5 bytes: %v", err)
 }
 if string(res) != "中文" {
  t.Errorf("Expected 中文, got %s", string(res))
 }
}
```

- [ ] `Step 6: Run tests to verify code coverage and correctness`

Run: `go test -v ./...`
Expected: ALL TESTS PASS

---

### Task 8: Add Makefile, Dockerfile and GitHub Actions Workflow

`Files:`

- Create: [Makefile](file:///Users/shuk/projects/gosdk/Makefile)
- Modify: [dockerfile](file:///Users/shuk/projects/gosdk/build/dockerfile)
- Create: [.github/workflows/ci.yml](file:///Users/shuk/projects/gosdk/.github/workflows/ci.yml)

- [ ] `Step 1: Create Makefile`

```makefile
.PHONY: build test generate run clean help

all: build

build:
 @echo "Building application..."
 go build -o bin/server main.go

test:
 @echo "Running tests..."
 go test -v ./...

generate:
 @echo "Generating code..."
 go generate ./...

run: build
 @echo "Running application..."
 ./bin/server

clean:
 @echo "Cleaning up..."
 rm -rf bin/

help:
 @echo "Available targets:"
 @echo "  build    - Build the server binary"
 @echo "  test     - Run Go tests"
 @echo "  generate - Run go generate for stringer"
 @echo "  run      - Build and run the server"
 @echo "  clean    - Clean up build artifacts"
```

- [ ] `Step 2: Update build/dockerfile`

```dockerfile
# Build stage
FROM golang:1.24-alpine AS builder

WORKDIR /app

# Copy dependency manifests
COPY go.mod go.sum ./
RUN go mod download

# Copy source code
COPY . .

# Build the application
RUN CGO_ENABLED=0 GOOS=linux go build -o /app/server main.go

# Production stage
FROM alpine:latest

WORKDIR /app

# Copy pre-built binary
COPY --from=builder /app/server /app/server

# Expose HTTP port
EXPOSE 8080

CMD ["./server"]
```

- [ ] `Step 3: Create .github/workflows/ci.yml`

```yaml
name: Go CI

on:
    push:
        branches: [main, master]
    pull_request:
        branches: [main, master]

jobs:
    build:
        name: Build and Test
        runs-on: ubuntu-latest
        steps:
            - name: Check out code
              uses: actions/checkout@v4

            - name: Set up Go
              uses: actions/setup-go@v5
              with:
                  go-version: "1.24"
                  cache: true

            - name: Install dependencies
              run: go mod download

            - name: Run Vet
              run: go vet ./...

            - name: Run Tests
              run: go test -v ./...

            - name: Build
              run: go build -v ./...
```

- [ ] `Step 4: Run make test to verify Makefile integration`

Run: `make test`
Expected: ALL TESTS PASS
