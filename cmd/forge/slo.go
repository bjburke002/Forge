package main

import (
	"fmt"
	"net/http"
	"strings"
	"text/tabwriter"

	"github.com/bjburke002/forge/internal/slo"
	"github.com/prometheus/client_golang/api"
	promv1 "github.com/prometheus/client_golang/api/prometheus/v1"
	"github.com/prometheus/sigv4"

	"github.com/spf13/cobra"
)

func newSLOCmd(opts *rootOptions) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "slo <name>",
		Short: "Query Prometheus for Forge Service availability, remaining error budget and burn rates",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			name := args[0]

			if opts.cfg.AMPEndpoint == "" {
				return fmt.Errorf("ampEndpoint empty (check config.yaml ampEndpoint or FORGE_AMP_ENDPOINT)")
			}

			if opts.cfg.AWSRegion == "" {
				return fmt.Errorf("no AWS region configured (check config.yaml awsRegion or AWS_REGION)")
			}

			// prom sigv4 round tripper so we can hit AMP
			sigv4Cfg := &sigv4.SigV4Config{
				Region:      opts.cfg.AWSRegion,
				ServiceName: "aps",
			}

			rt, err := sigv4.NewSigV4RoundTripper(sigv4Cfg, http.DefaultTransport, sigv4.WithContext(cmd.Context()))
			if err != nil {
				return fmt.Errorf("creating sigv4 round tripper: %w", err)
			}

			client, err := api.NewClient(api.Config{
				Address:      opts.cfg.AMPEndpoint,
				RoundTripper: rt,
			})
			if err != nil {
				return fmt.Errorf("creating AMP client: %w", err)
			}

			promAPI := promv1.NewAPI(client)

			sloInfo, err := slo.Get(cmd.Context(), promAPI, name)
			if err != nil {
				return fmt.Errorf("querying prometheus for slo information: %w", err)
			}

			w := tabwriter.NewWriter(cmd.OutOrStdout(), 0, 0, 2, ' ', 0)

			header := []string{"ENV", "AVAILABILITY", "BUDGET LEFT", "BURN 1H", "BURN 6H"}
			fmt.Fprintln(w, strings.Join(header, "\t"))

			// To avoid a format helper for each field, passing a format string in to fDash to handle them all together
			for _, res := range sloInfo {
				row := []string{res.Environment, fDash(res.Availability, "%.3f%%"), fDash(res.Remaining, "%.1f%%"),
					fDash(res.BurnRate1h, "%.2fx"), fDash(res.BurnRate6h, "%.2fx")}
				fmt.Fprintln(w, strings.Join(row, "\t"))
			}
			w.Flush()
			return nil
		},
	}
	return cmd
}

func fDash(i *float64, f string) string {
	if i == nil {
		return "-"
	}
	return fmt.Sprintf(f, *i)
}
