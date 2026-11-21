// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Aidan Reilly and the nasc contributors

package schema

import (
	"fmt"
	"os"
	"sort"

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
	// Key is the frontmatter key this field is stored under, when it differs
	// from the field's name. Use it to keep nasc's fields clear of keys a
	// site generator already owns, such as Docusaurus `id` or Hugo `type`.
	Key string `yaml:"key"`
	// Map translates a directory into a value for `derive: parent-dir`. Keys
	// are a directory name ("guides") or a repo-relative directory path
	// ("deploy/environments"); the deepest matching directory wins.
	Map map[string]string `yaml:"map"`
	// Vocabulary lists the values a list or string field should use.
	// validate warns on anything else, and the llm source is told to pick
	// from it and has other values removed.
	Vocabulary []string `yaml:"vocabulary"`
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
		Version int              `yaml:"version"`
		Fields  map[string]Field `yaml:"fields"`
		Rules   []Rule           `yaml:"rules"`
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

var validDerives = map[string]bool{
	"slug": true, "h1": true, "parent-dir": true, "llm": true,
	"codeowners": true, "git-last-change": true,
}

func (s *Schema) sortedNames() []string {
	names := make([]string, 0, len(s.Fields))
	for n := range s.Fields {
		names = append(names, n)
	}
	sort.Strings(names)
	return names
}

func contains(xs []string, x string) bool {
	for _, e := range xs {
		if e == x {
			return true
		}
	}
	return false
}

var validTypes = map[string]bool{
	"string": true, "number": true, "bool": true, "date": true, "list": true,
}

func (s *Schema) validate() error {
	keys := map[string]string{}
	for _, name := range s.sortedNames() {
		f := s.Fields[name]
		if !validTypes[f.Type] {
			return fmt.Errorf("field %q has unknown type %q", name, f.Type)
		}
		if f.Derive != "" && !validDerives[f.Derive] {
			return fmt.Errorf("field %q has unknown derive %q", name, f.Derive)
		}
		key := s.Key(name)
		if other, dup := keys[key]; dup {
			return fmt.Errorf("fields %q and %q both use frontmatter key %q", other, name, key)
		}
		keys[key] = name
		if len(f.Map) > 0 && f.Derive != "parent-dir" {
			return fmt.Errorf("field %q has a map but derive is %q; map only applies to derive: parent-dir", name, f.Derive)
		}
		if len(f.Enum) > 0 {
			for dir, v := range f.Map {
				if !contains(f.Enum, v) {
					return fmt.Errorf("field %q maps %q to %q, which is not in its enum", name, dir, v)
				}
			}
		}
	}
	for _, r := range s.Rules {
		if r.Severity != "error" && r.Severity != "warn" && r.Severity != "" {
			return fmt.Errorf("rule %q has unknown severity %q", r.Name, r.Severity)
		}
	}
	return nil
}

// Key returns the frontmatter key that stores the named field.
func (s *Schema) Key(name string) string {
	if s == nil {
		return name
	}
	if f, ok := s.Fields[name]; ok && f.Key != "" {
		return f.Key
	}
	return name
}

// Aliases maps each renamed field's name to its frontmatter key, for rule
// expressions that refer to fields by name.
func (s *Schema) Aliases() map[string]string {
	out := map[string]string{}
	if s == nil {
		return out
	}
	for name, f := range s.Fields {
		if f.Key != "" && f.Key != name {
			out[name] = f.Key
		}
	}
	return out
}

// Load reads and parses a schema file.
func Load(path string) (*Schema, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	return Parse(data)
}
