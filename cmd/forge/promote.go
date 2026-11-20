package main

import (
	"errors"
	"fmt"
	"maps"
	"slices"

	"github.com/bjburke002/forge/internal/gitops"
	"github.com/spf13/cobra"
)

func newPromoteCmd(opts *rootOptions) *cobra.Command {
	var dryRun bool
	var target string
	cmd := &cobra.Command{
		Use:   "promote <name>",
		Short: "Promote ForgeService from dev to prod",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			name := args[0]

			if target != "" && target != gitops.TargetECS && target != gitops.TargetEKS {
				return fmt.Errorf("target must either be empty, eks, or ecs: got %q", target)
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

			inferredTarget, svcExists, err := c.InferTarget(cmd.Context(), name)
			if err != nil {
				return err
			}
			if !svcExists {
				return fmt.Errorf("service %s does not exist in forge-gitops", name)
			}
			if target != "" && target != inferredTarget {
				return fmt.Errorf("target %q doesn't match inferred target %q", target, inferredTarget)
			}

			devPath := gitops.ServicePath(inferredTarget, name, "dev", "kustomization.yaml")
			d, found, err := c.ReadFile(cmd.Context(), devPath)
			if err != nil {
				return fmt.Errorf("reading dev overlay: %w", err)
			}
			if !found {
				return fmt.Errorf("dev overlay not found at %q", devPath)
			}

			prodPath := gitops.ServicePath(inferredTarget, name, "prod", "kustomization.yaml")
			p, found, err := c.ReadFile(cmd.Context(), prodPath)
			var body string
			if err != nil {
				return fmt.Errorf("reading prod overlay: %w", err)
			}
			if !found {
				p = nil
				body = "Adds " + inferredTarget + "/services/" + name + "/prod"
			}

			tag, files, err := gitops.PromoteFiles(d, p, name, inferredTarget)
			if err != nil {
				return fmt.Errorf("checking overlays: %w", err)
			}
			if len(files) == 0 {
				return fmt.Errorf("prod overlay is already on tag %q", tag)

			}

			if dryRun {
				for _, p := range slices.Sorted(maps.Keys(files)) {
					fmt.Fprintf(cmd.OutOrStdout(), "# %s\n%s\n", p, files[p])
				}
				return nil
			}

			branch := "forge/promote-" + name
			branchExists, err := c.BranchExists(cmd.Context(), branch)
			if err != nil {
				return err
			}
			if branchExists {
				return fmt.Errorf("branch %q already exists", branch)
			}

			title := "forge: promote " + name + " to prod (" + tag + ")"
			url, err := c.OpenPR(cmd.Context(), branch, title, body, files)
			if err != nil {
				return err
			}
			fmt.Fprintf(cmd.OutOrStdout(), "Pull Request opened: %s\n", url)
			return nil

		},
	}
	cmd.Flags().BoolVar(&dryRun, "dry-run", false, "Print the files that would be committed instead of opening the PR")
	cmd.Flags().StringVar(&target, "target", "", "Set the target platform for this promotion: EKS or ECS")
	return cmd
}
