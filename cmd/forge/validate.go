package main

import (
	"fmt"
	"os"

	"github.com/bjburke002/forge/config/crd"
	"github.com/bjburke002/forge/internal/validate"
	"github.com/spf13/cobra"
)

func newValidateCmd(opts *rootOptions) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "validate",
		Short: "Validate Forge Service CR",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			path := args[0]
			cr, err := os.ReadFile(path)
			if err != nil {
				return fmt.Errorf("reading manifest: %w", err)
			}

			validator, err := validate.New(crd.ForgeService)
			if err != nil {
				return fmt.Errorf("building validator: %w", err)
			}

			errs, err := validator.Validate(cmd.Context(), cr)
			if err != nil {
				return fmt.Errorf("%s: %w", path, err)
			}
			if len(errs) > 0 {
				for _, e := range errs {
					fmt.Fprintf(cmd.ErrOrStderr(), "%s: %v\n", path, e)
				}
				return fmt.Errorf("%s: %d validation error(s)", path, len(errs))

			}

			fmt.Fprintf(cmd.OutOrStdout(), "%s: valid\n", path)
			return nil

		},
	}
	return cmd
}
