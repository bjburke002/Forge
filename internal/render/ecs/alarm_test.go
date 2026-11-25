package ecs

import (
	"reflect"
	"testing"
)

func TestErrorRateAlarm(t *testing.T) {
	a := errorRateAlarm(testService(), "prod").Properties.(AlarmProperties)

	if a.AlarmName != "forge-payments-prod-5xx" || a.TreatMissingData != "notBreaching" {
		t.Errorf("name %q, missing data %q", a.AlarmName, a.TreatMissingData)
	}

	// Both target groups and Service Connect are watched, and only the ratio is returned.
	var groups, sc []any
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
		if m.ReturnData {
			returned++
			if m.Id != "ratio" {
				t.Errorf("returned %q, want ratio", m.Id)
			}
		}
	}
	want := []any{
		getAtt(targetGroupId, "TargetGroupFullName"), getAtt(alternateTargetGroupId, "TargetGroupFullName"),
		getAtt(targetGroupId, "TargetGroupFullName"), getAtt(alternateTargetGroupId, "TargetGroupFullName"),
	}
	if !reflect.DeepEqual(groups, want) {
		t.Errorf("target groups = %v", groups)
	}
	wantSC := []any{"HTTPCode_Target_2XX_Count", "HTTPCode_Target_3XX_Count", "HTTPCode_Target_4XX_Count", "HTTPCode_Target_5XX_Count"}
	if !reflect.DeepEqual(sc, wantSC) {
		t.Errorf("service connect metrics = %v", sc)
	}
	// CloudWatch's hard limit for a metric math alarm.
	if n := len(groups) + len(sc); n > 10 {
		t.Errorf("%d metrics, want at most 10", n)
	}
	if returned != 1 {
		t.Errorf("%d metrics return data, want 1", returned)
	}
}
