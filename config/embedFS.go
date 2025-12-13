package config

import (
	"bytes"
	"embed"
	"fmt"
	"log"

	"github.com/spf13/viper"
)

type FSConfig struct {
	fs       embed.FS
	fileName string
}

func NewFSConfig(fs embed.FS, filename string) Config {
	return FSConfig{
		fs:       fs,
		fileName: filename,
	}
}

func (c FSConfig) Load() *viper.Viper {
	r := GetFSReader(c.fs, c.fileName)
	v := viper.New()

	v.SetConfigType(GetFileExtension(c.fileName))

	if err := v.ReadConfig(r); err != nil {
		// 如果找不到配置檔 (FileNotFoundError)，通常是可接受的
		if _, ok := err.(viper.ConfigFileNotFoundError); ok {
			log.Fatalln("Config file not found. Using defaults and env variables.")
		} else { // 如果是其他讀取錯誤，則終止程式
			log.Fatalf("Fatal error reading config file: %s \n", err)
		}
	}

	fmt.Println(v.ConfigFileUsed())
	return v
}

func (c FSConfig) GetConfigName() string {
	return c.fileName
}

func GetFSReader(fs embed.FS, filename string) *bytes.Reader {
	data, err := fs.ReadFile(filename)
	if err != nil {
		fmt.Println(err)
	}
	return bytes.NewReader(data)
}

// Get file extension from string
func GetFileExtension(filename string) string {
	for i := len(filename) - 1; i >= 0; i-- {
		if filename[i] == '.' {
			return filename[i+1:]
		}
	}
	return ""
}
