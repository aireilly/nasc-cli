// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Aidan Reilly and the nasc contributors

package mark

import (
	"github.com/aireilly/nasc-cli/internal/model"
	"github.com/aireilly/nasc-cli/internal/schema"
)

// Result is the set of new field values for one document.
type Result struct {
	Path    string
	Updates map[string]model.Value
	Derived []string
}

func cloneFields(m map[string]model.Value) map[string]model.Value {
	out := make(map[string]model.Value, len(m))
	for k, v := range m {
		out[k] = v
	}
	return out
}

// Plan runs the requested tiers over docs and returns one Result per doc that
// gained a field. Tiers run in order; each fills only absent keys. Later tiers
// see earlier tiers' additions through the merged view.
func Plan(docs []model.Doc, s *schema.Schema, root string, tiers []string) []Result {
	var out []Result
	for _, d := range docs {
		merged := map[string]model.Value{}
		view := d
		view.Fields = cloneFields(d.Fields)
		for _, tier := range tiers {
			var up map[string]model.Value
			switch tier {
			case "file":
				up = FileTier(view)
			case "git":
				up = GitTier(view, root)
			case "llm":
				up = LLMTier(view, s, root)
			}
			for k, v := range up {
				merged[k] = v
				view.Fields[k] = v // so the next tier sees it as present
			}
		}
		if len(merged) == 0 {
			continue
		}
		out = append(out, Result{Path: d.Path, Updates: merged, Derived: keys(merged)})
	}
	return out
}

func keys(m map[string]model.Value) []string {
	var k []string
	for key := range m {
		k = append(k, key)
	}
	return k
}
