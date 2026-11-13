package main

import (
	"fmt"

	"github.com/spf13/cobra"
	"sigs.k8s.io/yaml"
)

func newConfigCmd(opts *rootOptions) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "config",
		Short: "Inspect the Forge CLI configuration",
	}
	cmd.AddCommand(newConfigViewCmd(opts))
	return cmd
}

func newConfigViewCmd(opts *rootOptions) *cobra.Command {
	return &cobra.Command{
		Use:   "view",
		Short: "Print the resolved configuration",
		RunE: func(cmd *cobra.Command, args []string) error {
			data, err := yaml.Marshal(opts.cfg)
			if err != nil {
				return fmt.Errorf("marshaling config YAML: %w", err)
			}
			_, err = cmd.OutOrStdout().Write(data)
			return err
		},
		Args: cobra.NoArgs,
	}
}
