package main

import (
	"fmt"
	"io"
	"os"

	"github.com/bjburke002/forge/config/crd"
	"github.com/bjburke002/forge/internal/render/ecs"
	"github.com/bjburke002/forge/internal/validate"
	"github.com/bjburke002/forge/pkg/contract/v1alpha1"
	"github.com/spf13/cobra"
	"sigs.k8s.io/yaml"
)

func newRenderCmd(opts *rootOptions) *cobra.Command {
	var env string
	cmd := &cobra.Command{
		Use:   "render <file> --env dev|prod",
		Short: "Render ECS Template for the selected environment",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			path := args[0]
			var file []byte
			var err error
			if path == "-" {
				file, err = io.ReadAll(cmd.InOrStdin())
				if err != nil {
					return fmt.Errorf("reading file at stdin: %w", err)
				}
			} else {
				file, err = os.ReadFile(path)
				if err != nil {
					return fmt.Errorf("reading file at %q: %w", path, err)
				}
			}

			validator, err := validate.New(crd.ForgeService)
			if err != nil {
				return fmt.Errorf("building validator: %w", err)
			}

			errs, err := validator.Validate(cmd.Context(), file)
			if err != nil {
				return fmt.Errorf("%s: %w", path, err)
			}
			if len(errs) > 0 {
				for _, e := range errs {
					fmt.Fprintf(cmd.ErrOrStderr(), "%s: %v\n", path, e)
				}
				return fmt.Errorf("%s: %d validation error(s)", path, len(errs))
			}

			var fsvc v1alpha1.ForgeService
			if err = yaml.Unmarshal(file, &fsvc); err != nil {
				return fmt.Errorf("%s: %w", file, err)
			}

			tmpl, err := ecs.Render(&fsvc, env)
			if err != nil {
				return fmt.Errorf("rendering ECS template: %w", err)
			}

			template, err := tmpl.JSON()
			if err != nil {
				return fmt.Errorf("converting template to JSON: %w", err)
			}

			fmt.Fprintf(cmd.OutOrStdout(), "%s", template)
			return nil
		},
	}
	cmd.Flags().StringVar(&env, "env", "", "Set the target environment for the ECS Service: dev or prod")
	cmd.MarkFlagRequired("env")
	return cmd
}
