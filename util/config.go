package util

import (
	"encoding/json"
	"fmt"
	"os"
)

func LoadConfig(cfgPath string, cfg interface{}) error {
	cfgFile, err := os.Open(cfgPath)
	if err != nil {
		return fmt.Errorf("error opening %s: %w", cfgPath, err)
	}
	defer cfgFile.Close()

	decoder := json.NewDecoder(cfgFile)
	return decoder.Decode(cfg)
}
