package ecs

import (
	"reflect"
	"testing"

	"github.com/bjburke002/forge/pkg/contract/v1alpha1"
)

func TestErrorRateAlarm(t *testing.T) {
	tests := []struct {
		name             string
		fsvc             *v1alpha1.ForgeService
		groups           []any
		errors, requests string
	}{
		{
			name: "ingress",
			fsvc: withIngress(testHost, "/"),
			groups: []any{
				getAtt(targetGroupId, "TargetGroupFullName"), getAtt(alternateTargetGroupId, "TargetGroupFullName"),
				getAtt(targetGroupId, "TargetGroupFullName"), getAtt(alternateTargetGroupId, "TargetGroupFullName"),
			},
			errors:   "FILL(e1, 0) + FILL(e2, 0) + FILL(s5, 0)",
			requests: "FILL(r1, 0) + FILL(r2, 0) + FILL(s2, 0) + FILL(s3, 0) + FILL(s4, 0) + FILL(s5, 0)",
		},
		{
			// No target groups to watch, so Service Connect only.
			name:     "no ingress",
			fsvc:     testService(),
			errors:   "FILL(s5, 0)",
			requests: "FILL(s2, 0) + FILL(s3, 0) + FILL(s4, 0) + FILL(s5, 0)",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			a := errorRateAlarm(tt.fsvc, "prod").Properties.(AlarmProperties)

			if a.AlarmName != "forge-payments-prod-5xx" || a.TreatMissingData != "notBreaching" {
				t.Errorf("name %q, missing data %q", a.AlarmName, a.TreatMissingData)
			}

			// Only the ratio is returned.
			var groups, sc []any
			exprs := map[string]string{}
			returned := 0
			for _, m := range a.Metrics {
				if m.MetricStat != nil {
					switch d := m.MetricStat.Metric.Dimensions; m.MetricStat.Metric.Namespace {
					case "AWS/ApplicationELB":
						groups = append(groups, d[1].Value)
					case "AWS/ECS":
						sc = append(sc, m.MetricStat.Metric.MetricName)
						want := []MetricDimension{{Name: "TargetDiscoveryName", Value: "payments-prod"}}
						if !reflect.DeepEqual(d, want) {
							t.Errorf("service connect dimensions = %+v", d)
						}
					}
				}
				exprs[m.Id] = m.Expression
				if m.ReturnData {
					returned++
					if m.Id != "ratio" {
						t.Errorf("returned %q, want ratio", m.Id)
					}
				}
			}
			if !reflect.DeepEqual(groups, tt.groups) {
				t.Errorf("target groups = %v", groups)
			}
			wantSC := []any{"HTTPCode_Target_2XX_Count", "HTTPCode_Target_3XX_Count", "HTTPCode_Target_4XX_Count", "HTTPCode_Target_5XX_Count"}
			if !reflect.DeepEqual(sc, wantSC) {
				t.Errorf("service connect metrics = %v", sc)
			}
			if exprs["errors"] != tt.errors || exprs["requests"] != tt.requests {
				t.Errorf("errors %q, requests %q", exprs["errors"], exprs["requests"])
			}
			// CloudWatch's hard limit for a metric math alarm.
			if n := len(groups) + len(sc); n > 10 {
				t.Errorf("%d metrics, want at most 10", n)
			}
			if returned != 1 {
				t.Errorf("%d metrics return data, want 1", returned)
			}
		})
	}
}
