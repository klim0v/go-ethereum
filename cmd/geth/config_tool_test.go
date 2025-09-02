package main

import (
	_ "embed"
	"testing"
)

func TestLoadConfig_default(t *testing.T) {
	err := loadConfig(defaultConfigName, &gethConfig{})
	if err != nil {
		t.Error(err)
	}
}
