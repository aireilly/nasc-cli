// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Aidan Reilly and the nasc contributors

package schema

import (
	"errors"
	"fmt"
	"io/fs"
	"os"

	"gopkg.in/yaml.v3"
)

// DefaultExcludes are files and trees that are almost never documentation an
// agent should navigate by. They apply on top of the config's own exclude list
// unless no_default_excludes is set.
var DefaultExcludes = []string{
	"CHANGELOG.md", "CHANGES.md", "HISTORY.md",
	"LICENSE.md", "NOTICE.md", "CODE_OF_CONDUCT.md",
	"**/testdata/**", "**/third_party/**",
}

// Config is a loaded .nasc/config.yaml. Every field is optional.
type Config struct {
	// Root is the directory to scan, relative to the project root (the
	// directory holding .nasc/, or the git top level). After LoadConfig the
	// caller rewrites it to a path usable from the current directory.
	Root              string   `yaml:"root"`
	Include           []string `yaml:"include"`
	Exclude           []string `yaml:"exclude"`
	NoDefaultExcludes bool     `yaml:"no_default_excludes"`
	Mark              struct {
		LLMCmd          string `yaml:"llm_cmd"`
		LLMExcerptBytes int    `yaml:"llm_excerpt_bytes"`
		LLMPrompt       string `yaml:"llm_prompt"`
		// LLMTimeout bounds one llm call, as a Go duration ("90s", "2m").
		LLMTimeout string `yaml:"llm_timeout"`
		// Jobs is how many documents mark processes at once.
		Jobs int `yaml:"jobs"`
		// OwnerFallback picks what fills owner when CODEOWNERS has no match.
		// Empty leaves owner unset. "blame" uses the git blame majority
		// author's email, which writes a personal address into the file.
		OwnerFallback string `yaml:"owner_fallback"`
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
	c.Mark.LLMTimeout = "2m"
	c.Mark.Jobs = 4
	c.Output.DefaultFormat = "auto"
	return c
}

// Excludes returns the effective exclude globs: the defaults (unless turned
// off) followed by the config's own list.
func (c *Config) Excludes() []string {
	if c.NoDefaultExcludes {
		return c.Exclude
	}
	out := append([]string{}, DefaultExcludes...)
	return append(out, c.Exclude...)
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
	if c.Mark.Jobs <= 0 {
		c.Mark.Jobs = 1
	}
	switch c.Mark.OwnerFallback {
	case "", "blame":
	default:
		return nil, fmt.Errorf("mark.owner_fallback: unknown value %q (want blame or empty)", c.Mark.OwnerFallback)
	}
	return c, nil
}
