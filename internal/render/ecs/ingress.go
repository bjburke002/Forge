package ecs

import (
	"hash/fnv"
	"strings"

	"github.com/bjburke002/forge/pkg/contract/v1alpha1"
)

const (
	targetGroupId          = "TargetGroup"
	alternateTargetGroupId = "AlternateTargetGroup"
	listenerRuleId         = "ListenerRule"
	testListenerRuleId     = "TestListenerRule"
	recordSetId            = "RecordSet"
)

// ALB's maximum Priority limit for rules
const maxRulePriority = 50000

type TargetGroupProperties struct {
	Port                  int32                  `json:"Port"`
	Protocol              string                 `json:"Protocol"`
	TargetType            string                 `json:"TargetType"`
	VpcId                 any                    `json:"VpcId"`
	HealthCheckPath       string                 `json:"HealthCheckPath"`
	HealthCheckProtocol   string                 `json:"HealthCheckProtocol"`
	TargetGroupAttributes []TargetGroupAttribute `json:"TargetGroupAttributes"`
	Tags                  []Tag                  `json:"Tags"`
}

type TargetGroupAttribute struct {
	Key   string `json:"Key"`
	Value string `json:"Value"`
}

type ListenerRuleProperties struct {
	ListenerArn any             `json:"ListenerArn"`
	Priority    int             `json:"Priority"`
	Conditions  []RuleCondition `json:"Conditions"`
	Actions     []RuleAction    `json:"Actions"`
}

type RuleCondition struct {
	Field             string             `json:"Field"`
	HostHeaderConfig  *HostHeaderConfig  `json:"HostHeaderConfig,omitempty"`
	PathPatternConfig *PathPatternConfig `json:"PathPatternConfig,omitempty"`
}

type PathPatternConfig struct {
	Values []string `json:"Values"`
}

type HostHeaderConfig struct {
	Values []string `json:"Values"`
}

type RuleAction struct {
	Type           string `json:"Type"`
	TargetGroupArn any    `json:"TargetGroupArn"`
}

type RecordSetProperties struct {
	HostedZoneId any         `json:"HostedZoneId"`
	Name         string      `json:"Name"`
	Type         string      `json:"Type"`
	AliasTarget  AliasTarget `json:"AliasTarget"`
}

type AliasTarget struct {
	DNSName              any  `json:"DNSName"`
	HostedZoneId         any  `json:"HostedZoneId"`
	EvaluateTargetHealth bool `json:"EvaluateTargetHealth"`
}

type LoadBalancer struct {
	ContainerName         string                 `json:"ContainerName"`
	ContainerPort         int32                  `json:"ContainerPort"`
	TargetGroupArn        any                    `json:"TargetGroupArn"`
	AdvancedConfiguration *AdvancedConfiguration `json:"AdvancedConfiguration,omitempty"`
}

type AdvancedConfiguration struct {
	AlternateTargetGroupArn any `json:"AlternateTargetGroupArn"`
	ProductionListenerRule  any `json:"ProductionListenerRule"`
	TestListenerRule        any `json:"TestListenerRule"`
	RoleArn                 any `json:"RoleArn"`
}

func rulePriority(host string) int {
	// Hashing each unique hostname to create a priority
	// Collisions do occasionally occur with this approach but are rare
	h := fnv.New32a()
	h.Write([]byte(host))
	sum := h.Sum32()

	// Modulo + maxRulePriority of 50000 gives us a lot of room to generate nums
	sum = sum%maxRulePriority + 1

	return int(sum)
}

func ingress(fsvc *v1alpha1.ForgeService, envName string) (map[string]Resource, error) {
	if fsvc.Spec.Ingress == nil {
		return nil, nil
	}

	host := fsvc.Spec.Ingress.Host
	targetGroupProps := TargetGroupProperties{
		Port:                fsvc.Spec.Runtime.Port,
		Protocol:            "HTTP",
		TargetType:          "ip",
		VpcId:               ref(paramVPCId),
		HealthCheckPath:     fsvc.Spec.Health.Readiness,
		HealthCheckProtocol: "HTTP",
		TargetGroupAttributes: []TargetGroupAttribute{
			{
				Key:   "deregistration_delay.timeout_seconds",
				Value: "30",
			},
		},
		Tags: serviceTags(fsvc.Name, envName, fsvc.Spec.Owner),
	}

	hostCondition := RuleCondition{
		Field:            "host-header",
		HostHeaderConfig: &HostHeaderConfig{Values: []string{host}},
	}
	conditions := []RuleCondition{hostCondition}

	if path := strings.TrimSuffix(fsvc.Spec.Ingress.Path, "/"); path != "" {
		conditions = append(conditions, RuleCondition{
			Field: "path-pattern",
			PathPatternConfig: &PathPatternConfig{
				Values: []string{path, path + "/*"}},
		},
		)
	}

	listenerRuleProps := ListenerRuleProperties{
		ListenerArn: ref(paramHTTPSListenerArn),
		Priority:    rulePriority(host),
		Conditions:  conditions,
		Actions: []RuleAction{
			{
				Type:           "forward",
				TargetGroupArn: ifThen(useAlternateTargetGroup, ref(alternateTargetGroupId), ref(targetGroupId)),
			},
		},
	}

	testListenerRuleProps := listenerRuleProps
	testListenerRuleProps.ListenerArn = ref(paramTestListenerArn)
	testListenerRuleProps.Conditions = []RuleCondition{hostCondition}

	recordSetProps := RecordSetProperties{
		HostedZoneId: ref(paramHostedZoneId),
		Name:         host,
		Type:         "A",
		AliasTarget: AliasTarget{
			DNSName:              ref(paramALBDNSName),
			HostedZoneId:         ref(paramALBZoneId),
			EvaluateTargetHealth: false,
		},
	}

	targetGroup := Resource{
		Type:       "AWS::ElasticLoadBalancingV2::TargetGroup",
		Properties: targetGroupProps,
	}

	alternateTargetGroup := Resource{
		Type:       "AWS::ElasticLoadBalancingV2::TargetGroup",
		Properties: targetGroupProps,
	}

	listenerRule := Resource{
		Type:       "AWS::ElasticLoadBalancingV2::ListenerRule",
		Properties: listenerRuleProps,
	}

	testListenerRule := Resource{
		Type:       "AWS::ElasticLoadBalancingV2::ListenerRule",
		Properties: testListenerRuleProps,
	}

	recordSet := Resource{
		Type:       "AWS::Route53::RecordSet",
		Properties: recordSetProps,
	}

	results := map[string]Resource{
		targetGroupId:          targetGroup,
		listenerRuleId:         listenerRule,
		alternateTargetGroupId: alternateTargetGroup,
		testListenerRuleId:     testListenerRule,
		recordSetId:            recordSet,
		alarmId:                errorRateAlarm(fsvc, envName),
	}

	return results, nil
}
