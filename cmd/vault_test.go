package cmd

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	sdkconfig "github.com/bizshuk/gosdk/config"
	"github.com/bizshuk/gosdk/config/vault"
)

// The crypto and the file format are covered in config/vault. What is only
// testable here is the CLI wiring: which file each subcommand defaults to,
// where its output goes, and that the password comes from the environment so
// the commands work unattended.

const testPassword = "pw-123"

// vaultFixture makes a temp working directory the current one, exports the
// master password, and returns the directory.
func vaultFixture(t *testing.T, files map[string]string) string {
	t.Helper()

	dir := t.TempDir()
	for name, body := range files {
		if err := os.WriteFile(filepath.Join(dir, name), []byte(body), 0o600); err != nil {
			t.Fatalf("write %s: %v", name, err)
		}
	}
	t.Chdir(dir)
	t.Setenv(sdkconfig.VAULT_PASSWORD_ENV, testPassword)
	t.Setenv(sdkconfig.VAULT_TOKEN_ENV, "")
	// Keep the device key inside the temp dir: issuing a token must never
	// write to, or rotate, the developer's real ~/.config key.
	t.Setenv(vault.DEVICE_KEY_ENV, filepath.Join(dir, "device.key"))
	return dir
}

// runVault executes the shared VaultCmd tree and returns its combined output.
func runVault(t *testing.T, args ...string) (string, error) {
	t.Helper()

	var buf bytes.Buffer
	VaultCmd.SetOut(&buf)
	VaultCmd.SetErr(&buf)
	VaultCmd.SetArgs(args)
	t.Cleanup(func() { VaultCmd.SetArgs(nil) })

	err := VaultCmd.Execute()
	return buf.String(), err
}

func TestVaultEncryptThenShow(t *testing.T) {
	vaultFixture(t, map[string]string{".env": "API_KEY=sk-123\nDB_DSN=postgres://h/db\n"})

	out, err := runVault(t, "encrypt")
	if err != nil {
		t.Fatalf("encrypt: %v (%s)", err, out)
	}
	if !strings.Contains(out, "encrypted 2 variables") {
		t.Errorf("encrypt output = %q, want a count of 2", out)
	}
	if _, err := os.Stat(VAULT_FILE); err != nil {
		t.Fatalf("encrypt did not write %s: %v", VAULT_FILE, err)
	}

	out, err = runVault(t, "show")
	if err != nil {
		t.Fatalf("show: %v (%s)", err, out)
	}
	if !strings.Contains(out, "API_KEY=sk-123") {
		t.Errorf("show output = %q, want the decrypted API_KEY", out)
	}
}

// encrypt must not delete or alter the plaintext file: what happens to .env
// afterwards is the operator's decision.
func TestVaultEncryptKeepsPlaintext(t *testing.T) {
	vaultFixture(t, map[string]string{".env": "API_KEY=sk-123\n"})

	if _, err := runVault(t, "encrypt"); err != nil {
		t.Fatalf("encrypt: %v", err)
	}
	body, err := os.ReadFile(".env")
	if err != nil || string(body) != "API_KEY=sk-123\n" {
		t.Fatalf(".env = %q, err = %v; want it untouched", body, err)
	}
}

func TestVaultGetPrintsValueOnly(t *testing.T) {
	vaultFixture(t, map[string]string{".env": "API_KEY=sk-123\nOTHER=x\n"})
	if _, err := runVault(t, "encrypt"); err != nil {
		t.Fatalf("encrypt: %v", err)
	}

	out, err := runVault(t, "get", "API_KEY")
	if err != nil {
		t.Fatalf("get: %v (%s)", err, out)
	}
	if out != "sk-123\n" {
		t.Errorf("get output = %q, want just the value: it is captured with $(...)", out)
	}
}

func TestVaultSetUpdatesInPlace(t *testing.T) {
	vaultFixture(t, map[string]string{".env": "API_KEY=old\n"})
	if _, err := runVault(t, "encrypt"); err != nil {
		t.Fatalf("encrypt: %v", err)
	}

	if _, err := runVault(t, "set", "API_KEY", "new"); err != nil {
		t.Fatalf("set: %v", err)
	}
	v, err := vault.OpenFile(VAULT_FILE, []byte(testPassword))
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	if got, _ := v.Get("API_KEY"); got != "new" {
		t.Errorf("API_KEY = %q, want new", got)
	}
}

// list reads names out of the file itself, so it must work with no password
// available at all.
func TestVaultListNeedsNoPassword(t *testing.T) {
	vaultFixture(t, map[string]string{".env": "B_KEY=2\nA_KEY=1\n"})
	if _, err := runVault(t, "encrypt"); err != nil {
		t.Fatalf("encrypt: %v", err)
	}
	t.Setenv(sdkconfig.VAULT_PASSWORD_ENV, "")

	out, err := runVault(t, "list")
	if err != nil {
		t.Fatalf("list: %v (%s)", err, out)
	}
	if out != "A_KEY\nB_KEY\n" {
		t.Errorf("list output = %q, want both names in sorted order", out)
	}
}

func TestVaultDecryptWritesPlaintextFile(t *testing.T) {
	dir := vaultFixture(t, map[string]string{".env": "API_KEY=sk-123\n"})
	if _, err := runVault(t, "encrypt"); err != nil {
		t.Fatalf("encrypt: %v", err)
	}
	if err := os.Remove(".env"); err != nil {
		t.Fatalf("remove .env: %v", err)
	}

	if _, err := runVault(t, "decrypt"); err != nil {
		t.Fatalf("decrypt: %v", err)
	}
	body, err := os.ReadFile(filepath.Join(dir, ".env"))
	if err != nil {
		t.Fatalf("read .env: %v", err)
	}
	if !strings.Contains(string(body), "API_KEY=sk-123") {
		t.Errorf(".env = %q, want the decrypted value", body)
	}
	st, err := os.Stat(filepath.Join(dir, ".env"))
	if err != nil || st.Mode().Perm() != VAULT_FILE_PERM {
		t.Errorf("mode = %v, want %v", st.Mode().Perm(), os.FileMode(VAULT_FILE_PERM))
	}
}

// A vault whose name does not end in .vault must not be overwritten by its own
// plaintext — that would destroy the only encrypted copy.
func TestPlaintextPathForNeverOverwritesTheVault(t *testing.T) {
	if got := plaintextPathFor(".env.vault"); got != ".env" {
		t.Errorf("plaintextPathFor(.env.vault) = %q, want .env", got)
	}
	if got := plaintextPathFor("secrets"); got != "secrets"+VAULT_DECRYPT_TO {
		t.Errorf("plaintextPathFor(secrets) = %q, want secrets%s", got, VAULT_DECRYPT_TO)
	}
}

func TestVaultWrongPasswordFails(t *testing.T) {
	vaultFixture(t, map[string]string{".env": "API_KEY=sk-123\n"})
	if _, err := runVault(t, "encrypt"); err != nil {
		t.Fatalf("encrypt: %v", err)
	}
	t.Setenv(sdkconfig.VAULT_PASSWORD_ENV, "wrong-password")

	if _, err := runVault(t, "show"); err == nil {
		t.Fatal("show 用錯誤密碼卻成功了")
	}
}

// --- token ----------------------------------------------------------------

// runVaultSplit is runVault with stdout and stderr kept apart, which is the
// whole point for "token": the token goes to stdout and everything else to
// stderr so $(...) captures only the credential.
//
// Execution always goes through VaultCmd, never through a leaf: cobra routes a
// subcommand's Execute() up to its root, so calling VaultTokenCmd.Execute()
// would run VaultCmd with the test binary's own arguments.
func runVaultSplit(t *testing.T, args ...string) (string, string, error) {
	t.Helper()

	var out, errOut bytes.Buffer
	VaultCmd.SetOut(&out)
	VaultCmd.SetErr(&errOut)
	VaultCmd.SetArgs(args)
	t.Cleanup(func() { VaultCmd.SetArgs(nil) })

	err := VaultCmd.Execute()
	return out.String(), errOut.String(), err
}

// issueToken runs "vault token" and returns just the token, the way a shell
// would with $(...).
func issueToken(t *testing.T, args ...string) string {
	t.Helper()

	// VaultTokenCmd is a package-level command, so --ttl survives between
	// runs; reset it so each test starts from the documented default.
	vaultTokenTTL = DEFAULT_TOKEN_TTL

	out, _, err := runVaultSplit(t, append([]string{"token"}, args...)...)
	if err != nil {
		t.Fatalf("token: %v", err)
	}
	return strings.TrimSpace(out)
}

func TestVaultTokenUnlocksWithoutPassword(t *testing.T) {
	vaultFixture(t, map[string]string{".env": "API_KEY=sk-123\n"})
	if _, err := runVault(t, "encrypt"); err != nil {
		t.Fatalf("encrypt: %v", err)
	}
	token := issueToken(t)

	// 密碼從環境移除,只留 token。
	t.Setenv(sdkconfig.VAULT_PASSWORD_ENV, "")
	t.Setenv(sdkconfig.VAULT_TOKEN_ENV, token)

	out, err := runVault(t, "get", "API_KEY")
	if err != nil {
		t.Fatalf("get with token: %v (%s)", err, out)
	}
	if out != "sk-123\n" {
		t.Errorf("get output = %q, want sk-123", out)
	}
}

func TestVaultTokenPrintsExpiryToStderr(t *testing.T) {
	vaultFixture(t, map[string]string{".env": "A=1\n"})
	if _, err := runVault(t, "encrypt"); err != nil {
		t.Fatalf("encrypt: %v", err)
	}

	vaultTokenTTL = DEFAULT_TOKEN_TTL
	stdout, stderr, err := runVaultSplit(t, "token", "--ttl", "30m")
	if err != nil {
		t.Fatalf("token: %v", err)
	}

	if strings.Contains(stdout, "valid until") {
		t.Error("到期資訊跑到 stdout:會污染 $(app vault token) 的取值")
	}
	if !strings.Contains(stderr, "valid until") {
		t.Errorf("stderr = %q, want the expiry line", stderr)
	}
	exp, err := vault.TokenExpiry(strings.TrimSpace(stdout))
	if err != nil {
		t.Fatalf("TokenExpiry: %v", err)
	}
	if d := time.Until(exp); d > 31*time.Minute || d < 29*time.Minute {
		t.Errorf("到期時間距現在 %v, want 約 30m(--ttl 未生效?)", d)
	}
}

// A token must not be able to mint another token: that would let a holder
// extend the deadline forever without ever re-entering the password.
func TestVaultTokenCannotBeMintedFromAToken(t *testing.T) {
	vaultFixture(t, map[string]string{".env": "A=1\n"})
	if _, err := runVault(t, "encrypt"); err != nil {
		t.Fatalf("encrypt: %v", err)
	}
	token := issueToken(t)

	t.Setenv(sdkconfig.VAULT_PASSWORD_ENV, "")
	t.Setenv(sdkconfig.VAULT_TOKEN_ENV, token)

	// 沒有密碼也沒有終端機,發 token 必須失敗而不是沿用 token。
	vaultTokenTTL = DEFAULT_TOKEN_TTL
	if _, _, err := runVaultSplit(t, "token"); err == nil {
		t.Fatal("用 token 發出了新 token")
	}
}

func TestVaultRevokeInvalidatesTokens(t *testing.T) {
	vaultFixture(t, map[string]string{".env": "A=1\n"})
	if _, err := runVault(t, "encrypt"); err != nil {
		t.Fatalf("encrypt: %v", err)
	}
	token := issueToken(t)

	if _, err := runVault(t, "revoke"); err != nil {
		t.Fatalf("revoke: %v", err)
	}

	t.Setenv(sdkconfig.VAULT_PASSWORD_ENV, "")
	t.Setenv(sdkconfig.VAULT_TOKEN_ENV, token)
	if _, err := runVault(t, "show"); err == nil {
		t.Fatal("撤銷後 token 仍然可用")
	}
}
