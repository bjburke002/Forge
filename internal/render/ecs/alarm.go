package ecs

import (
	"fmt"

	"github.com/bjburke002/forge/pkg/contract/v1alpha1"
)

const alarmId = "ErrorRateAlarm"

const (
	alarmThresholdPercent = 5
	alarmMinRequests      = 20
	alarmPeriodSeconds    = 60
)

type AlarmProperties struct {
	AlarmName          string            `json:"AlarmName"`
	AlarmDescription   string            `json:"AlarmDescription"`
	ComparisonOperator string            `json:"ComparisonOperator"`
	Threshold          float64           `json:"Threshold"`
	EvaluationPeriods  int               `json:"EvaluationPeriods"`
	DatapointsToAlarm  int               `json:"DatapointsToAlarm"`
	TreatMissingData   string            `json:"TreatMissingData"`
	Metrics            []MetricDataQuery `json:"Metrics"`
}

type MetricDataQuery struct {
	Id         string      `json:"Id"`
	Expression string      `json:"Expression,omitempty"`
	MetricStat *MetricStat `json:"MetricStat,omitempty"`
	Label      string      `json:"Label,omitempty"`
	ReturnData bool        `json:"ReturnData"`
}

type MetricStat struct {
	Metric Metric `json:"Metric"`
	Period int    `json:"Period"`
	Stat   string `json:"Stat"`
}

type Metric struct {
	Namespace  string            `json:"Namespace"`
	MetricName string            `json:"MetricName"`
	Dimensions []MetricDimension `json:"Dimensions"`
}

type MetricDimension struct {
	Name  string `json:"Name"`
	Value any    `json:"Value"`
}

func getAtt(resource, attribute string) map[string][]string {
	return map[string][]string{"Fn::GetAtt": {resource, attribute}}
}

// errorRateAlarm watches the 5xx ratio across both target groups combined
func errorRateAlarm(fsvc *v1alpha1.ForgeService, envName string) Resource {
	albMetric := func(id, metricName, targetGroup string) MetricDataQuery {
		return MetricDataQuery{
			Id: id,
			MetricStat: &MetricStat{
				Metric: Metric{
					Namespace:  "AWS/ApplicationELB",
					MetricName: metricName,
					Dimensions: []MetricDimension{
						{Name: "LoadBalancer", Value: ref(paramALBFullName)},
						{Name: "TargetGroup", Value: getAtt(targetGroup, "TargetGroupFullName")},
					},
				},
				Period: alarmPeriodSeconds,
				Stat:   "Sum",
			},
		}
	}

	return Resource{
		Type: "AWS::CloudWatch::Alarm",
		Properties: AlarmProperties{
			AlarmName:          ResourceName(fsvc.Name, envName) + "-5xx",
			AlarmDescription:   "Rolls back a deployment of " + ResourceName(fsvc.Name, envName) + " on a high 5xx ratio",
			ComparisonOperator: "GreaterThanThreshold",
			Threshold:          alarmThresholdPercent,
			EvaluationPeriods:  3,
			DatapointsToAlarm:  2,
			TreatMissingData:   "notBreaching",
			Metrics: []MetricDataQuery{
				albMetric("e1", "HTTPCode_Target_5XX_Count", targetGroupId),
				albMetric("e2", "HTTPCode_Target_5XX_Count", alternateTargetGroupId),
				albMetric("r1", "RequestCount", targetGroupId),
				albMetric("r2", "RequestCount", alternateTargetGroupId),
				{Id: "errors", Expression: "FILL(e1, 0) + FILL(e2, 0)"},
				{Id: "requests", Expression: "FILL(r1, 0) + FILL(r2, 0)"},
				{
					Id:         "ratio",
					Expression: fmt.Sprintf("IF(requests >= %d, 100 * errors / requests, 0)", alarmMinRequests),
					Label:      "5xx %",
					ReturnData: true,
				},
			},
		},
	}
}
