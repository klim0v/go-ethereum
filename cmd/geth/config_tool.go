package main

import (
	"bytes"
	_ "embed"
	"fmt"
)

//go:embed default/config-tool.toml
var defaultConfig []byte

const defaultConfigName = "default"

func loadDefaultConfig(cfg *gethConfig) error {
	err := tomlSettings.NewDecoder(bytes.NewReader(defaultConfig)).Decode(cfg)
	if err != nil {
		return fmt.Errorf("failed to load default config, err: %w", err)
	}
	return nil
}
