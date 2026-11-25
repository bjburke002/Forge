package ecs

import (
	"encoding/json"
	"fmt"

	"github.com/bjburke002/forge/pkg/contract/v1alpha1"
)

const (
	serviceId        = "Service"
	scalableTargetId = "ScalableTarget"
	scalingPolicyId  = "ScalingPolicy"
)

// Matching the CRD's +kubebuilder:default=75
const defaultTargetCPU = 75

// Gives new tasks time to start before the ALB health checks count
const healthCheckGracePeriodSeconds = 60

type ServiceProperties struct {
	ServiceName                   string                         `json:"ServiceName"`
	Cluster                       any                            `json:"Cluster"`
	TaskDefinition                any                            `json:"TaskDefinition"`
	CapacityProviderStrategy      []CapacityProviderStrategyItem `json:"CapacityProviderStrategy"`
	NetworkConfiguration          NetworkConfiguration           `json:"NetworkConfiguration"`
	LoadBalancers                 []LoadBalancer                 `json:"LoadBalancers,omitempty"`
	HealthCheckGracePeriodSeconds int                            `json:"HealthCheckGracePeriodSeconds,omitempty"`
	DeploymentConfiguration       DeploymentConfiguration        `json:"DeploymentConfiguration"`
	ServiceConnectConfiguration   *ServiceConnectConfiguration   `json:"ServiceConnectConfiguration,omitempty"`
	PropagateTags                 string                         `json:"PropagateTags"`
	Tags                          []Tag                          `json:"Tags"`
}

type CapacityProviderStrategyItem struct {
	CapacityProvider string `json:"CapacityProvider"`
	Base             int    `json:"Base,omitempty"`
	Weight           int    `json:"Weight"`
}

type NetworkConfiguration struct {
	AwsvpcConfiguration AwsvpcConfiguration `json:"AwsvpcConfiguration"`
}

type AwsvpcConfiguration struct {
	Subnets        any    `json:"Subnets"`
	SecurityGroups []any  `json:"SecurityGroups"`
	AssignPublicIp string `json:"AssignPublicIp"`
}

type DeploymentConfiguration struct {
	MinimumHealthyPercent    int                      `json:"MinimumHealthyPercent"`
	MaximumPercent           int                      `json:"MaximumPercent"`
	DeploymentCircuitBreaker DeploymentCircuitBreaker `json:"DeploymentCircuitBreaker"`
	Strategy                 string                   `json:"Strategy,omitempty"`
	BakeTimeInMinutes        int                      `json:"BakeTimeInMinutes,omitempty"`
	CanaryConfiguration      *CanaryConfiguration     `json:"CanaryConfiguration,omitempty"`
	LifecycleHooks           []LifecycleHook          `json:"LifecycleHooks,omitempty"`
	Alarms                   *DeploymentAlarms        `json:"Alarms,omitempty"`
}

type DeploymentAlarms struct {
	AlarmNames []any `json:"AlarmNames"`
	Enable     bool  `json:"Enable"`
	Rollback   bool  `json:"Rollback"`
}

type CanaryConfiguration struct {
	CanaryPercent           float64 `json:"CanaryPercent"`
	CanaryBakeTimeInMinutes int     `json:"CanaryBakeTimeInMinutes"`
}

type LifecycleHook struct {
	HookTargetArn   any      `json:"HookTargetArn"`
	RoleArn         any      `json:"RoleArn"`
	LifecycleStages []string `json:"LifecycleStages"`
	HookDetails     any      `json:"HookDetails"`
}
type DeploymentCircuitBreaker struct {
	Enable   bool `json:"Enable"`
	Rollback bool `json:"Rollback"`
}

type ScalableTargetProperties struct {
	ServiceNamespace  string `json:"ServiceNamespace"`
	ScalableDimension string `json:"ScalableDimension"`
	ResourceId        any    `json:"ResourceId"`
	MinCapacity       int32  `json:"MinCapacity"`
	MaxCapacity       int32  `json:"MaxCapacity"`
}

type ScalingPolicyProperties struct {
	PolicyName                               string                      `json:"PolicyName"`
	PolicyType                               string                      `json:"PolicyType"`
	ScalingTargetId                          any                         `json:"ScalingTargetId"`
	TargetTrackingScalingPolicyConfiguration TargetTrackingConfiguration `json:"TargetTrackingScalingPolicyConfiguration"`
}

type TargetTrackingConfiguration struct {
	TargetValue                   float64                       `json:"TargetValue"`
	PredefinedMetricSpecification PredefinedMetricSpecification `json:"PredefinedMetricSpecification"`
}

type PredefinedMetricSpecification struct {
	PredefinedMetricType string `json:"PredefinedMetricType"`
}

type ServiceConnectConfiguration struct {
	Enabled   bool                    `json:"Enabled"`
	Namespace any                     `json:"Namespace"`
	Services  []ServiceConnectService `json:"Services"`
}

type ServiceConnectService struct {
	PortName      string                `json:"PortName"`
	DiscoveryName string                `json:"DiscoveryName"`
	ClientAliases []ServiceConnectAlias `json:"ClientAliases"`
}

type ServiceConnectAlias struct {
	Port    int    `json:"Port"`
	DnsName string `json:"DnsName"`
}

func serviceDefinition(fsvc *v1alpha1.ForgeService, envName string) (map[string]Resource, error) {
	name := ResourceName(fsvc.Name, envName)
	var capacityItem []CapacityProviderStrategyItem
	switch envName {
	case "dev":
		capacityItem = []CapacityProviderStrategyItem{
			{
				CapacityProvider: "FARGATE_SPOT",
				Weight:           1,
			},
		}
	case "prod":
		capacityItem = []CapacityProviderStrategyItem{
			{
				CapacityProvider: "FARGATE",
				Base:             1,
				Weight:           0,
			},
			{
				CapacityProvider: "FARGATE_SPOT",
				Weight:           1,
			},
		}
	default:
		return nil, fmt.Errorf("environment %q not supported", envName)
	}

	loadBalancers := []LoadBalancer{
		{
			ContainerName:  fsvc.Name,
			ContainerPort:  fsvc.Spec.Runtime.Port,
			TargetGroupArn: ref(targetGroupId),
			AdvancedConfiguration: &AdvancedConfiguration{
				AlternateTargetGroupArn: ref(alternateTargetGroupId),
				ProductionListenerRule:  ref(listenerRuleId),
				TestListenerRule:        ref(testListenerRuleId),
				RoleArn:                 ref(paramInfrastructureRoleArn),
			},
		},
	}

	canaryBake, finalBake := 5, 5
	if envName == "dev" {
		canaryBake, finalBake = 2, 4
	}

	serviceProperties := ServiceProperties{
		ServiceName:              name,
		Cluster:                  ref(paramClusterName),
		TaskDefinition:           ref(taskDefinitionId),
		CapacityProviderStrategy: capacityItem,
		NetworkConfiguration: NetworkConfiguration{
			AwsvpcConfiguration: AwsvpcConfiguration{
				Subnets:        ref(paramPrivateSubnetIds),
				SecurityGroups: []any{ref(paramTaskSecurityGroupId)},
				AssignPublicIp: "DISABLED",
			},
		},
		DeploymentConfiguration: DeploymentConfiguration{
			MinimumHealthyPercent: 100,
			MaximumPercent:        200,
			DeploymentCircuitBreaker: DeploymentCircuitBreaker{
				Enable:   true,
				Rollback: true,
			},
		},
		ServiceConnectConfiguration: &ServiceConnectConfiguration{
			Enabled:   true,
			Namespace: ref(paramServiceConnectNamespace),
			Services: []ServiceConnectService{
				{
					PortName:      "http",
					DiscoveryName: fsvc.Name,
					ClientAliases: []ServiceConnectAlias{
						{
							Port:    80,
							DnsName: fsvc.Name,
						},
					},
				},
			},
		},
		PropagateTags: "SERVICE",
		Tags:          serviceTags(fsvc.Name, envName, fsvc.Spec.Owner),
	}

	if fsvc.Spec.Ingress != nil {
		// HookDetails needs to be encoded as a JSON object for cloudformation which means it has to be created and marshaled first
		details, err := json.Marshal(map[string]string{
			"url": "https://" + fsvc.Spec.Ingress.Host + ":8443" + fsvc.Spec.Health.Readiness,
		})
		if err != nil {
			return nil, fmt.Errorf("hook details: %w", err)
		}

		canaryConfiguration := &CanaryConfiguration{
			CanaryPercent:           10.0,
			CanaryBakeTimeInMinutes: canaryBake,
		}
		lifecycleHooks := []LifecycleHook{
			{
				HookTargetArn:   ref(paramCanaryHookArn),
				RoleArn:         ref(paramHookInvokeRoleArn),
				LifecycleStages: []string{"POST_TEST_TRAFFIC_SHIFT"},
				HookDetails:     string(details),
			},
		}

		serviceProperties.LoadBalancers = loadBalancers
		serviceProperties.DeploymentConfiguration.Alarms = &DeploymentAlarms{
			AlarmNames: []any{ref(alarmId)},
			Enable:     true,
			Rollback:   true,
		}
		serviceProperties.DeploymentConfiguration.Strategy = "CANARY"
		serviceProperties.DeploymentConfiguration.BakeTimeInMinutes = finalBake
		serviceProperties.DeploymentConfiguration.CanaryConfiguration = canaryConfiguration
		serviceProperties.DeploymentConfiguration.LifecycleHooks = lifecycleHooks
		serviceProperties.HealthCheckGracePeriodSeconds = healthCheckGracePeriodSeconds
	}

	scalableTargetProps := ScalableTargetProperties{
		ServiceNamespace:  "ecs",
		ScalableDimension: "ecs:service:DesiredCount",
		ResourceId:        sub("service/${" + paramClusterName + "}/" + name),
		MinCapacity:       fsvc.Spec.Scaling.MinReplicas,
		MaxCapacity:       fsvc.Spec.Scaling.MaxReplicas,
	}

	targetCPU := float64(fsvc.Spec.Scaling.TargetCPU)
	if targetCPU == 0 {
		targetCPU = defaultTargetCPU
	}

	scalingPolicyProps := ScalingPolicyProperties{
		PolicyName:      name + "-cpu",
		PolicyType:      "TargetTrackingScaling",
		ScalingTargetId: ref(scalableTargetId),
		TargetTrackingScalingPolicyConfiguration: TargetTrackingConfiguration{
			TargetValue: targetCPU,
			PredefinedMetricSpecification: PredefinedMetricSpecification{
				PredefinedMetricType: "ECSServiceAverageCPUUtilization",
			},
		},
	}

	service := Resource{
		Type:       "AWS::ECS::Service",
		Properties: serviceProperties,
	}

	scalableTarget := Resource{
		Type:       "AWS::ApplicationAutoScaling::ScalableTarget",
		Properties: scalableTargetProps,
		// ResourceId is a string, so CloudFormation can't see that the target needs the service unless we DependOn
		DependsOn: []string{serviceId},
	}

	scalingPolicy := Resource{
		Type:       "AWS::ApplicationAutoScaling::ScalingPolicy",
		Properties: scalingPolicyProps,
	}

	return map[string]Resource{
		serviceId:        service,
		scalableTargetId: scalableTarget,
		scalingPolicyId:  scalingPolicy,
	}, nil
}
