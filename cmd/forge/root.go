package main

import (
	"fmt"

	"github.com/bjburke002/forge/internal/config"
	"github.com/spf13/cobra"
)

type rootOptions struct {
	configPath  string // --config
	kubeContext string // --context
	cfg         config.Config
}

func newRootCmd() *cobra.Command {
	opts := &rootOptions{}

	cmd := &cobra.Command{
		Use:          "forge",
		Short:        "Register, inspect and promot Forge services",
		SilenceUsage: true,
		PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
			cfg, err := config.Load(opts.configPath)
			if err != nil {
				return fmt.Errorf("loading config: %w", err)
			}

			if cmd.Flags().Changed("context") {
				cfg.KubeContext = opts.kubeContext
			}

			opts.cfg = cfg
			return nil
		},
	}

	cmd.PersistentFlags().StringVar(&opts.configPath, "config", "", "path to the Forge config file")
	cmd.PersistentFlags().StringVar(&opts.kubeContext, "context", "", "kube context to use")
	cmd.AddCommand(newConfigCmd(opts))

	return cmd
}
