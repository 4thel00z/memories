package main

import (
	"encoding/json"
	"io"
)

// printJSON writes v as indented JSON — the one output format every --json
// command shares.
func printJSON(w io.Writer, v any) error {
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	return enc.Encode(v)
}
