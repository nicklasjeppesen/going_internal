package validation

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
)

// RuleConfig definerer strukturen for rules.json
type RuleConfig struct {
	DefaultMessages map[string]string            `json:"default_messages"`
	FieldMessages   map[string]map[string]string `json:"field_messages"`
}

var (
	config     *RuleConfig
	configOnce sync.Once
)

// Load finder og indlæser rules.json fra projektet (singleton)
func Load() (*RuleConfig, error) {
	var loadErr error

	configOnce.Do(func() {
		path, err := findRulesFile()
		if err != nil {
			loadErr = fmt.Errorf("rules.json ikke fundet: %w", err)
			return
		}

		data, err := os.ReadFile(path)
		if err != nil {
			loadErr = fmt.Errorf("kunne ikke læse rules.json: %w", err)
			return
		}

		config = &RuleConfig{}
		if err := json.Unmarshal(data, config); err != nil {
			loadErr = fmt.Errorf("ugyldig rules.json: %w", err)
			config = nil
			return
		}
	})

	if loadErr != nil {
		return nil, loadErr
	}
	return config, nil
}

func findRulesFile() (string, error) {
	const expectedPath = "internal/resources/rules/rules.json"

	dir, err := os.Getwd()
	if err != nil {
		return "", fmt.Errorf("kunne ikke hente working directory: %w", err)
	}

	candidate := filepath.Join(dir, expectedPath)
	if _, err := os.Stat(candidate); err == nil {
		return candidate, nil
	}

	return "", fmt.Errorf("rules.json ikke fundet - forventet sti: '%s'", candidate)
}

// GetDefaultMessage returnerer default besked for et givent tag
func GetDefaultMessage(tag string) (string, bool) {
	cfg, err := Load()
	if err != nil {
		return "", false
	}
	msg, ok := cfg.DefaultMessages[tag]
	return msg, ok
}

// GetFieldMessage returnerer specifik besked for struct.felt.regel
func GetFieldMessage(structName, field, tag string) (string, bool) {
	cfg, err := Load()
	if err != nil {
		return "", false
	}
	if structFields, ok := cfg.FieldMessages[structName]; ok {
		key := field + "." + tag
		if msg, ok := structFields[key]; ok {
			return msg, true
		}
	}
	return "", false
}
