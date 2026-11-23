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

	// Both target groups are watched, and only the ratio is returned.
	var groups []any
	returned := 0
	for _, m := range a.Metrics {
		if m.MetricStat != nil {
			groups = append(groups, m.MetricStat.Metric.Dimensions[1].Value)
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
	if returned != 1 {
		t.Errorf("%d metrics return data, want 1", returned)
	}
}
