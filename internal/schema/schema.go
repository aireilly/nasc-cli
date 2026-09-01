// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Aidan Reilly and the nasc contributors

package schema

import (
	"fmt"
	"os"

	"gopkg.in/yaml.v3"
)

// Field is one declared frontmatter field.
type Field struct {
	Type      string   `yaml:"type"`
	Required  bool     `yaml:"required"`
	Derive    string   `yaml:"derive"`
	Enum      []string `yaml:"enum"`
	MinLength int      `yaml:"min_length"`
	MaxLength int      `yaml:"max_length"`
	Items     string   `yaml:"items"`
}

// Rule is one custom validation rule.
type Rule struct {
	Name     string `yaml:"name"`
	When     string `yaml:"when"`
	Assert   string `yaml:"assert"`
	Severity string `yaml:"severity"`
	Message  string `yaml:"message"`
}

// Schema is a loaded .nasc/schema.yaml.
type Schema struct {
	Version    int
	Fields     map[string]Field
	FieldOrder []string
	Rules      []Rule
}

// Parse decodes schema bytes, preserving field declaration order.
func Parse(data []byte) (*Schema, error) {
	var root yaml.Node
	if err := yaml.Unmarshal(data, &root); err != nil {
		return nil, err
	}
	var raw struct {
		Version int             `yaml:"version"`
		Fields  map[string]Field `yaml:"fields"`
		Rules   []Rule          `yaml:"rules"`
	}
	if err := yaml.Unmarshal(data, &raw); err != nil {
		return nil, err
	}
	s := &Schema{Version: raw.Version, Fields: raw.Fields, Rules: raw.Rules}
	if s.Fields == nil {
		s.Fields = map[string]Field{}
	}
	s.FieldOrder = fieldOrder(&root)
	if err := s.validate(); err != nil {
		return nil, err
	}
	return s, nil
}

// fieldOrder walks the document node to recover the order keys appear under
// the top-level "fields" mapping.
func fieldOrder(root *yaml.Node) []string {
	if len(root.Content) == 0 {
		return nil
	}
	doc := root.Content[0]
	for i := 0; i+1 < len(doc.Content); i += 2 {
		if doc.Content[i].Value == "fields" {
			m := doc.Content[i+1]
			var order []string
			for j := 0; j+1 < len(m.Content); j += 2 {
				order = append(order, m.Content[j].Value)
			}
			return order
		}
	}
	return nil
}

var validTypes = map[string]bool{
	"string": true, "number": true, "bool": true, "date": true, "list": true,
}

func (s *Schema) validate() error {
	for name, f := range s.Fields {
		if !validTypes[f.Type] {
			return fmt.Errorf("field %q has unknown type %q", name, f.Type)
		}
	}
	for _, r := range s.Rules {
		if r.Severity != "error" && r.Severity != "warn" && r.Severity != "" {
			return fmt.Errorf("rule %q has unknown severity %q", r.Name, r.Severity)
		}
	}
	return nil
}

// Load reads and parses a schema file.
func Load(path string) (*Schema, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	return Parse(data)
}
