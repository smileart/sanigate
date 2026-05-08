package main

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"

	toml "github.com/pelletier/go-toml/v2"
	openai "github.com/sashabaranov/go-openai"
)

const (
	apiKeyEnvVar = "SNGT_OPENAI_API_KEY"
	modelEnvVar  = "SNGT_MODEL"
	defaultModel = openai.GPT4oMini
)

const defaultConfigBody = `# SaniGate config
# WARNING: this file may contain your OpenAI API key.
# Do NOT commit it to git or sync it to Dropbox/iCloud/etc.
# SaniGate enforces mode 0600 on POSIX systems.

# model = "gpt-4o-mini"
# api_key = "sk-..."  # SNGT_OPENAI_API_KEY env var takes precedence
`

type Config struct {
	Model  string `toml:"model"`
	APIKey string `toml:"api_key"`
}

func configPath() (string, error) {
	dir, err := os.UserConfigDir()
	if err != nil {
		return "", fmt.Errorf("failed to resolve user config directory: %w", err)
	}
	return filepath.Join(dir, "sanigate", "config.toml"), nil
}

// loadConfig reads and parses the config file at path.
// Returns fs.ErrNotExist (wrapped) if the file does not exist; callers
// should use errors.Is(err, fs.ErrNotExist) to detect first-run.
func loadConfig(path string) (Config, error) {
	info, err := os.Stat(path)
	if err != nil {
		return Config{}, err
	}

	if runtime.GOOS != "windows" {
		if perm := info.Mode().Perm(); perm&0o077 != 0 {
			return Config{}, fmt.Errorf(
				"config file %s has unsafe permissions %#o; fix with: chmod 0600 %s",
				path, perm, path,
			)
		}
	}

	data, err := os.ReadFile(path)
	if err != nil {
		return Config{}, fmt.Errorf("failed to read %s: %w", path, err)
	}

	var cfg Config
	if err := toml.Unmarshal(data, &cfg); err != nil {
		return Config{}, fmt.Errorf("failed to parse %s: %w", path, err)
	}
	return cfg, nil
}

func writeDefaultConfig(path string) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return fmt.Errorf("failed to create config dir: %w", err)
	}
	if err := os.WriteFile(path, []byte(defaultConfigBody), 0o600); err != nil {
		return fmt.Errorf("failed to write default config: %w", err)
	}
	return nil
}

func resolveModel(flagVal, envVal, cfgVal string) string {
	if flagVal != "" {
		return flagVal
	}
	if envVal != "" {
		return envVal
	}
	if cfgVal != "" {
		return cfgVal
	}
	return defaultModel
}

func resolveAPIKey(envVal, cfgVal string) string {
	if envVal != "" {
		return envVal
	}
	return cfgVal
}
