package config

import (
	"os"
	"path/filepath"
	"slices"
	"testing"
	"time"

	"github.com/bizshuk/gosdk/config/vault"
	"github.com/spf13/viper"
)

// writeVault encrypts entries with password and writes them to dir/name.
func writeVault(t *testing.T, dir, name, password string, entries map[string]any) string {
	t.Helper()
	data, err := vault.Codec{Password: []byte(password)}.Encode(entries)
	if err != nil {
		t.Fatalf("encode vault: %v", err)
	}
	path := filepath.Join(dir, name)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("mkdir for %s: %v", name, err)
	}
	if err := os.WriteFile(path, data, 0o600); err != nil {
		t.Fatalf("write %s: %v", name, err)
	}
	return path
}

// viper.Reset() restores the built-in extension list, so the format has to be
// re-registered by the loader itself, not once at init.
func TestVaultFormatIsRegisteredWithViper(t *testing.T) {
	viper.Reset()
	t.Cleanup(viper.Reset)
	NewVaultConfig().Load()

	if !slices.Contains(viper.SupportedExts, VAULT_FORMAT) {
		t.Fatalf("viper.SupportedExts = %v, missing %q", viper.SupportedExts, VAULT_FORMAT)
	}
}

func TestVaultConfig_LoadsBaseAndLocal(t *testing.T) {
	wd, _ := sourceFixture(t, "")
	t.Setenv(VAULT_PASSWORD_ENV, "pw")

	writeVault(t, wd, VAULT_FILE, "pw", map[string]any{
		"API_KEY": "from-base",
		"DB_DSN":  "base-dsn",
	})
	writeVault(t, wd, VAULT_LOCAL_FILE, "pw", map[string]any{
		"API_KEY": "from-local",
	})

	v := NewVaultConfig().Load()
	if got := v.GetString("API_KEY"); got != "from-local" {
		t.Errorf("API_KEY = %q, want from-local (.local must override the base file)", got)
	}
	if got := v.GetString("DB_DSN"); got != "base-dsn" {
		t.Errorf("DB_DSN = %q, want base-dsn", got)
	}
}

// Without a password the layer contributes nothing and does not fail: an
// application with no vault must keep starting normally.
func TestVaultConfig_NoPasswordLoadsNothing(t *testing.T) {
	wd, _ := sourceFixture(t, "")
	t.Setenv(VAULT_PASSWORD_ENV, "")

	writeVault(t, wd, VAULT_FILE, "pw", map[string]any{"API_KEY": "secret"})

	if got := NewVaultConfig().Load().AllSettings(); len(got) != 0 {
		t.Errorf("AllSettings() = %v, want empty when no password is set", got)
	}
}

func TestVaultConfig_WrongPasswordLoadsNothing(t *testing.T) {
	wd, _ := sourceFixture(t, "")
	t.Setenv(VAULT_PASSWORD_ENV, "wrong")

	writeVault(t, wd, VAULT_FILE, "right", map[string]any{"API_KEY": "secret"})

	if got := NewVaultConfig().Load().AllSettings(); len(got) != 0 {
		t.Errorf("AllSettings() = %v, want empty when the password is wrong", got)
	}
}

// The vault layer resolves two exact file names. A plaintext .env sitting in
// the same directory must never reach the decoder.
func TestVaultConfig_IgnoresPlainDotenv(t *testing.T) {
	wd, _ := sourceFixture(t, "")
	t.Setenv(VAULT_PASSWORD_ENV, "pw")

	writeFile(t, wd, ".env", "API_KEY=plaintext\n")

	if got := NewVaultConfig().Load().AllSettings(); len(got) != 0 {
		t.Errorf("AllSettings() = %v, want empty: .env is not a vault file", got)
	}
}

func TestVaultConfig_SearchesAppConfigDir(t *testing.T) {
	_, appDir := sourceFixture(t, "vaultapp")
	t.Setenv(VAULT_PASSWORD_ENV, "pw")

	writeVault(t, appDir, VAULT_FILE, "pw", map[string]any{"API_KEY": "from-app-dir"})

	if got := NewVaultConfig().Load().GetString("API_KEY"); got != "from-app-dir" {
		t.Errorf("API_KEY = %q, want from-app-dir", got)
	}
}

func TestSources_ResolvesVaultFiles(t *testing.T) {
	wd, _ := sourceFixture(t, "")
	want := writeVault(t, wd, VAULT_FILE, "pw", map[string]any{"A": "1"})

	got := sourceFor(t, VAULT_FILE)
	if got.Layer != "vault" {
		t.Errorf("Layer = %q, want vault", got.Layer)
	}
	if abs, _ := filepath.Abs(want); got.Path != abs {
		t.Errorf("Path = %q, want %q", got.Path, abs)
	}
}

func TestLoadFile_VaultLayerDecrypts(t *testing.T) {
	wd, _ := sourceFixture(t, "")
	t.Setenv(VAULT_PASSWORD_ENV, "pw")
	path := writeVault(t, wd, VAULT_FILE, "pw", map[string]any{"API_KEY": "secret"})

	v, err := LoadFile(path, "vault")
	if err != nil {
		t.Fatalf("LoadFile: %v", err)
	}
	if got := v.GetString("API_KEY"); got != "secret" {
		t.Errorf("API_KEY = %q, want secret", got)
	}
}

func TestLoadFile_VaultLayerFailsWithoutPassword(t *testing.T) {
	wd, _ := sourceFixture(t, "")
	t.Setenv(VAULT_PASSWORD_ENV, "")
	path := writeVault(t, wd, VAULT_FILE, "pw", map[string]any{"API_KEY": "secret"})

	if _, err := LoadFile(path, "vault"); err == nil {
		t.Fatal("LoadFile 沒有回報錯誤:缺密碼時不應該安靜地回傳空設定")
	}
}

// Cross-format precedence: the vault sits above settings.json and below .env,
// so a committed secret loses to a developer's local override.
func TestLoadAllConfigs_VaultPrecedence(t *testing.T) {
	wd, _ := sourceFixture(t, "")
	t.Setenv(VAULT_PASSWORD_ENV, "pw")
	viper.Reset()
	t.Cleanup(viper.Reset)

	writeFile(t, wd, "settings.json", `{"API_KEY": "from-json", "TOKEN": "json-token"}`)
	writeVault(t, wd, VAULT_FILE, "pw", map[string]any{
		"API_KEY": "from-vault",
		"DB_DSN":  "vault-dsn",
	})
	writeFile(t, wd, ".env", "API_KEY=from-env\n")

	if err := loadAllConfigs(); err != nil {
		t.Fatalf("loadAllConfigs: %v", err)
	}
	for key, want := range map[string]string{
		"API_KEY": "from-env",   // .env outranks the vault
		"DB_DSN":  "vault-dsn",  // only the vault has it
		"TOKEN":   "json-token", // the vault does not clear other layers
	} {
		if got := viper.GetString(key); got != want {
			t.Errorf("viper.GetString(%q) = %q, want %q", key, got, want)
		}
	}
}

// --- token credential ------------------------------------------------------

// The config layer must accept a time-limited token in place of the master
// password: that is the whole point of issuing one for a build agent.
func TestVaultConfig_LoadsWithToken(t *testing.T) {
	wd, _ := sourceFixture(t, "")
	t.Setenv(VAULT_PASSWORD_ENV, "")
	t.Setenv(vault.DEVICE_KEY_ENV, filepath.Join(wd, "device.key"))

	writeVault(t, wd, VAULT_FILE, "pw", map[string]any{"API_KEY": "sk-123"})
	deviceKey, err := vault.EnsureDeviceKey(vault.DefaultDeviceKeyPath())
	if err != nil {
		t.Fatalf("EnsureDeviceKey: %v", err)
	}
	v, err := vault.OpenFile(filepath.Join(wd, VAULT_FILE), []byte("pw"))
	if err != nil {
		t.Fatalf("OpenFile: %v", err)
	}
	token, err := v.IssueToken(deviceKey, time.Hour)
	v.Close()
	if err != nil {
		t.Fatalf("IssueToken: %v", err)
	}
	t.Setenv(VAULT_TOKEN_ENV, token)

	if got := NewVaultConfig().Load().GetString("API_KEY"); got != "sk-123" {
		t.Errorf("API_KEY = %q, want sk-123 (token 未被採用?)", got)
	}
}

// A token this machine cannot unwrap — expired, forged, or issued elsewhere —
// yields no values rather than an empty-but-successful load. The expiry rule
// itself is tested in config/vault, where the clock can be pinned; here the
// question is only what the config layer does with a token it cannot use.
func TestVaultConfig_UnusableTokenLoadsNothing(t *testing.T) {
	wd, _ := sourceFixture(t, "")
	t.Setenv(VAULT_PASSWORD_ENV, "")
	t.Setenv(vault.DEVICE_KEY_ENV, filepath.Join(wd, "device.key"))

	writeVault(t, wd, VAULT_FILE, "pw", map[string]any{"API_KEY": "sk-123"})
	if _, err := vault.EnsureDeviceKey(vault.DefaultDeviceKeyPath()); err != nil {
		t.Fatalf("EnsureDeviceKey: %v", err)
	}
	// 在另一台「機器」(另一把裝置金鑰)上發出的 token。
	t.Setenv(VAULT_TOKEN_ENV, foreignToken(t, wd))

	if got := NewVaultConfig().Load().AllSettings(); len(got) != 0 {
		t.Errorf("AllSettings() = %v, want empty:無法使用的 token 不該解出任何值", got)
	}
}

// foreignToken issues a valid token under a device key this machine does not
// have, which is what a leaked-and-replayed token looks like from here.
func foreignToken(t *testing.T, wd string) string {
	t.Helper()

	otherKey, err := vault.EnsureDeviceKey(filepath.Join(t.TempDir(), "other.key"))
	if err != nil {
		t.Fatalf("EnsureDeviceKey: %v", err)
	}
	v, err := vault.OpenFile(filepath.Join(wd, VAULT_FILE), []byte("pw"))
	if err != nil {
		t.Fatalf("OpenFile: %v", err)
	}
	defer v.Close()

	token, err := v.IssueToken(otherKey, time.Hour)
	if err != nil {
		t.Fatalf("IssueToken: %v", err)
	}
	return token
}
