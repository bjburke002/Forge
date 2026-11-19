package ecs

import (
	"reflect"
	"testing"

	"github.com/bjburke002/forge/pkg/contract/v1alpha1"
)

const testHost = "payments-dev.forgeappservices.com"

func withIngress(host, path string) *v1alpha1.ForgeService {
	fsvc := testService()
	fsvc.Spec.Ingress = &v1alpha1.IngressSpec{Host: host, Path: path}
	fsvc.Spec.Health = v1alpha1.HealthSpec{Readiness: "/health/ready", Liveness: "/health/live"}
	return fsvc
}

func renderIngress(t *testing.T, fsvc *v1alpha1.ForgeService) (TargetGroupProperties, ListenerRuleProperties, RecordSetProperties) {
	t.Helper()
	res, err := ingress(fsvc, "dev")
	if err != nil {
		t.Fatalf("ingress: %v", err)
	}
	want := map[string]string{
		targetGroupId:  "AWS::ElasticLoadBalancingV2::TargetGroup",
		listenerRuleId: "AWS::ElasticLoadBalancingV2::ListenerRule",
		recordSetId:    "AWS::Route53::RecordSet",
	}
	if len(res) != len(want) {
		t.Fatalf("got %d resources, want %d", len(res), len(want))
	}
	for id, typ := range want {
		if r, ok := res[id]; !ok || r.Type != typ {
			t.Fatalf("resource %s: got %+v, want type %s", id, r, typ)
		}
	}
	return res[targetGroupId].Properties.(TargetGroupProperties),
		res[listenerRuleId].Properties.(ListenerRuleProperties),
		res[recordSetId].Properties.(RecordSetProperties)
}

func TestRulePriority(t *testing.T) {
	dev, prod := rulePriority(testHost), rulePriority("payments.forgeappservices.com")
	if dev != rulePriority(testHost) {
		t.Error("same host gave different priorities")
	}
	for _, p := range []int{dev, prod} {
		if p < 1 || p > maxRulePriority {
			t.Errorf("priority %d outside 1–%d", p, maxRulePriority)
		}
	}
	if dev == prod {
		t.Errorf("dev and prod hosts share priority %d", dev)
	}
}

func TestIngress(t *testing.T) {
	tg, rule, rec := renderIngress(t, withIngress(testHost, "/"))

	if tg.TargetType != "ip" || tg.Port != 8080 || tg.HealthCheckPath != "/health/ready" ||
		!reflect.DeepEqual(tg.VpcId, ref(paramVPCId)) {
		t.Errorf("target group = %+v", tg)
	}
	if !reflect.DeepEqual(tg.TargetGroupAttributes,
		[]TargetGroupAttribute{{Key: "deregistration_delay.timeout_seconds", Value: "30"}}) {
		t.Errorf("attributes = %+v", tg.TargetGroupAttributes)
	}

	if !reflect.DeepEqual(rule.ListenerArn, ref(paramHTTPSListenerArn)) {
		t.Errorf("listener = %v, want a Ref to %s", rule.ListenerArn, paramHTTPSListenerArn)
	}
	if rule.Priority != rulePriority(testHost) {
		t.Errorf("priority = %d", rule.Priority)
	}
	wantCond := []RuleCondition{{Field: "host-header", HostHeaderConfig: &HostHeaderConfig{Values: []string{testHost}}}}
	if !reflect.DeepEqual(rule.Conditions, wantCond) {
		t.Errorf("conditions = %+v, want host only", rule.Conditions)
	}
	if !reflect.DeepEqual(rule.Actions, []RuleAction{{Type: "forward", TargetGroupArn: ref(targetGroupId)}}) {
		t.Errorf("actions = %+v", rule.Actions)
	}

	// The record lives in our zone; the alias points at the ALB's zone.
	if rec.Name != testHost || rec.Type != "A" || !reflect.DeepEqual(rec.HostedZoneId, ref(paramHostedZoneId)) {
		t.Errorf("record = %+v", rec)
	}
	want := AliasTarget{DNSName: ref(paramALBDNSName), HostedZoneId: ref(paramALBZoneId)}
	if !reflect.DeepEqual(rec.AliasTarget, want) {
		t.Errorf("alias = %+v, want %+v", rec.AliasTarget, want)
	}
}

func TestIngressPath(t *testing.T) {
	tests := []struct {
		path string
		want []string // nil: no path condition
	}{
		{path: "", want: nil},
		{path: "/", want: nil},
		{path: "/api", want: []string{"/api", "/api/*"}},
		{path: "/api/", want: []string{"/api", "/api/*"}},
	}
	for _, tt := range tests {
		t.Run("path "+tt.path, func(t *testing.T) {
			_, rule, _ := renderIngress(t, withIngress(testHost, tt.path))
			var got []string
			for _, c := range rule.Conditions {
				if c.Field == "path-pattern" {
					got = c.PathPatternConfig.Values
				}
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("path pattern = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestIngressNone(t *testing.T) {
	res, err := ingress(testService(), "dev")
	if err != nil || len(res) != 0 {
		t.Errorf("got %d resources, err %v; want none", len(res), err)
	}
}
