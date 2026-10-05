package main

import (
	"context"
	"os"

	"github.com/charmbracelet/fang"
	"github.com/inf0-dev/alma/app/cli/export"
	"github.com/inf0-dev/alma/app/cli/render"
	"github.com/inf0-dev/alma/app/cli/schema"
	"github.com/inf0-dev/alma/app/cli/serve"
	"github.com/inf0-dev/alma/app/cli/validate"
	"github.com/spf13/cobra"
)

var version = "n/a"

func main() {
	cmd := &cobra.Command{
		Use:   "alma",
		Short: "alma — alignment matrix CLI",
	}

	cmd.AddCommand(
		validate.NewValidateCommand(),
		render.NewRenderCommand(),
		export.NewExportCommand(),
		serve.NewServeCommand(),
		schema.NewSchemaCommand(),
	)

	if err := fang.Execute(context.Background(), cmd, fang.WithVersion(version)); err != nil {
		os.Exit(1)
	}
}
