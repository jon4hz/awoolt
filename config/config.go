// Package config provides functionality to load configuration settings for awoolt.
package config

import (
	"errors"
	"os"
	"path"

	"github.com/adrg/xdg"
	"github.com/spf13/viper"
)

// Config holds the configuration options for awoolt.
type Config struct {
	Engine string `mapstructure:"engine"`
}

// Load loads the configuration from the specified path or from default locations.
func Load(path string) (cfg *Config, err error) {
	if path != "" {
		return load(path)
	}
	for _, f := range [...]string{
		".config.yml",
		"config.yml",
		".config.yaml",
		"config.yaml",
		"awoolt.yml",
		"awoolt.yaml",
	} {
		cfg, err = load(f)
		if err != nil && os.IsNotExist(err) {
			err = nil
			continue
		} else if err != nil && errors.As(err, &viper.ConfigFileNotFoundError{}) {
			err = nil
			continue
		}
	}
	if cfg == nil {
		return cfg, viper.Unmarshal(&cfg) //nolint:wrapcheck
	}
	return
}

func load(file string) (cfg *Config, err error) {
	viper.SetConfigName(file)
	viper.SetConfigType("yaml")
	viper.AddConfigPath("./")
	viper.AddConfigPath(path.Join(xdg.ConfigHome, "awoolt"))
	viper.AddConfigPath("/etc/awoolt/")
	if err = viper.ReadInConfig(); err != nil {
		return
	}
	if err = viper.Unmarshal(&cfg); err != nil {
		return
	}
	return
}
