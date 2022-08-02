package config

import (
	"os"
)

var (
	Version string // This can use `go build -ldflags="-X github.com/bizshuk/gin_default/config.Version=0.1.0"` to inject version
	Profile string // dev, qa, cert, prod, default: profile in .env
)

func Default() {
	GetProfile()
	LoadViperConfig()
}

func GetProfile() string {
	if Profile == "" {
		Profile = os.Getenv("profile")
	}
	return Profile
}
