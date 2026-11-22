// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Aidan Reilly and the nasc contributors

package edit

import (
	"bytes"
	"sort"
	"strings"

	"github.com/aireilly/nasc-cli/internal/model"
	"github.com/aireilly/nasc-cli/internal/parse"
	"gopkg.in/yaml.v3"
)

// Set applies updates to a document's frontmatter, preserving key order,
// comments, quoting, and line endings, and never touching the body.
func Set(original []byte, updates map[string]model.Value) ([]byte, error) {
	// TOP GUARD: no updates means no edit. This is the only no-op condition;
	// nothing else short-circuits before the real edit path runs.
	if len(updates) == 0 {
		return original, nil // no-op keeps bytes identical for every input
	}

	// Locate frontmatter directly in the ORIGINAL bytes (never a
	// line-ending-normalized copy), so bodyStart is a byte offset into
	// original and the body can be spliced out verbatim below. A leading
	// UTF-8 BOM is stripped internally by SplitFrontmatter's fence search but
	// is not re-emitted here on a real edit; that is intended, not a bug.
	fm, bodyStart, fmStart, _, ok, _ := parse.SplitFrontmatter(original)

	// Only the frontmatter region's own line endings decide whether the
	// freshly-encoded frontmatter gets CRLF applied. The body is never
	// inspected or rewritten for line endings.
	var fmCRLF bool
	if ok {
		fmCRLF = bytes.Contains(original[fmStart:bodyStart], []byte("\r\n"))
	}

	var doc yaml.Node
	if ok {
		// Normalize only the frontmatter bytes for YAML parsing; the body is
		// never touched by this normalization.
		fmLF := bytes.ReplaceAll(fm, []byte("\r\n"), []byte("\n"))
		if err := yaml.Unmarshal(fmLF, &doc); err != nil {
			return original, err // never edit unparseable frontmatter
		}
	}
	if ok {
		if spliced, sok := splice(fmLFOf(fm), &doc, updates); sok {
			contentStart := fmStart + 4
			if bytes.HasPrefix(original[fmStart:], []byte("---\r\n")) {
				contentStart = fmStart + 5
			}
			if fmCRLF {
				spliced = bytes.ReplaceAll(spliced, []byte("\n"), []byte("\r\n"))
			}
			var out bytes.Buffer
			out.Write(original[:contentStart])
			out.Write(spliced)
			out.Write(original[contentStart+len(fm):])
			return out.Bytes(), nil
		}
	}
	mapping := rootMapping(&doc)

	for _, key := range sortedKeys(updates) {
		setKey(mapping, key, updates[key])
	}
	// Drop the deprecated provenance key so re-marking a file cleans it up.
	removeKey(mapping, "x-nasc-generated")

	var buf bytes.Buffer
	enc := yaml.NewEncoder(&buf)
	enc.SetIndent(2) // default is 4 and would reformat every touched file
	if err := enc.Encode(&doc); err != nil {
		return original, err
	}
	if err := enc.Close(); err != nil {
		return original, err
	}

	fmBlock := "---\n" + buf.String() + "---\n"
	if fmCRLF {
		fmBlock = strings.ReplaceAll(fmBlock, "\n", "\r\n")
	}

	var out bytes.Buffer
	out.WriteString(fmBlock)
	if ok {
		// Splice the body from the ORIGINAL bytes, byte-for-byte. It is
		// never normalized and never has its line endings rewritten.
		out.Write(original[bodyStart:])
	} else {
		// No frontmatter existed; the entire original document becomes the
		// body of the newly inserted block, verbatim.
		out.Write(original)
	}

	return out.Bytes(), nil
}

// rootMapping returns the top-level mapping node, creating a document and
// mapping when the node is empty (no frontmatter present).
func rootMapping(doc *yaml.Node) *yaml.Node {
	if len(doc.Content) == 0 {
		m := &yaml.Node{Kind: yaml.MappingNode, Tag: "!!map"}
		doc.Kind = yaml.DocumentNode
		doc.Content = []*yaml.Node{m}
		return m
	}
	return doc.Content[0]
}

func setKey(m *yaml.Node, key string, v model.Value) {
	valNode := valueNode(v)
	for i := 0; i+1 < len(m.Content); i += 2 {
		if m.Content[i].Value == key {
			m.Content[i+1] = valNode
			return
		}
	}
	keyNode := &yaml.Node{Kind: yaml.ScalarNode, Tag: "!!str", Value: key}
	m.Content = append(m.Content, keyNode, valNode)
}

func valueNode(v model.Value) *yaml.Node {
	switch v.Kind {
	case model.KindList:
		n := &yaml.Node{Kind: yaml.SequenceNode, Tag: "!!seq"}
		for _, e := range v.List() {
			n.Content = append(n.Content, &yaml.Node{Kind: yaml.ScalarNode, Tag: "!!str", Value: e})
		}
		return n
	case model.KindDate:
		// Tag as a timestamp, not a string, so the encoder emits the ISO date
		// unquoted. A quoted value re-parses as !!str (KindString) and fails
		// validate's date type check.
		return &yaml.Node{Kind: yaml.ScalarNode, Tag: "!!timestamp", Value: v.Str}
	default:
		return &yaml.Node{Kind: yaml.ScalarNode, Tag: "!!str", Value: v.String()}
	}
}

// removeKey deletes key from the mapping if present, preserving the order of
// the remaining keys.
func removeKey(m *yaml.Node, key string) {
	for i := 0; i+1 < len(m.Content); i += 2 {
		if m.Content[i].Value == key {
			m.Content = append(m.Content[:i], m.Content[i+2:]...)
			return
		}
	}
}

func sortedKeys(m map[string]model.Value) []string {
	var keys []string
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}

func fmLFOf(fm []byte) []byte {
	return bytes.ReplaceAll(fm, []byte("\r\n"), []byte("\n"))
}

// splice applies updates by rewriting only the lines of the keys it sets,
// and of the deprecated provenance key it removes. Every other line of the
// frontmatter, including its formatting, quoting, and comments, is kept byte
// for byte. It reports false when the frontmatter is not a plain block
// mapping it can edit line by line, and the caller then re-encodes the whole
// block.
func splice(fmLF []byte, doc *yaml.Node, updates map[string]model.Value) ([]byte, bool) {
	if doc.Kind != yaml.DocumentNode || len(doc.Content) != 1 {
		return nil, false
	}
	m := doc.Content[0]
	if m.Kind != yaml.MappingNode || m.Style&yaml.FlowStyle != 0 || usesAnchors(m) {
		return nil, false
	}
	lines := strings.SplitAfter(string(fmLF), "\n")
	if lines[len(lines)-1] == "" {
		lines = lines[:len(lines)-1]
	}

	// replace maps a key's first line (0-based) to its span end and new text.
	type edit struct {
		end  int
		text string
	}
	edits := map[int]edit{}
	present := map[string]bool{}
	for i := 0; i+1 < len(m.Content); i += 2 {
		k := m.Content[i]
		if k.Kind != yaml.ScalarNode || k.Column != 1 || k.Line < 1 || k.Line > len(lines) {
			return nil, false
		}
		present[k.Value] = true
		v, set := updates[k.Value]
		if !set && k.Value != "x-nasc-generated" {
			continue
		}
		start := k.Line - 1
		end := spanEnd(lines, start)
		text := ""
		if set {
			enc, err := encodePair(k, valueNode(v))
			if err != nil {
				return nil, false
			}
			text = enc
		}
		edits[start] = edit{end: end, text: text}
	}

	var out strings.Builder
	for i := 0; i < len(lines); i++ {
		if e, hit := edits[i]; hit {
			out.WriteString(e.text)
			i = e.end
			continue
		}
		out.WriteString(lines[i])
	}
	if out.Len() > 0 && !strings.HasSuffix(out.String(), "\n") {
		out.WriteString("\n")
	}
	for _, key := range sortedKeys(updates) {
		if present[key] {
			continue
		}
		enc, err := encodePair(&yaml.Node{Kind: yaml.ScalarNode, Tag: "!!str", Value: key}, valueNode(updates[key]))
		if err != nil {
			return nil, false
		}
		out.WriteString(enc)
	}
	return []byte(out.String()), true
}

// spanEnd returns the last line of the top-level entry that starts on line
// start: the lines after it that are indented, or that continue a block
// sequence written at the key's own indent. Trailing blank lines are left
// for the next entry.
func spanEnd(lines []string, start int) int {
	end := start
	for i := start + 1; i < len(lines); i++ {
		l := strings.TrimRight(lines[i], "\n")
		switch {
		case strings.TrimSpace(l) == "":
			continue
		case l[0] == ' ' || l[0] == '\t' || l == "-" || strings.HasPrefix(l, "- "):
			end = i
		default:
			return end
		}
	}
	return end
}

// encodePair renders one top-level entry the way the full encoder would.
func encodePair(k, v *yaml.Node) (string, error) {
	key := *k
	key.HeadComment, key.LineComment, key.FootComment = "", "", ""
	m := &yaml.Node{Kind: yaml.MappingNode, Tag: "!!map", Content: []*yaml.Node{&key, v}}
	var buf bytes.Buffer
	enc := yaml.NewEncoder(&buf)
	enc.SetIndent(2)
	if err := enc.Encode(m); err != nil {
		return "", err
	}
	if err := enc.Close(); err != nil {
		return "", err
	}
	return buf.String(), nil
}

// usesAnchors reports whether any node under n has an anchor or is an alias.
// Rewriting one entry could then break a reference, so splice declines.
func usesAnchors(n *yaml.Node) bool {
	if n.Anchor != "" || n.Kind == yaml.AliasNode {
		return true
	}
	for _, c := range n.Content {
		if usesAnchors(c) {
			return true
		}
	}
	return false
}
