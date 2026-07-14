package main

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"time"
)

// Model capability metadata comes from models.dev, a community-maintained registry
// of LLMs. We use it only as a preflight courtesy: it lets us warn before spending
// a token when the configured model can't do what this tool needs. It is never
// authoritative — the OpenAI API is. A stale, missing, or unreachable registry, or
// a model the registry has never heard of, always means "proceed and let the API
// decide", never "refuse to run".
const (
	modelsDevURL       = "https://models.dev/api.json"
	modelsCacheTTL     = 7 * 24 * time.Hour
	modelsFetchTimeout = 5 * time.Second
)

// modelsDevProvider and modelsDevModel are the slice of the models.dev schema we
// read. The registry carries far more per model; unlisted fields are ignored.
type modelsDevProvider struct {
	Models map[string]modelsDevModel `json:"models"`
}

type modelsDevModel struct {
	StructuredOutput bool `json:"structured_output"`
	Temperature      bool `json:"temperature"`
}

// checkModelCapabilities warns, to the given logger, when the configured model is
// known to lack a capability this tool depends on. It never fails and never blocks:
// see the note on modelsDevURL.
func checkModelCapabilities(errLog *log.Logger, model string) {
	reg, err := loadModelsRegistry()
	if err != nil || reg == nil {
		return // no registry data — say nothing, let the API be the judge
	}

	m, found := lookupModel(reg, model)
	if !found {
		return // unknown model — may simply be newer than the registry; proceed
	}

	if !m.StructuredOutput {
		errLog.Printf(
			"sanigate: model %q is not known to support structured outputs — the analysis request may fail. Try gpt-4o-mini or newer.",
			model,
		)
	}
	// This tool sends temperature=0; models that reject the parameter (the o-series)
	// will error on it.
	if !m.Temperature {
		errLog.Printf(
			"sanigate: model %q is not known to accept a temperature parameter — the analysis request may fail.",
			model,
		)
	}
}

// lookupModel finds a model by id, preferring the openai provider and then falling
// back to any provider that lists it.
func lookupModel(reg map[string]modelsDevProvider, id string) (modelsDevModel, bool) {
	if p, ok := reg["openai"]; ok {
		if m, ok := p.Models[id]; ok {
			return m, true
		}
	}
	for _, p := range reg {
		if m, ok := p.Models[id]; ok {
			return m, true
		}
	}
	return modelsDevModel{}, false
}

// loadModelsRegistry returns the parsed registry, using a cached copy when it is
// fresh, fetching over the network when it is stale or absent, and falling back to
// a stale cache when the network is unavailable. Returns nil, nil when there is no
// data to be had — callers must treat that as "no opinion", not an error worth
// surfacing.
func loadModelsRegistry() (map[string]modelsDevProvider, error) {
	path, err := modelsCachePath()
	if err != nil {
		return nil, err
	}

	if data, err := readFreshCache(path); err == nil {
		return parseModelsRegistry(data)
	}

	if data, err := fetchModelsDev(); err == nil {
		writeModelsCache(path, data) // best-effort; a failed write only costs a refetch
		return parseModelsRegistry(data)
	}

	// Network failed — a stale cache still beats no data.
	if stale, err := os.ReadFile(path); err == nil {
		return parseModelsRegistry(stale)
	}

	return nil, nil
}

func modelsCachePath() (string, error) {
	dir, err := os.UserCacheDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "sanigate", "models.json"), nil
}

// readFreshCache returns the cached bytes only if the file exists and is younger
// than modelsCacheTTL; otherwise it returns an error so the caller refetches.
func readFreshCache(path string) ([]byte, error) {
	info, err := os.Stat(path)
	if err != nil {
		return nil, err
	}
	if time.Since(info.ModTime()) > modelsCacheTTL {
		return nil, fmt.Errorf("models cache is stale")
	}
	return os.ReadFile(path)
}

func fetchModelsDev() ([]byte, error) {
	client := &http.Client{Timeout: modelsFetchTimeout}
	resp, err := client.Get(modelsDevURL)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("models.dev returned %s", resp.Status)
	}
	return io.ReadAll(resp.Body)
}

func writeModelsCache(path string, data []byte) {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return
	}
	_ = os.WriteFile(path, data, 0o644)
}

func parseModelsRegistry(data []byte) (map[string]modelsDevProvider, error) {
	var reg map[string]modelsDevProvider
	if err := json.Unmarshal(data, &reg); err != nil {
		return nil, err
	}
	return reg, nil
}
