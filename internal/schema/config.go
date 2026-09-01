// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Aidan Reilly and the nasc contributors

package schema

import (
	"errors"
	"io/fs"
	"os"

	"gopkg.in/yaml.v3"
)

// Config is a loaded .nasc/config.yaml. Every field is optional.
type Config struct {
	Root    string   `yaml:"root"`
	Include []string `yaml:"include"`
	Exclude []string `yaml:"exclude"`
	Mark    struct {
		LLMCmd          string `yaml:"llm_cmd"`
		LLMExcerptBytes int    `yaml:"llm_excerpt_bytes"`
		LLMPrompt       string `yaml:"llm_prompt"`
	} `yaml:"mark"`
	Index struct {
		Template string `yaml:"template"`
		Output   string `yaml:"output"`
	} `yaml:"index"`
	Output struct {
		DefaultFormat string `yaml:"default_format"`
	} `yaml:"output"`
}

// DefaultConfig returns the zero-config defaults.
func DefaultConfig() *Config {
	c := &Config{Root: "."}
	c.Mark.LLMExcerptBytes = 2000
	c.Index.Output = "AGENTS.md"
	c.Output.DefaultFormat = "auto"
	return c
}

// LoadConfig reads config from path, returning defaults when it is absent.
func LoadConfig(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if errors.Is(err, fs.ErrNotExist) {
		return DefaultConfig(), nil
	}
	if err != nil {
		return nil, err
	}
	c := DefaultConfig()
	if err := yaml.Unmarshal(data, c); err != nil {
		return nil, err
	}
	if c.Root == "" {
		c.Root = "."
	}
	if c.Output.DefaultFormat == "" {
		c.Output.DefaultFormat = "auto"
	}
	return c, nil
}
