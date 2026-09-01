// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Aidan Reilly and the nasc contributors

package parse

import (
	"encoding/json"
	"fmt"
	"time"

	"github.com/aireilly/nasc-cli/internal/model"
	"gopkg.in/yaml.v3"
)

// coerce turns a decoded YAML value into a model.Value with a resolved Kind.
func coerce(raw any) model.Value {
	switch v := raw.(type) {
	case nil:
		return model.Value{Kind: model.KindNull}
	case bool:
		n := 0.0
		if v {
			n = 1
		}
		return model.Value{Kind: model.KindBool, Num: n}
	case int:
		return model.Value{Kind: model.KindNumber, Num: float64(v)}
	case int64:
		return model.Value{Kind: model.KindNumber, Num: float64(v)}
	case float64:
		return model.Value{Kind: model.KindNumber, Num: v}
	case time.Time:
		return model.Value{Kind: model.KindDate, Str: v.Format("2006-01-02"), Num: float64(v.Unix())}
	case string:
		return model.Value{Kind: model.KindString, Str: v}
	case []any:
		out := make([]string, 0, len(v))
		for _, e := range v {
			out = append(out, coerce(e).String())
		}
		b, _ := json.Marshal(out)
		return model.Value{Kind: model.KindList, Str: string(b)}
	default:
		// Fall back to string form of anything unexpected.
		return model.Value{Kind: model.KindString, Str: fmt.Sprintf("%v", v)}
	}
}

// decodeFields unmarshals frontmatter into ordered fields. yaml.v3 decodes
// unquoted ISO dates as time.Time and quoted ones as string, which gives the
// date-versus-string distinction for free.
func decodeFields(fm []byte) (map[string]model.Value, []string, error) {
	var root yaml.Node
	if err := yaml.Unmarshal(fm, &root); err != nil {
		return nil, nil, err
	}

	fields := make(map[string]model.Value)
	var order []string

	// Unmarshal wraps the actual document in a document node.
	// The actual content is in root.Content[0].
	var mapNode *yaml.Node
	if root.Kind == yaml.DocumentNode && len(root.Content) > 0 {
		mapNode = root.Content[0]
	} else {
		mapNode = &root
	}

	// The node should be a mapping.
	if mapNode.Kind != yaml.MappingNode || len(mapNode.Content) == 0 {
		return fields, order, nil
	}

	// Iterate through key-value pairs in the mapping.
	for i := 0; i < len(mapNode.Content); i += 2 {
		keyNode := mapNode.Content[i]
		valNode := mapNode.Content[i+1]

		key := keyNode.Value
		if key == "" {
			continue
		}

		// Decode the value based on its tag.
		val := decodeValue(valNode)
		fields[key] = val
		order = append(order, key)
	}

	return fields, order, nil
}

// decodeValue converts a yaml.Node to a model.Value based on its tag.
func decodeValue(n *yaml.Node) model.Value {
	if n == nil {
		return model.Value{Kind: model.KindNull}
	}

	switch n.Tag {
	case "!!null":
		return model.Value{Kind: model.KindNull}
	case "!!bool":
		var b bool
		if err := n.Decode(&b); err == nil {
			num := 0.0
			if b {
				num = 1
			}
			return model.Value{Kind: model.KindBool, Num: num}
		}
		return model.Value{Kind: model.KindBool, Num: 0}
	case "!!int":
		var v int64
		_ = n.Decode(&v)
		return model.Value{Kind: model.KindNumber, Num: float64(v)}
	case "!!float":
		var v float64
		_ = n.Decode(&v)
		return model.Value{Kind: model.KindNumber, Num: v}
	case "!!timestamp":
		var v time.Time
		_ = n.Decode(&v)
		return model.Value{Kind: model.KindDate, Str: v.Format("2006-01-02"), Num: float64(v.Unix())}
	case "!!str":
		return model.Value{Kind: model.KindString, Str: n.Value}
	case "!!seq":
		var items []any
		_ = n.Decode(&items)
		out := make([]string, 0, len(items))
		for _, e := range items {
			out = append(out, coerce(e).String())
		}
		b, _ := json.Marshal(out)
		return model.Value{Kind: model.KindList, Str: string(b)}
	default:
		// Implicit types: try to detect based on content.
		var v any
		_ = n.Decode(&v)
		return coerce(v)
	}
}
