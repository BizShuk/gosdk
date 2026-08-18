package config

import (
	"testing"

	"github.com/spf13/viper"
)

func TestCurrentProfile_NormalizesValue(t *testing.T) {
	cases := map[string]string{
		"PRODUCTION":  "production",
		"  Staging  ": "staging",
		"":            "",
	}

	for input, want := range cases {
		viper.Reset()
		t.Setenv(PROFILE_KEY, input)

		if got := currentProfile(); got != want {
			t.Errorf("currentProfile() with PROFILE=%q = %q, want %q", input, got, want)
		}
	}
}

func TestIsProduction_ByEnv(t *testing.T) {
	cases := map[string]bool{
		"production":  true,
		"Production":  true,
		"PROD":        true,
		"development": false,
		"staging":     false,
		"":            false,
		"productions": false,
	}

	for input, want := range cases {
		viper.Reset()
		t.Setenv(PROFILE_KEY, input)

		if got := IsProduction(); got != want {
			t.Errorf("IsProduction() with PROFILE=%q = %v, want %v", input, got, want)
		}
	}
}

func TestIsProduction_ByViperKey(t *testing.T) {
	viper.Reset()
	t.Setenv(PROFILE_KEY, "")
	viper.Set(PROFILE_KEY, "production")

	if !IsProduction() {
		t.Error("IsProduction() = false, want true when viper holds PROFILE=production")
	}

	viper.Reset()
}
