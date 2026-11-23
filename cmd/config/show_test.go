package config

import (
	"os"
	"path/filepath"
	"testing"

	sdkconfig "github.com/bizshuk/gosdk/v2/config"
	"github.com/spf13/viper"
)

// The WithDefaultValue layer is part of the merged view: keys only it has show
// source "default", and a file value that overrides it lists it as shadowed.
func TestShow_IncludesDefaultValueLayer(t *testing.T) {
	dir := appDirFixture(t, "showdefault")
	t.Chdir(t.TempDir())
	viper.Reset()
	t.Cleanup(func() { viper.Reset(); sdkconfig.Default() })

	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "settings.json"), []byte(`{"port": 9000}`), 0o644); err != nil {
		t.Fatal(err)
	}
	sdkconfig.Default(sdkconfig.WithAppName("showdefault"),
		sdkconfig.WithDefaultValue(`{"port": 8080, "server": {"host": "0.0.0.0"}}`))

	byKey := map[string]Entry{}
	for _, e := range Show() {
		byKey[e.Key] = e
	}

	host := byKey["server.host"]
	if host.Value != "0.0.0.0" || host.Source != sdkconfig.LAYER_DEFAULT {
		t.Errorf("server.host = %+v, want value 0.0.0.0 from %q", host, sdkconfig.LAYER_DEFAULT)
	}

	port := byKey["port"]
	if port.Layer != "json" {
		t.Errorf("port layer = %q, want json (file overrides default)", port.Layer)
	}
	if len(port.Shadowed) != 1 || port.Shadowed[0].Layer != sdkconfig.LAYER_DEFAULT {
		t.Errorf("port shadowed = %+v, want the default layer", port.Shadowed)
	}
}
