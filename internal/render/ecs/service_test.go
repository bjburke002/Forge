package ecs

import (
	"reflect"
	"testing"

	"github.com/bjburke002/forge/pkg/contract/v1alpha1"
)

type serviceResources struct {
	service ServiceProperties
	target  ScalableTargetProperties
	policy  ScalingPolicyProperties
	deps    []string // the scalable target's DependsOn
}

// renderService calls serviceDefinition and returns its three resources' typed properties.
func renderService(t *testing.T, fsvc *v1alpha1.ForgeService, env string) serviceResources {
	t.Helper()
	res, err := serviceDefinition(fsvc, env)
	if err != nil {
		t.Fatalf("serviceDefinition: %v", err)
	}
	want := map[string]string{
		serviceId:        "AWS::ECS::Service",
		scalableTargetId: "AWS::ApplicationAutoScaling::ScalableTarget",
		scalingPolicyId:  "AWS::ApplicationAutoScaling::ScalingPolicy",
	}
	if len(res) != len(want) {
		t.Fatalf("got %d resources, want %d", len(res), len(want))
	}
	for id, typ := range want {
		if r, ok := res[id]; !ok || r.Type != typ {
			t.Fatalf("resource %s: got %+v, want type %s", id, r, typ)
		}
	}
	return serviceResources{
		service: res[serviceId].Properties.(ServiceProperties),
		target:  res[scalableTargetId].Properties.(ScalableTargetProperties),
		policy:  res[scalingPolicyId].Properties.(ScalingPolicyProperties),
		deps:    res[scalableTargetId].DependsOn,
	}
}

func TestServiceDefinition(t *testing.T) {
	r := renderService(t, testService(), "dev")
	s := r.service

	if s.ServiceName != "forge-payments-dev" {
		t.Errorf("service name = %q, want forge-payments-dev", s.ServiceName)
	}
	if !reflect.DeepEqual(s.Cluster, ref(paramClusterName)) ||
		!reflect.DeepEqual(s.TaskDefinition, ref(taskDefinitionId)) {
		t.Errorf("cluster %v, task definition %v; want Refs to %s and %s",
			s.Cluster, s.TaskDefinition, paramClusterName, taskDefinitionId)
	}

	// Private networking only.
	net := s.NetworkConfiguration.AwsvpcConfiguration
	if !reflect.DeepEqual(net.Subnets, ref(paramPrivateSubnetIds)) {
		t.Errorf("subnets = %v, want a Ref to %s", net.Subnets, paramPrivateSubnetIds)
	}
	if !reflect.DeepEqual(net.SecurityGroups, []any{ref(paramTaskSecurityGroupId)}) {
		t.Errorf("security groups = %v, want [Ref %s]", net.SecurityGroups, paramTaskSecurityGroupId)
	}
	if net.AssignPublicIp != "DISABLED" {
		t.Errorf("assign public IP = %q, want DISABLED", net.AssignPublicIp)
	}

	// A failed deployment rolls back (E3.6), without dropping below the running count.
	d := s.DeploymentConfiguration
	if d.MinimumHealthyPercent != 100 || d.MaximumPercent != 200 ||
		!d.DeploymentCircuitBreaker.Enable || !d.DeploymentCircuitBreaker.Rollback {
		t.Errorf("deployment configuration = %+v, want 100/200 with the circuit breaker rolling back", d)
	}
	if s.PropagateTags != "SERVICE" {
		t.Errorf("propagate tags = %q, want SERVICE", s.PropagateTags)
	}
	if !reflect.DeepEqual(s.Tags, serviceTags("payments", "dev", "payments-team")) {
		t.Errorf("tags = %+v", s.Tags)
	}

	// The scalable target names the service by string, so it must depend on it explicitly.
	tg := r.target
	if !reflect.DeepEqual(tg.ResourceId, sub("service/${ClusterName}/forge-payments-dev")) {
		t.Errorf("resource ID = %v", tg.ResourceId)
	}
	if tg.ServiceNamespace != "ecs" || tg.ScalableDimension != "ecs:service:DesiredCount" {
		t.Errorf("namespace %q, dimension %q", tg.ServiceNamespace, tg.ScalableDimension)
	}
	if tg.MinCapacity != 2 || tg.MaxCapacity != 5 {
		t.Errorf("capacity = %d–%d, want 2–5", tg.MinCapacity, tg.MaxCapacity)
	}
	if !reflect.DeepEqual(r.deps, []string{serviceId}) {
		t.Errorf("scalable target DependsOn = %v, want [%s]", r.deps, serviceId)
	}

	p := r.policy
	if p.PolicyName == "" || p.PolicyType != "TargetTrackingScaling" ||
		!reflect.DeepEqual(p.ScalingTargetId, ref(scalableTargetId)) {
		t.Errorf("policy = %+v", p)
	}
	if m := p.TargetTrackingScalingPolicyConfiguration.PredefinedMetricSpecification.PredefinedMetricType; m != "ECSServiceAverageCPUUtilization" {
		t.Errorf("metric = %q, want ECSServiceAverageCPUUtilization", m)
	}
}

func TestServiceCapacityStrategy(t *testing.T) {
	tests := []struct {
		env  string
		want []CapacityProviderStrategyItem
	}{
		{env: "dev", want: []CapacityProviderStrategyItem{
			{CapacityProvider: "FARGATE_SPOT", Weight: 1},
		}},
		// Prod keeps its first task on-demand, so a Spot shortage can't take it to zero.
		{env: "prod", want: []CapacityProviderStrategyItem{
			{CapacityProvider: "FARGATE", Base: 1, Weight: 0},
			{CapacityProvider: "FARGATE_SPOT", Weight: 1},
		}},
	}
	for _, tt := range tests {
		t.Run(tt.env, func(t *testing.T) {
			got := renderService(t, testService(), tt.env).service.CapacityProviderStrategy
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("capacity strategy = %+v, want %+v", got, tt.want)
			}
		})
	}
}

func TestServiceUnsupportedEnvironment(t *testing.T) {
	if _, err := serviceDefinition(testService(), "staging"); err == nil {
		t.Fatal("got no error for environment staging")
	}
}

func TestServiceTargetCPU(t *testing.T) {
	tests := []struct {
		name     string
		contract int32
		want     float64
	}{
		{name: "set in the contract", contract: 50, want: 50},
		// kustomize output never passes through the API server, so the CRD default may be missing.
		{name: "missing uses the CRD default", contract: 0, want: 75},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			fsvc := testService()
			fsvc.Spec.Scaling.TargetCPU = tt.contract
			got := renderService(t, fsvc, "dev").policy.TargetTrackingScalingPolicyConfiguration.TargetValue
			if got != tt.want {
				t.Errorf("target value = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestServiceLoadBalancer(t *testing.T) {
	// With ingress: attached to the target group, with both rules and the alternate target group for canaries.
	// No DependsOn: the Refs in AdvancedConfiguration order it after the rules.
	res, err := serviceDefinition(withIngress(testHost, "/"), "dev")
	if err != nil {
		t.Fatalf("serviceDefinition: %v", err)
	}
	s := res[serviceId].Properties.(ServiceProperties)
	wantLB := []LoadBalancer{{
		ContainerName:  "payments",
		ContainerPort:  8080,
		TargetGroupArn: ref(targetGroupId),
		AdvancedConfiguration: &AdvancedConfiguration{
			AlternateTargetGroupArn: ref(alternateTargetGroupId),
			ProductionListenerRule:  ref(listenerRuleId),
			TestListenerRule:        ref(testListenerRuleId),
			RoleArn:                 ref(paramInfrastructureRoleArn),
		},
	}}
	if !reflect.DeepEqual(s.LoadBalancers, wantLB) || s.HealthCheckGracePeriodSeconds != healthCheckGracePeriodSeconds {
		t.Errorf("load balancers %+v, grace period %d", s.LoadBalancers, s.HealthCheckGracePeriodSeconds)
	}
	if res[serviceId].DependsOn != nil {
		t.Errorf("DependsOn = %v, want none", res[serviceId].DependsOn)
	}

	// Without ingress: none of the above, and a rolling deployment.
	r := renderService(t, testService(), "dev")
	d := r.service.DeploymentConfiguration
	if r.service.LoadBalancers != nil || r.service.HealthCheckGracePeriodSeconds != 0 ||
		d.Strategy != "" || d.CanaryConfiguration != nil || d.LifecycleHooks != nil || d.Alarms != nil {
		t.Errorf("worker service has load balancer or canary settings: %+v", r.service)
	}
}

func TestServiceCanary(t *testing.T) {
	tests := []struct {
		env                   string
		canaryBake, finalBake int
	}{
		{env: "dev", canaryBake: 1, finalBake: 1},
		{env: "prod", canaryBake: 5, finalBake: 5},
	}
	for _, tt := range tests {
		t.Run(tt.env, func(t *testing.T) {
			d := renderService(t, withIngress(testHost, "/api"), tt.env).service.DeploymentConfiguration

			// Same deployment pattern in every environment; only the bake times differ.
			if d.Strategy != "CANARY" || d.BakeTimeInMinutes != tt.finalBake ||
				!reflect.DeepEqual(d.CanaryConfiguration, &CanaryConfiguration{CanaryPercent: 10, CanaryBakeTimeInMinutes: tt.canaryBake}) {
				t.Errorf("strategy %q, bake %d, canary %+v", d.Strategy, d.BakeTimeInMinutes, d.CanaryConfiguration)
			}

			// The smoke test hits the readiness path on the test listener, whatever ingress.path is.
			wantHook := []LifecycleHook{{
				HookTargetArn:   ref(paramCanaryHookArn),
				RoleArn:         ref(paramHookInvokeRoleArn),
				LifecycleStages: []string{"POST_TEST_TRAFFIC_SHIFT"},
				HookDetails:     `{"url":"https://` + testHost + `:8443/health/ready"}`,
			}}
			if !reflect.DeepEqual(d.LifecycleHooks, wantHook) {
				t.Errorf("hooks = %+v", d.LifecycleHooks)
			}

			if !reflect.DeepEqual(d.Alarms, &DeploymentAlarms{AlarmNames: []any{ref(alarmId)}, Enable: true, Rollback: true}) {
				t.Errorf("alarms = %+v", d.Alarms)
			}
		})
	}
}
