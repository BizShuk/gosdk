package config

import (
	"log/slog"
	"os"
	"slices"

	"github.com/bizshuk/gosdk/config/vault"
	"github.com/spf13/viper"
)

// This file adds a fourth config format to the loader chain: an encrypted
// dotenv document (.env.vault, scrypt + AES-256-GCM) produced by config/vault.
// It is the only format whose file is safe to commit, which is the whole point
// — a secret can live next to the code that needs it instead of in a separate
// out-of-band channel.
//
// The master password never comes from viper. Everything viper knows is either
// on disk in plaintext or in the environment, so reading the password from it
// would make the vault decryptable by whatever it is meant to protect against.
// It is read from the OS environment only, once, at load time.

const (
	// VAULT_FORMAT is the viper format name and the file extension.
	VAULT_FORMAT = "vault"
	// VAULT_PASSWORD_ENV is the OS environment variable holding the master
	// password. Unset means "no vault": the layer loads nothing and the rest
	// of the chain behaves exactly as before.
	VAULT_PASSWORD_ENV = "VAULT_PASSWORD"
	// VAULT_TOKEN_ENV holds a time-limited token issued by "vault token".
	// It takes precedence over the password: a caller who went to the trouble
	// of scoping a credential meant it to be used.
	VAULT_TOKEN_ENV = "VAULT_TOKEN"

	VAULT_FILE       = ".env.vault"
	VAULT_LOCAL_FILE = ".env.local.vault"
)

func init() { registerVaultExt() }

// registerVaultExt adds "vault" to viper's supported extensions. viper checks
// the format against that package-level list before it ever consults a codec
// registry, so a codec alone is not enough to make a new format readable.
//
// It is idempotent and re-run on every vault load rather than only at init:
// viper.Reset() restores the built-in list, and a format that silently stopped
// working after an unrelated Reset would be a very hard failure to place.
func registerVaultExt() {
	if !slices.Contains(viper.SupportedExts, VAULT_FORMAT) {
		viper.SupportedExts = append(viper.SupportedExts, VAULT_FORMAT)
	}
}

// VaultConfig loads .env.vault and merges .env.local.vault, decrypting both
// with either the master password (VAULT_PASSWORD) or a time-limited token
// (VAULT_TOKEN).
type VaultConfig struct {
	// Password is the master password. NewVaultConfig fills it from the
	// environment; a caller holding the password by other means (a prompt, a
	// secret manager) can construct the struct directly.
	Password []byte

	// Token is a credential issued by vault.Vault.IssueToken. When set it is
	// used instead of Password, and the machine's device key must be readable
	// for it to mean anything.
	Token string
}

// NewVaultConfig returns a loader that takes its credential from VAULT_TOKEN,
// falling back to VAULT_PASSWORD.
func NewVaultConfig() Config {
	return VaultConfig{
		Password: []byte(os.Getenv(VAULT_PASSWORD_ENV)),
		Token:    os.Getenv(VAULT_TOKEN_ENV),
	}
}

// Load reads .env.vault and merges .env.local.vault. Without a credential the
// layer is skipped entirely and an empty viper is returned — an application
// that does not use a vault must not be forced to set anything.
//
// Files are resolved through findInSearchPath rather than handed to viper as a
// config name: viper would expand a name into every supported extension and
// happily feed the plaintext .env to the vault decoder. Resolving the exact
// two names keeps the encrypted layer from ever touching an unencrypted file.
//
// The master password is wiped as soon as the files have been read. That only
// shortens its lifetime — the decrypted values are now strings inside viper,
// and those cannot be cleared. Anything needing a shorter leash than "until
// the process exits" should read it through vault.Vault.GetBytes instead of
// through the config layer.
func (c VaultConfig) Load() *viper.Viper {
	v := viper.NewWithOptions(viper.WithCodecRegistry(c.registry()))
	if c.Token == "" && len(c.Password) == 0 {
		slog.Debug("vault layer skipped: no credential",
			"password_env", VAULT_PASSWORD_ENV, "token_env", VAULT_TOKEN_ENV)
		return v
	}
	defer vault.Wipe(c.Password)

	mergeNamedFiles(v, VAULT_FORMAT, VAULT_FILE, VAULT_LOCAL_FILE)
	return v
}

// registry builds the codec registry for this loader's credential, resolving
// the device key only on the token path — a password user has no reason to own
// one, and demanding it would turn a working setup into an error.
func (c VaultConfig) registry() *viper.DefaultCodecRegistry {
	codec := vault.Codec{Password: c.Password, Token: c.Token}
	if c.Token != "" {
		key, err := vault.LoadDeviceKey(vault.DefaultDeviceKeyPath())
		if err != nil {
			slog.Warn("vault token cannot be used: no device key",
				"path", vault.DefaultDeviceKeyPath(), "err", err)
		}
		codec.DeviceKey = key
	}
	return newVaultRegistry(codec)
}

func (c VaultConfig) GetConfigName() string {
	return VAULT_FILE
}

// newVaultRegistry builds a codec registry that decodes the vault format with
// the given credential, leaving viper's built-in codecs in place for everything
// else. The registry is per-load rather than process-wide because the credential
// is: a shared registry would have to hold a secret for the lifetime of the
// process, and would make two vaults with different passwords unrepresentable.
func newVaultRegistry(codec vault.Codec) *viper.DefaultCodecRegistry {
	registerVaultExt()
	r := viper.NewCodecRegistry()
	_ = r.RegisterCodec(VAULT_FORMAT, codec)
	return r
}
