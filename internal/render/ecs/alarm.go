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

// errorRateAlarm watches the 5xx ratio across both target groups and Service Connect combined
func errorRateAlarm(fsvc *v1alpha1.ForgeService, envName string) Resource {
	name := ResourceName(fsvc.Name, envName)
	stat := func(id, namespace, metricName string, dims ...MetricDimension) MetricDataQuery {
		return MetricDataQuery{
			Id: id,
			MetricStat: &MetricStat{
				Metric: Metric{Namespace: namespace, MetricName: metricName, Dimensions: dims},
				Period: alarmPeriodSeconds,
				Stat:   "Sum",
			},
		}
	}
	alb := func(id, metricName, targetGroup string) MetricDataQuery {
		return stat(id, "AWS/ApplicationELB", metricName,
			MetricDimension{Name: "LoadBalancer", Value: ref(paramALBFullName)},
			MetricDimension{Name: "TargetGroup", Value: getAtt(targetGroup, "TargetGroupFullName")})
	}
	// Service Connect calls into this service from every caller; requests are the sum of its status codes.
	sc := func(code int) MetricDataQuery {
		return stat(fmt.Sprintf("s%d", code), "AWS/ECS", fmt.Sprintf("HTTPCode_Target_%dXX_Count", code),
			MetricDimension{Name: "TargetDiscoveryName", Value: discoveryName(fsvc.Name, envName)})
	}

	return Resource{
		Type: "AWS::CloudWatch::Alarm",
		Properties: AlarmProperties{
			AlarmName:          name + "-5xx",
			AlarmDescription:   "Rolls back a deployment of " + name + " on a high 5xx ratio",
			ComparisonOperator: "GreaterThanThreshold",
			Threshold:          alarmThresholdPercent,
			EvaluationPeriods:  3,
			DatapointsToAlarm:  2,
			TreatMissingData:   "notBreaching",
			Metrics: []MetricDataQuery{
				alb("e1", "HTTPCode_Target_5XX_Count", targetGroupId),
				alb("e2", "HTTPCode_Target_5XX_Count", alternateTargetGroupId),
				alb("r1", "RequestCount", targetGroupId),
				alb("r2", "RequestCount", alternateTargetGroupId),
				sc(2), sc(3), sc(4), sc(5),
				{Id: "errors", Expression: "FILL(e1, 0) + FILL(e2, 0) + FILL(s5, 0)"},
				{Id: "requests", Expression: "FILL(r1, 0) + FILL(r2, 0) + FILL(s2, 0) + FILL(s3, 0) + FILL(s4, 0) + FILL(s5, 0)"},
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
