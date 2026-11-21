package main

import (
	"context"
	"errors"
	"fmt"
	"io"
	"strconv"
	"strings"
	"text/tabwriter"
	"time"

	awsconfig "github.com/aws/aws-sdk-go-v2/config"
	"github.com/bjburke002/forge/internal/ecsstatus"
	"github.com/bjburke002/forge/internal/status"
	"github.com/spf13/cobra"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/tools/clientcmd"
)

var backends = []string{"ecs", "eks"}

func newStatusCmd(opts *rootOptions) *cobra.Command {
	var target string
	cmd := &cobra.Command{
		Use:   "status <name>",
		Short: "Get status of a ForgeService",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if target != "ecs" && target != "eks" && target != "" {
				return fmt.Errorf("--target must be: eks, ecs or empty, got: %q", target)
			}

			name := args[0]

			var results []status.Result
			unreachable := map[string]error{}

			if target != "ecs" {
				eksResults, err := eksStatus(cmd.Context(), opts, name)
				if err != nil {
					unreachable["eks"] = err
				} else {
					results = append(results, eksResults...)
				}
			}

			if target != "eks" {
				ecsResults, err := ecsStatus(cmd.Context(), opts, name)
				if err != nil {
					unreachable["ecs"] = err
				} else {
					results = append(results, ecsResults...)
				}
			}

			if len(results) == 0 {
				var errs []error
				for _, b := range backends {
					if err, ok := unreachable[b]; ok {
						errs = append(errs, fmt.Errorf("%s: %w", b, err))
					}
				}
				return errors.Join(errs...)
			}

			if target == "" {
				// If no target is provided we scan through results and return results where at least one target is Deployed to
				// Disincluding any service that isn't deployed to both ECS or EKS
				deployedOn := map[string]bool{}

				var filteredResults []status.Result
				for _, res := range results {
					if res.Deployed {
						deployedOn[res.Backend] = true
					}
				}
				if len(deployedOn) == 0 {
					fmt.Fprintf(cmd.OutOrStdout(), "%s is not deployed on any reachable backend\n", name)
					printUnreachable(cmd.ErrOrStderr(), unreachable)
					return nil
				}

				for _, r := range results {
					if deployedOn[r.Backend] {
						filteredResults = append(filteredResults, r)
					}
				}
				results = filteredResults
			}

			w := tabwriter.NewWriter(cmd.OutOrStdout(), 0, 0, 2, ' ', 0)

			header := []string{"BACKEND", "ENV", "SYNC", "HEALTH", "READY", "AVAILABLE", "IMAGE", "URL"}
			fmt.Fprintln(w, strings.Join(header, "\t"))

			for _, res := range results {
				if !res.Deployed {
					row := []string{res.Backend, res.Environment, "not deployed", "-", "-", "-", "-", "-"}
					fmt.Fprintln(w, strings.Join(row, "\t"))
					continue
				}
				row := []string{res.Backend, res.Environment, dash(res.Sync), dash(res.Health), dash(res.Ready),
					strconv.Itoa(int(res.Available)), dash(res.Image), dash(res.URL)}
				fmt.Fprintln(w, strings.Join(row, "\t"))
			}
			w.Flush()

			out := cmd.OutOrStdout()
			printedAny := false
			for _, res := range results {
				if !res.Stale && len(res.Conditions) == 0 {
					continue
				}
				if !printedAny {
					fmt.Fprintln(out)
					printedAny = true
				}
				fmt.Fprintf(out, "%s:\n", res.Backend+"/"+res.Environment)
				if res.Stale {
					fmt.Fprintln(out, "  spec not yet reconciled")
				}
				for _, c := range res.Conditions {
					fmt.Fprintf(out, "  %s\n", c)
				}
			}
			printUnreachable(cmd.ErrOrStderr(), unreachable)
			return nil
		},
	}
	cmd.Flags().StringVar(&target, "target", "", "Set the target platform for the status request. Limit to one backend: eks or ecs (default both)")
	return cmd
}

func ecsStatus(ctx context.Context, opts *rootOptions, name string) ([]status.Result, error) {
	if opts.cfg.AWSRegion == "" {
		return nil, fmt.Errorf("no AWS region configured: check config.yaml, awsRegion or AWS_REGION")
	}

	ctx, cancel := context.WithTimeout(ctx, 15*time.Second)
	defer cancel()
	awsCfg, err := awsconfig.LoadDefaultConfig(ctx, awsconfig.WithRegion(opts.cfg.AWSRegion))
	if err != nil {
		return nil, fmt.Errorf("loading AWS config: %w", err)
	}

	r, err := ecsstatus.Get(ctx, awsCfg, name)
	if err != nil {
		return nil, fmt.Errorf("querying ECS: %w", err)
	}
	return r, nil
}

func eksStatus(ctx context.Context, opts *rootOptions, name string) ([]status.Result, error) {
	loadingRules := clientcmd.NewDefaultClientConfigLoadingRules()                      // Reads KUBECONFIG or ~/.kube/config
	configOverrides := &clientcmd.ConfigOverrides{CurrentContext: opts.cfg.KubeContext} // Empty here means current context
	kubeConfig := clientcmd.NewNonInteractiveDeferredLoadingClientConfig(loadingRules, configOverrides)

	kubeCtx := opts.cfg.KubeContext
	if kubeCtx == "" {
		raw, err := kubeConfig.RawConfig()
		if err != nil {
			return nil, fmt.Errorf("reading kubeconfig: %w", err)
		}
		kubeCtx = raw.CurrentContext
	}

	restConfig, err := kubeConfig.ClientConfig()
	if err != nil {
		return nil, fmt.Errorf("loading kubeconfig, is it empty or misspelled?: %w", err)
	}
	restConfig.Timeout = 10 * time.Second

	client, err := dynamic.NewForConfig(restConfig)
	if err != nil {
		return nil, fmt.Errorf("loading kube client: %w", err)
	}

	s, err := status.Get(ctx, client, opts.cfg.ArgoNamespace, name)
	if err != nil {
		return nil, fmt.Errorf("querying cluster (context %q): %w", kubeCtx, err)
	}

	return s, nil
}

func printUnreachable(w io.Writer, unreachable map[string]error) {
	for _, b := range backends {
		if err, ok := unreachable[b]; ok {
			fmt.Fprintf(w, "%s: unreachable: %v\n", b, err)
		}
	}
}
func dash(s string) string {
	if s == "" {
		return "-"
	}
	return s
}
