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
