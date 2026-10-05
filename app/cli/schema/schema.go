package schema

import (
	"encoding/json"
	"fmt"

	v1 "github.com/inf0-dev/alma/api/v1"
	"github.com/invopop/jsonschema"
	"github.com/spf13/cobra"
)

var schemas = map[string]struct {
	target      any
	title       string
	description string
}{
	"document": {
		target:      &v1.Document{},
		title:       "Alma Document",
		description: "Input schema for an alma alignment session. Define requirements, items, and design options in YAML or JSON. Feed this to `alma serve`, `alma render`, or `alma validate`.",
	},
	"record": {
		target:      &v1.Record{},
		title:       "Alma Record",
		description: "Output schema for an alma decision record. Produced by `alma export` or the server after a session. Embeds the full input document so one file is enough to reopen or continue a decision.",
	},
}

func NewSchemaCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:       "schema [document|record]",
		Short:     "Print JSON Schema for alma types",
		Long:      "Print the JSON Schema for an alma document (input) or record (output) to stdout.",
		Args:      cobra.ExactArgs(1),
		ValidArgs: []string{"document", "record"},
		RunE: func(cmd *cobra.Command, args []string) error {
			entry, ok := schemas[args[0]]
			if !ok {
				return fmt.Errorf("unknown schema type: %s (use 'document' or 'record')", args[0])
			}

			r := new(jsonschema.Reflector)
			s := r.Reflect(entry.target)
			s.Title = entry.title
			s.Description = entry.description

			data, err := json.MarshalIndent(s, "", "  ")
			if err != nil {
				return fmt.Errorf("marshal: %w", err)
			}

			cmd.Println(string(data))
			return nil
		},
	}

	return cmd
}
