package main

import (
	"fmt"
	"strconv"
	"strings"
	"text/tabwriter"

	"github.com/bjburke002/forge/internal/status"
	"github.com/spf13/cobra"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/tools/clientcmd"
)

func newStatusCmd(opts *rootOptions) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "status <name>",
		Short: "Get status of a ForgeService",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			name := args[0]
			loadingRules := clientcmd.NewDefaultClientConfigLoadingRules()                      // Reads KUBECONFIG or ~/.kube/config
			configOverrides := &clientcmd.ConfigOverrides{CurrentContext: opts.cfg.KubeContext} // Empty here means current context
			kubeConfig := clientcmd.NewNonInteractiveDeferredLoadingClientConfig(loadingRules, configOverrides)

			kubeCtx := opts.cfg.KubeContext
			if kubeCtx == "" {
				raw, err := kubeConfig.RawConfig()
				if err != nil {
					return fmt.Errorf("reading kubeconfig: %w", err)
				}
				kubeCtx = raw.CurrentContext
			}

			restConfig, err := kubeConfig.ClientConfig()
			if err != nil {
				return fmt.Errorf("loading kubeconfig, is it empty or misspelled?: %w", err)
			}

			client, err := dynamic.NewForConfig(restConfig)
			if err != nil {
				return fmt.Errorf("loading kube client: %w", err)
			}

			s, err := status.Get(cmd.Context(), client, opts.cfg.ArgoNamespace, name)
			if err != nil {
				return fmt.Errorf("querying cluster (context %q): %w", kubeCtx, err)
			}

			w := tabwriter.NewWriter(cmd.OutOrStdout(), 0, 0, 2, ' ', 0)

			header := []string{"ENV", "SYNC", "HEALTH", "READY", "AVAILABLE", "IMAGE", "URL"}
			fmt.Fprintln(w, strings.Join(header, "\t"))

			for _, res := range s {
				if !res.Deployed {
					row := []string{res.Environment, "not deployed", "-", "-", "-", "-", "-"}
					fmt.Fprintln(w, strings.Join(row, "\t"))
					continue
				}
				row := []string{res.Environment, dash(res.Sync), dash(res.Health), dash(res.Ready),
					strconv.Itoa(int(res.Available)), dash(res.Image), dash(res.URL)}
				fmt.Fprintln(w, strings.Join(row, "\t"))
			}
			w.Flush()

			out := cmd.OutOrStdout()
			printedAny := false
			for _, res := range s {
				if !res.Stale && len(res.Conditions) == 0 {
					continue
				}
				if !printedAny {
					fmt.Fprintln(out)
					printedAny = true
				}
				fmt.Fprintf(out, "%s:\n", res.Environment)
				if res.Stale {
					fmt.Fprintln(out, "  spec not yet reconciled")
				}
				for _, c := range res.Conditions {
					fmt.Fprintf(out, "  %s\n", c)
				}
			}
			return nil
		},
	}
	return cmd
}

func dash(s string) string {
	if s == "" {
		return "-"
	}
	return s
}
