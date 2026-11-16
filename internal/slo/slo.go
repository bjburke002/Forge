package slo

import (
	"context"
	"fmt"
	"math"
	"time"

	promv1 "github.com/prometheus/client_golang/api/prometheus/v1"
	"github.com/prometheus/common/model"
)

type Result struct {
	Environment  string
	Availability *float64
	Remaining    *float64
	BurnRate1h   *float64
	BurnRate6h   *float64
}

var environments = []string{"dev", "prod"}

const (
	availabilityQuery = `(1 - sum by (namespace, service)
						(forge:error_budget_consumed:minutes30d{namespace="%[1]s", service="%[2]s"}) / sum by (namespace, service) 
						(forge:uptime:minutes30d{namespace="%[1]s", service="%[2]s"})) * 100`
	errorBudgetQuery = `(43.2 - sum by (namespace, service)
						(forge:error_budget_consumed:minutes30d{namespace="%[1]s", service="%[2]s"})) / 43.2 * 100`
	burnRate1hQuery = `forge:burn_rate:1h{namespace="%[1]s", service="%[2]s"}`
	burnRate6hQuery = `forge:burn_rate:6h{namespace="%[1]s", service="%[2]s"}`
)

// Get queries Prometheus managed rule groups through AMP and returns formatted results
func Get(ctx context.Context, prom promv1.API, name string) ([]Result, error) {
	results := make([]Result, 0, len(environments))

	for _, e := range environments {
		// Because we're using format placeholders in the consts, need to build the full query expression
		// with service namespace and name before querying
		var err error
		envName := name + "-" + e

		r := Result{Environment: e}

		availabilityExpr := fmt.Sprintf(availabilityQuery, envName, name)
		r.Availability, err = query(ctx, prom, availabilityExpr)
		if err != nil {
			return nil, fmt.Errorf("%s: %w", e, err)
		}

		ebExpr := fmt.Sprintf(errorBudgetQuery, envName, name)
		r.Remaining, err = query(ctx, prom, ebExpr)
		if err != nil {
			return nil, fmt.Errorf("%s: %w", e, err)
		}

		burn1hExpr := fmt.Sprintf(burnRate1hQuery, envName, name)
		r.BurnRate1h, err = query(ctx, prom, burn1hExpr)
		if err != nil {
			return nil, fmt.Errorf("%s: %w", e, err)
		}

		burn6hExpr := fmt.Sprintf(burnRate6hQuery, envName, name)
		r.BurnRate6h, err = query(ctx, prom, burn6hExpr)
		if err != nil {
			return nil, fmt.Errorf("%s: %w", e, err)
		}
		results = append(results, r)
	}
	return results, nil
}

func query(ctx context.Context, prom promv1.API, expr string) (*float64, error) {
	// Ignoring warnings because AMP rarely returns them for these queries
	result, _, err := prom.Query(ctx, expr, time.Now())
	if err != nil {
		return nil, fmt.Errorf("running query: %q: %w", expr, err)
	}

	vector, ok := result.(model.Vector)
	if !ok {
		return nil, fmt.Errorf("expected vector, got %s", result.Type())
	}
	if len(vector) == 0 {
		return nil, nil
	}

	val := float64(vector[0].Value)

	if math.IsNaN(val) {
		return nil, nil
	}
	return &val, nil

}
