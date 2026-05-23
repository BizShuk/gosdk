package common

type ServerConfig struct {
	Host string `mapstructure:"host"`
	Port int    `mapstructure:"port"`
}

type DBConfig struct {
	Driver string `mapstructure:"driver"`
	URL    string `mapstructure:"url"` // 這裡通常是完整的 DSN 字串
}

// DBConnConfig 是 DBConfig 的型別別名，用於向下相容。
type DBConnConfig = DBConfig

type ConfigSchema struct {
	Name     string              `mapstructure:"name"`
	Version  string              `mapstructure:"version"`
	LogLevel string              `mapstructure:"log_level"`
	Server   ServerConfig        `mapstructure:"server"`
	DB       map[string]DBConfig `mapstructure:"db"`
}
