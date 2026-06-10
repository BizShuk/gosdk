package metric

import "github.com/spf13/viper"

// mimirDefaultURL is Mimir's remote-write endpoint; the path (/api/v1/push)
// is the only difference from other Prometheus remote-write backends.
const mimirDefaultURL = "http://localhost:9009/api/v1/push"

// MimirService is kept for backward compatibility. Mimir is just one of the
// Prometheus remote-write backends supported by RemoteWriteService.
//
// Deprecated: use RemoteWriteService instead.
type MimirService = RemoteWriteService

// NewMimirService creates a RemoteWriteService targeting Mimir, honoring the
// MIMIR_URL config if set.
//
// Deprecated: use NewRemoteWriteService instead.
func NewMimirService() *MimirService {
	url := viper.GetString("MIMIR_URL")
	if url == "" {
		url = mimirDefaultURL
	}
	return NewRemoteWriteService(url)
}
