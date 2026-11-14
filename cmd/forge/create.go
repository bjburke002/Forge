package main

import (
	"errors"
	"fmt"
	"maps"
	"os"
	"slices"

	"github.com/bjburke002/forge/config/crd"
	"github.com/bjburke002/forge/internal/gitops"
	"github.com/bjburke002/forge/internal/validate"
	"github.com/spf13/cobra"
)

func newCreateCmd(opts *rootOptions) *cobra.Command {
	var dryRun bool
	cmd := &cobra.Command{
		Use:   "create <file>",
		Short: "Create a new Forge Service in a GitOps repository",
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

			name, files, err := gitops.Files(cr, opts.cfg.Domain)
			if err != nil {
				return fmt.Errorf("creating files: %w", err)
			}

			if dryRun {
				for _, p := range slices.Sorted(maps.Keys(files)) {
					fmt.Fprintf(cmd.OutOrStdout(), "# %s\n%s\n", p, files[p])
				}
				return nil
			}

			if opts.cfg.GitOpsRepo.Owner == "" {
				return errors.New("no GitOps repo configured: set gitopsRepo in the forge config or FORGE_GITOPS_REPO")
			}

			token, err := githubToken(cmd.Context())
			if err != nil {
				return err
			}

			c, err := gitops.NewGitHubClient(token, opts.cfg.GitOpsRepo.Owner, opts.cfg.GitOpsRepo.Name, opts.cfg.GitOpsBranch)
			if err != nil {
				return err
			}

			svcExists, err := c.ServiceExists(cmd.Context(), name)
			if err != nil {
				return err
			}
			if svcExists {
				return fmt.Errorf("service %q already exists", name)
			}

			branch := "forge/create-" + name
			branchExists, err := c.BranchExists(cmd.Context(), branch)
			if err != nil {
				return err
			}
			if branchExists {
				return fmt.Errorf("branch %q already exists", branch)
			}

			title := "forge: add " + name
			body := fmt.Sprintf("Opened by forge create\n\nAdds services/%s/base and dev. Prod is added later with forge promote", name)

			url, err := c.OpenPR(cmd.Context(), branch, title, body, files)
			if err != nil {
				return err
			}
			fmt.Fprintf(cmd.OutOrStdout(), "Pull Request opened: %s\n", url)
			return nil
		},
	}
	cmd.Flags().BoolVar(&dryRun, "dry-run", false, "Print the files that would be committed instead of opening the PR")
	return cmd
}
