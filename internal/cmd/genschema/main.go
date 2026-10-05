package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	v1 "github.com/inf0-dev/alma/api/v1"
	"github.com/invopop/jsonschema"
)

func main() {
	outDir := filepath.Join("docs", "schema")
	if err := os.MkdirAll(outDir, 0o755); err != nil {
		fmt.Fprintf(os.Stderr, "mkdir: %v\n", err)
		os.Exit(1)
	}

	r := new(jsonschema.Reflector)
	if err := r.AddGoComments("github.com/inf0-dev/alma/api/v1", "api/v1"); err != nil {
		fmt.Fprintf(os.Stderr, "AddGoComments: %v\n", err)
		os.Exit(1)
	}

	if err := generate(r, &v1.Document{}, filepath.Join(outDir, "document.schema.json"),
		"Alma Document",
		"Input schema for an alma alignment session. "+
			"Define requirements, items, and design options in YAML or JSON. "+
			"Feed this to `alma serve`, `alma render`, or `alma validate`.",
	); err != nil {
		fmt.Fprintf(os.Stderr, "document schema: %v\n", err)
		os.Exit(1)
	}
	fmt.Println("generated docs/schema/document.schema.json")

	if err := generate(r, &v1.Record{}, filepath.Join(outDir, "record.schema.json"),
		"Alma Record",
		"Output schema for an alma decision record. "+
			"Produced by `alma export` or the server after a session. "+
			"Embeds the full input document so one file is enough to reopen or continue a decision.",
	); err != nil {
		fmt.Fprintf(os.Stderr, "record schema: %v\n", err)
		os.Exit(1)
	}
	fmt.Println("generated docs/schema/record.schema.json")
}

func generate(r *jsonschema.Reflector, v any, path, title, description string) error {
	schema := r.Reflect(v)
	schema.Title = title
	schema.Description = description

	data, err := json.MarshalIndent(schema, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal: %w", err)
	}
	data = append(data, '\n')
	return os.WriteFile(path, data, 0o644)
}
