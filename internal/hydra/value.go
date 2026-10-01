package hydra

import (
	"encoding/json"
	"fmt"
)

// Int reads a vertex_id, integer or signed_integer cell.
func (v Value) Int() (int64, error) {
	switch v.Type {
	case "vertex_id", "integer", "signed_integer":
		var n int64
		err := json.Unmarshal(v.Value, &n)
		return n, err
	}
	return 0, fmt.Errorf("hydradb: want an integer, got %s", v.Type)
}

// String reads a string cell. Null reads as "".
func (v Value) String() (string, error) {
	switch v.Type {
	case "string":
		var s string
		err := json.Unmarshal(v.Value, &s)
		return s, err
	case "null":
		return "", nil
	}
	return "", fmt.Errorf("hydradb: want a string, got %s", v.Type)
}

// Bool reads a boolean cell. Null reads as false.
func (v Value) Bool() (bool, error) {
	switch v.Type {
	case "boolean":
		var b bool
		err := json.Unmarshal(v.Value, &b)
		return b, err
	case "null":
		return false, nil
	}
	return false, fmt.Errorf("hydradb: want a boolean, got %s", v.Type)
}

// Node is one vertex of a path, with its properties as plain Go values
// (string, int64, float64 or bool).
type Node struct {
	ID     uint64
	Labels []string
	Props  map[string]any
}

// Path reads a path cell, as returned by the algo.*paths procedures.
func (v Value) Path() ([]Node, error) {
	if v.Type != "path" {
		return nil, fmt.Errorf("hydradb: want a path, got %s", v.Type)
	}
	var raw struct {
		Nodes []struct {
			ID         uint64                     `json:"id"`
			Labels     []string                   `json:"labels"`
			Properties map[string]json.RawMessage `json:"properties"`
		} `json:"nodes"`
	}
	if err := json.Unmarshal(v.Value, &raw); err != nil {
		return nil, err
	}
	out := make([]Node, len(raw.Nodes))
	for i, n := range raw.Nodes {
		out[i] = Node{ID: n.ID, Labels: n.Labels, Props: map[string]any{}}
		for k, p := range n.Properties {
			val, err := propValue(p)
			if err != nil {
				return nil, fmt.Errorf("hydradb: property %s: %w", k, err)
			}
			out[i].Props[k] = val
		}
	}
	return out, nil
}

// propValue unwraps a property inside a path, which HydraDB tags with its
// storage type: {"String": "x"}, {"Integer": 1}, {"Float": 1.5} or
// {"Boolean": true}.
func propValue(raw json.RawMessage) (any, error) {
	var tagged map[string]json.RawMessage
	if err := json.Unmarshal(raw, &tagged); err != nil || len(tagged) != 1 {
		return nil, fmt.Errorf("unexpected value %s", raw)
	}
	for tag, val := range tagged {
		switch tag {
		case "String":
			var s string
			return s, json.Unmarshal(val, &s)
		case "Integer", "SignedInteger":
			var n int64
			return n, json.Unmarshal(val, &n)
		case "Float":
			var f float64
			return f, json.Unmarshal(val, &f)
		case "Boolean":
			var b bool
			return b, json.Unmarshal(val, &b)
		}
		return nil, fmt.Errorf("unknown type %s", tag)
	}
	panic("unreachable")
}
