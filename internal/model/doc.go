// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Aidan Reilly and the nasc contributors

package model

// Heading is one markdown heading found in a document body.
type Heading struct {
	Level int
	Text  string
}

// Doc is one markdown file held in memory.
type Doc struct {
	Path     string
	Type     string
	Title    string
	Fields   map[string]Value
	Tags     []string
	Headings []Heading
	Paths    []string
	Body     string
	FMStart  int
	FMEnd    int
	CRLF     bool
	Warnings []string
}

// Field looks up a frontmatter value by key.
func (d Doc) Field(name string) (Value, bool) {
	v, ok := d.Fields[name]
	return v, ok
}
