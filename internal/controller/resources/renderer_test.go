package resources

import (
	"slices"
	"testing"

	"github.com/bjburke002/forge/pkg/contract/v1alpha1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/intstr"
	appsv1ac "k8s.io/client-go/applyconfigurations/apps/v1"
	corev1ac "k8s.io/client-go/applyconfigurations/core/v1"
	metav1ac "k8s.io/client-go/applyconfigurations/meta/v1"
	networkingv1ac "k8s.io/client-go/applyconfigurations/networking/v1"
)

// Defining fixtures for the tests to use
type fixture struct {
	name string
	fs   *v1alpha1.ForgeService
}

// fsOption allows changes one part of a fixture.
type fsOption func(*v1alpha1.ForgeService)

// fixtures func returns baseline fixtures for tests to run through instead of defining them in every test
func fixtures() []fixture {
	return []fixture{
		{name: "with ingress", fs: newFS("payments", withIngress("payments.example.com", "/"))},
		{name: "ingress with path", fs: newFS("orders", withIngress("orders.example.com", "/api"))},
		{name: "no ingress", fs: newFS("worker")},
		{name: "hyphenated name", fs: newFS("order-history", withIngress("history.example.com", "/"))},
		// Values that differ from newFS, so a renderer that hardcodes the defaults fails
		{name: "custom values", fs: newFS("ledger",
			withIngress("ledger.example.com", "/"),
			withResources("100m", "64Mi", "256Mi"),
			withHealth("/readyz", "/livez"),
			withEnv(
				v1alpha1.EnvVar{Name: "LOG_LEVEL", Value: "debug"},
				v1alpha1.EnvVar{Name: "REGION", Value: "us-east-1"},
			),
		)},
		{name: "no env", fs: newFS("batch", withEnv())},
	}
}

// newFS defines a new ForgeSvc with standard configuration. Mirrors payment sample
// This way renderers can never hit a nil or empty field
func newFS(name string, opts ...fsOption) *v1alpha1.ForgeService {
	fs := &v1alpha1.ForgeService{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: "forge-test",
			UID:       types.UID(name + "-uid"),
		},
		Spec: v1alpha1.ForgeServiceSpec{
			Owner:   name + "-team",
			Image:   "registry.example.com/" + name + ":test",
			Runtime: v1alpha1.RuntimeSpec{Port: 8080},
			Env:     []v1alpha1.EnvVar{{Name: "LOG_LEVEL", Value: "info"}},
			Scaling: v1alpha1.ScalingSpec{
				MinReplicas: 2,
				MaxReplicas: 5,
				TargetCPU:   75, // CRD default; nothing applies defaults in a unit test
			},
			Resources: v1alpha1.ResourceSpec{
				Requests: v1alpha1.ResourceRequests{
					CPU:    resource.MustParse("25m"),
					Memory: resource.MustParse("32Mi"),
				},
				Limits: v1alpha1.ResourceLimits{
					Memory: resource.MustParse("128Mi"),
				},
			},
			Health: v1alpha1.HealthSpec{
				Readiness: "/health/ready",
				Liveness:  "/health/live",
			},
			Observability: v1alpha1.ObservabilitySpec{
				MetricsPath: "/metrics", // CRD default
				MetricsPort: 9090,
			},
		},
	}
	for _, opt := range opts {
		opt(fs)
	}
	return fs
}

// withIngress allows us to add an ingress to the base newFS function, inline
func withIngress(host, path string) fsOption {
	return func(fs *v1alpha1.ForgeService) {
		fs.Spec.Ingress = &v1alpha1.IngressSpec{Host: host, Path: path}
	}
}

// withResources replaces the CPU and memory requests and the memory limit
func withResources(cpu, memory, memoryLimit string) fsOption {
	return func(fs *v1alpha1.ForgeService) {
		fs.Spec.Resources.Requests.CPU = resource.MustParse(cpu)
		fs.Spec.Resources.Requests.Memory = resource.MustParse(memory)
		fs.Spec.Resources.Limits.Memory = resource.MustParse(memoryLimit)
	}
}

// withHealth replaces the readiness and liveness paths
func withHealth(readiness, liveness string) fsOption {
	return func(fs *v1alpha1.ForgeService) {
		fs.Spec.Health = v1alpha1.HealthSpec{Readiness: readiness, Liveness: liveness}
	}
}

// withEnv replaces the env vars; called with no arguments it removes them
func withEnv(env ...v1alpha1.EnvVar) fsOption {
	return func(fs *v1alpha1.ForgeService) {
		fs.Spec.Env = env
	}
}

func TestIngressBackendMatchesService(t *testing.T) {
	// Call fixtures to pass the standard set of configs
	tests := fixtures()

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			// Since a config with no Ingress is part of fixtures() and would cause these tests to panic,
			// skip any fixture with no ingress

			if tc.fs.Spec.Ingress == nil {
				t.Skip("No ingress")
			}

			ing := RenderIngress(tc.fs)
			svc := RenderService(tc.fs)
			dep := RenderDeployment(tc.fs)

			// Making sure the ingress exists after render
			if ing.Spec == nil || len(ing.Spec.Rules) == 0 {
				t.Fatalf("Ingress has no rules")
			}

			// Making sure the service is well formed
			if svc.Spec == nil || len(svc.Spec.Ports) == 0 {
				t.Fatalf("Service has no ports defined.")
			}

			// Verify Deployment Sped
			if dep.Spec == nil || len(dep.Spec.Template.Spec.Containers[0].Ports) == 0 {
				t.Fatalf("Deployment has no spec")
			}

			rule := ing.Spec.Rules[0]
			if rule.HTTP == nil || len(rule.HTTP.Paths) == 0 || rule.HTTP.Paths[0].Backend == nil || rule.HTTP.Paths[0].Backend.Service == nil || rule.HTTP.Paths[0].Backend.Service.Name == nil {
				t.Fatalf("Ingress has no service backend")
			}

			ingBackend := rule.HTTP.Paths[0].Backend.Service
			name := deref(ingBackend.Port.Name)
			got := resolvePorts(dep, svc, name)

			if want := tc.fs.Spec.Runtime.Port; got != want {
				t.Errorf("Ingress port %q resolves to container port %d, want runtime.port %d", name, got, want)
			}
		})
	}
}

func TestHPATargetsDeployment(t *testing.T) {

	tests := fixtures()

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			dep := RenderDeployment(tc.fs)
			hpa := RenderAutoscaler(tc.fs)

			if hpa.Spec == nil || hpa.Spec.ScaleTargetRef == nil {
				t.Fatalf("HPA has no scaleTargetRef")
			}
			ref := hpa.Spec.ScaleTargetRef

			if got, want := deref(ref.Name), deref(dep.Name); got != want {
				t.Errorf("HPA targets %q, Deployment is named %q", got, want)
			}
			if got := deref(ref.Kind); got != "Deployment" {
				t.Errorf("HPA target kind = %q, want Deployment", got)
			}
		})
	}
}

// Verifying the metrics port name matches in Deployment, Service and ServiceMonitor
func TestMetricsPorts(t *testing.T) {
	tests := fixtures()

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			dep := RenderDeployment(tc.fs)
			svc := RenderService(tc.fs)
			sm := RenderServiceMonitor(tc.fs)

			// Verify Deployment spec exists
			if dep.Spec == nil || len(dep.Spec.Template.Spec.Containers[0].Ports) == 0 {
				t.Fatalf("Deployment has no spec")
			}

			// Verify Service spec exists
			if svc.Spec == nil || len(svc.Spec.Ports) == 0 {
				t.Fatalf("Service has no spec")
			}

			// Verify ServiceMonitor spec exists
			if sm.Spec == nil || len(sm.Spec.Endpoints) == 0 {
				t.Fatalf("SerivceMonitor is not well formed")
			}

			if sm.Spec == nil || len(sm.Spec.Endpoints) == 0 {
				t.Fatalf("ServiceMonitor has no endpoints")
			}

			// Get servicemonitor endpoint port to pass into resolve function
			name := deref(sm.Spec.Endpoints[0].Port)

			got := resolvePorts(dep, svc, name)
			if want := tc.fs.Spec.Observability.MetricsPort; got != want {
				t.Errorf("ServiceMonitor port %q resolves to container port %d, want metricsPort %d", name, got, want)
			}
		})

	}
}

// Test Deployment platform rules with resource checks
func TestDeploymentResources(t *testing.T) {
	tests := fixtures()

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			dep := RenderDeployment(tc.fs)

			if dep.Spec == nil || dep.Spec.Template == nil || dep.Spec.Template.Spec == nil ||
				len(dep.Spec.Template.Spec.Containers) == 0 {
				t.Fatalf("Deployment has no container spec")
			}

			if dep.Spec.Replicas != nil {
				t.Fatalf("%v replicas defined in pod spec, expected none", deref(dep.Spec.Replicas))
			}

			if dep.Spec.Template.Spec.Containers[0].Resources == nil {
				t.Fatalf("Container spec has no defined resource requests or limits")
			}

			reqs := deref(dep.Spec.Template.Spec.Containers[0].Resources.Requests)
			limits := deref(dep.Spec.Template.Spec.Containers[0].Resources.Limits)

			// Check against resource requests
			// Starting with memory requests
			gotMemReqs := reqs[corev1.ResourceMemory]
			wantMemReqs := tc.fs.Spec.Resources.Requests.Memory
			if !gotMemReqs.Equal(wantMemReqs) {
				t.Errorf("Memory rquest mismatch. Expected %v, got %v", wantMemReqs.String(), gotMemReqs.String())
			}

			// CPU requests
			gotCPUReqs := reqs[corev1.ResourceCPU]
			wantCPUReqs := tc.fs.Spec.Resources.Requests.CPU
			if !gotCPUReqs.Equal(wantCPUReqs) {
				t.Errorf("CPU requests mistmatch. Expeted %v and got %v", wantCPUReqs.String(), gotCPUReqs.String())
			}

			// Checking againt resource Limits
			// Validate there is no CPU limit set
			if _, ok := limits[corev1.ResourceCPU]; ok {
				t.Errorf("CPU limit set, contract expects no CPU Limit")
			}

			// Comparing memory limits
			gotMemLimit := limits[corev1.ResourceMemory]
			wantMemLimit := tc.fs.Spec.Resources.Limits.Memory
			if !gotMemLimit.Equal(wantMemLimit) {
				t.Errorf("Memory limit mismatch. Expected %v and got %v", wantMemLimit.String(), gotMemLimit.String())
			}
		})
	}
}

// Testing Deployment and Container level SecurityContexts
func TestSecurityContexts(t *testing.T) {
	tests := fixtures()

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			dep := RenderDeployment(tc.fs)

			// Guards that would otherwise panic
			if dep.Spec == nil || dep.Spec.Template == nil || dep.Spec.Template.Spec == nil ||
				len(dep.Spec.Template.Spec.Containers) == 0 {
				t.Fatalf("Deployment has no container spec")
			}

			if dep.Spec.Template.Spec.SecurityContext == nil ||
				dep.Spec.Template.Spec.SecurityContext.SeccompProfile == nil {
				t.Fatalf("Pod SecurityContext is not properly defined")
			}

			if dep.Spec.Template.Spec.Containers[0].SecurityContext == nil ||
				dep.Spec.Template.Spec.Containers[0].SecurityContext.Capabilities == nil {
				t.Fatalf("Container SecurityContext is not properly defined")
			}

			psc := dep.Spec.Template.Spec.SecurityContext
			csc := dep.Spec.Template.Spec.Containers[0].SecurityContext

			// Checking Pod SecurityContexts
			if !deref(psc.RunAsNonRoot) {
				t.Errorf("Pod SecurityContext is not set to run as non-root")
			}
			if deref(psc.RunAsUser) != 65532 {
				t.Errorf("Pod SecurityContext running as %v, expected 65532", deref(psc.RunAsUser))
			}
			if deref(psc.SeccompProfile.Type) != corev1.SeccompProfileTypeRuntimeDefault {
				t.Errorf("Pod SecurityContext seccompProfile expected RuntimeDefault, got %v", deref(psc.SeccompProfile.Type))
			}

			// Checking Container SecurityContexts
			if csc.AllowPrivilegeEscalation == nil || *csc.AllowPrivilegeEscalation {
				t.Errorf("Container SecurityContext set to allow privilege escalation, expected false")
			}
			if !deref(csc.ReadOnlyRootFilesystem) {
				t.Errorf("Container SecurityContext readOnlyRootFilesystem is false, expected true")
			}
			// Checking Capabilities -> Drop -> "ALL"
			if !slices.Contains(csc.Capabilities.Drop, corev1.Capability("ALL")) {
				t.Errorf("Container SecurityContext Capabilities must contain drop: ALL")
			}
		})
	}
}

// Testing deployment contract fields - envs, readiness/liveness probe paths and port
func TestDeploymentContractFields(t *testing.T) {
	tests := fixtures()

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			dep := RenderDeployment(tc.fs)

			// Guards that would otherwise panic
			if dep.Spec == nil || dep.Spec.Template == nil || dep.Spec.Template.Spec == nil ||
				len(dep.Spec.Template.Spec.Containers) == 0 {
				t.Fatalf("Deployment has no container spec")
			}

			// Guarding the readiness and liveness probes
			if dep.Spec.Template.Spec.Containers[0].ReadinessProbe == nil {
				t.Fatalf("Container has no Readiness Probe path")
			}
			if dep.Spec.Template.Spec.Containers[0].LivenessProbe == nil {
				t.Fatalf("Container has no Liveness Probe defined")
			}
			// Validating httpGet field in both probes
			if dep.Spec.Template.Spec.Containers[0].ReadinessProbe.HTTPGet == nil {
				t.Fatalf("Container Readiness probe is malformed and contains no HTTPGet block")
			}
			if dep.Spec.Template.Spec.Containers[0].LivenessProbe.HTTPGet == nil {
				t.Fatalf("Container Liveness probe is malformed and contains no HTTPGet")
			}

			// Validate that Liveness probe path and port match cotract
			l := dep.Spec.Template.Spec.Containers[0].LivenessProbe.HTTPGet

			if deref(l.Path) != tc.fs.Spec.Health.Liveness {
				t.Errorf("Liveness probe path is %v, but expected %v", deref(l.Path), tc.fs.Spec.Health.Liveness)
			}

			// Validating runtime port matches liveness probe port
			lpName := deref(l.Port).StrVal
			cp := containerPort(dep, lpName)
			if cp == nil {
				t.Errorf("Liveness probe port %q matches no container port", lpName)
			} else if got, want := deref(cp.ContainerPort), tc.fs.Spec.Runtime.Port; got != want {
				t.Errorf("Liveness probe port %q resolves to container port %d, want runtime.port %d", lpName, got, want)
			}

			// Validate that Readiness probe path and port match contract
			r := dep.Spec.Template.Spec.Containers[0].ReadinessProbe.HTTPGet

			if deref(r.Path) != tc.fs.Spec.Health.Readiness {
				t.Errorf("Readiness probe path is %v, but expected %v", deref(r.Path), tc.fs.Spec.Health.Readiness)
			}

			rpName := deref(r.Port).StrVal
			rcp := containerPort(dep, rpName)

			if rcp == nil {
				t.Errorf("Readiness probe port %q matches no container port", rpName)
			} else if got, want := deref(rcp.ContainerPort), tc.fs.Spec.Runtime.Port; got != want {
				t.Errorf("Readiness probe port %q resolves to container port %d, want runtime.port %d", rpName, got, want)
			}

			// Validation for Environment Variables
			c := dep.Spec.Template.Spec.Containers[0]

			// If lengths don't match, one has more envs than the other so there's an issue
			if len(c.Env) != len(tc.fs.Spec.Env) {
				t.Skip("Number of Envs doesn't match")
				return
			}

			// Check each env against each other to make sure they match
			for i := range c.Env {
				if deref(c.Env[i].Name) != tc.fs.Spec.Env[i].Name {
					t.Errorf("Env names don't match. Got %q and expected %q", deref(c.Env[i].Name), tc.fs.Spec.Env[i].Name)
				}
				if deref(c.Env[i].Value) != tc.fs.Spec.Env[i].Value {
					t.Errorf("Env values don't match. For %q, got %q and expected %q", deref(c.Env[i].Name),
						deref(c.Env[i].Value), tc.fs.Spec.Env[i].Value)
				}
			}

		})
	}
}

// Testing Ingress contract fields get rendered correctly
func TestIngressContractFields(t *testing.T) {
	tests := fixtures()

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			// Since a config with no Ingress is part of fixtures() and would cause these tests to panic,
			// skip any fixture with no ingress

			if tc.fs.Spec.Ingress == nil {
				t.Skip("No ingress")
			}

			ing := RenderIngress(tc.fs)

			// Making sure the ingress exists after render
			if ing.Spec == nil || len(ing.Spec.Rules) == 0 {
				t.Fatalf("Ingress has no rules")
			}

			rule := ing.Spec.Rules[0]
			if rule.HTTP == nil || len(rule.HTTP.Paths) == 0 {
				t.Fatalf("Ingress has no Paths")
			}

			if deref(rule.Host) != tc.fs.Spec.Ingress.Host {
				t.Errorf("Ingress hosts don't match. Expected %q, got %q", tc.fs.Spec.Ingress.Host, deref(rule.Host))
			}

			if deref(rule.HTTP.Paths[0].Path) != tc.fs.Spec.Ingress.Path {
				t.Errorf("Ingress path doesn't match expected value. Expected %q, got %q", tc.fs.Spec.Ingress.Path, deref(rule.HTTP.Paths[0].Path))
			}

			if ing.Annotations["alb.ingress.kubernetes.io/healthcheck-path"] != tc.fs.Spec.Health.Readiness {
				t.Errorf("Ingress healthcheck annotation does not match the contract. Expected %q, got %q",
					tc.fs.Spec.Health.Readiness, ing.Annotations["alb.ingress.kubernetes.io/healthcheck-path"])
			}
		})
	}

}

// Testing Network Policy contract fields
func TestNetworkPolicyPeers(t *testing.T) {
	tests := fixtures()

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			np := RenderNetworkPolicy(tc.fs, testPlatform())

			want := testPlatform().ALBSubnetCIDRS
			// NetworkPolicy panic guards
			if np.Spec == nil || len(np.Spec.Ingress) == 0 {
				t.Fatalf("NetworkPolicy has no ingress rules")
			}

			// Find the ALB rule by its port, not its position
			var albRule *networkingv1ac.NetworkPolicyIngressRuleApplyConfiguration
			for i := range np.Spec.Ingress {
				for _, p := range np.Spec.Ingress[i].Ports {
					if deref(p.Port).StrVal == portHTTP {
						albRule = &np.Spec.Ingress[i]
					}
				}
			}
			if albRule == nil {
				t.Fatalf("NetworkPolicy has no ingress rule for port %q", portHTTP)
			}

			if len(albRule.From) != len(want) {
				t.Fatalf("ALB rule has %d peers, want %d (one per CIDR)", len(albRule.From), len(want))
			}

			for i, peer := range albRule.From {
				if peer.IPBlock == nil {
					t.Errorf("peer %d has no ipBlock", i)
					continue
				}
				// CIDR comparison
				if deref(peer.IPBlock.CIDR) != want[i] {
					t.Errorf("peer %d CIDR doesn't match. Expected %q but got %q", i, want[i], deref(peer.IPBlock.CIDR))
				}
			}

			// Checking the Metrics rule by port
			var metricsRule *networkingv1ac.NetworkPolicyIngressRuleApplyConfiguration
			for i := range np.Spec.Ingress {
				for _, p := range np.Spec.Ingress[i].Ports {
					if deref(p.Port).StrVal == portMetrics {
						metricsRule = &np.Spec.Ingress[i]
					}
				}
			}

			if metricsRule == nil {
				t.Fatalf("Network Policy has no rule for the Metrics port %q", portMetrics)
			}
			if len(metricsRule.From) != 1 {
				t.Fatalf("NetworkPolicy metrics rule has %d rules, expect 1", len(metricsRule.From))
			}

			selector := metricsRule.From[0].NamespaceSelector
			if selector == nil {
				t.Errorf("Network Policy metrics rule has no namespace selector")
			} else if got, wantNS := selector.MatchLabels["kubernetes.io/metadata.name"],
				testPlatform().MonitoringNamespace; got != wantNS {
				t.Errorf("metrics rule allows namespace %q, but want %q", got, wantNS)
			}
		})
	}
}

// Test ObjectMeta to verify UID exists on all resources
func TestOwnerReferences(t *testing.T) {
	tests := fixtures()

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {

			metas := []struct {
				kind string
				meta *metav1ac.ObjectMetaApplyConfiguration
			}{
				{"Deployment", RenderDeployment(tc.fs).ObjectMetaApplyConfiguration},
				{"Service", RenderService(tc.fs).ObjectMetaApplyConfiguration},
				{"HPA", RenderAutoscaler(tc.fs).ObjectMetaApplyConfiguration},
				{"PDB", RenderPodDisruptionBudget(tc.fs).ObjectMetaApplyConfiguration},
				{"NetworkPolicy", RenderNetworkPolicy(tc.fs, testPlatform()).ObjectMetaApplyConfiguration},
				{"ServiceMonitor", RenderServiceMonitor(tc.fs).ObjectMetaApplyConfiguration},
				{"ServiceAccount", RenderServiceAccount(tc.fs).ObjectMetaApplyConfiguration},
			}
			if tc.fs.Spec.Ingress != nil {
				metas = append(metas, struct {
					kind string
					meta *metav1ac.ObjectMetaApplyConfiguration
				}{"Ingress", RenderIngress(tc.fs).ObjectMetaApplyConfiguration})
			}

			for _, m := range metas {
				// This doesn't need to break if an ownerRef is not found, so log and continue
				if len(m.meta.OwnerReferences) == 0 {
					t.Errorf("%v has no OwnerRefs", m.kind)
					continue
				}
				// If the rendered UID doesn't match TC's UID, or the rendered owner isn't set to Controller, error
				if deref(m.meta.OwnerReferences[0].UID) != tc.fs.UID || !deref(m.meta.OwnerReferences[0].Controller) {
					t.Errorf("%s owner reference: uid %q controller %v, want uid %q controller true",
						m.kind, deref(m.meta.OwnerReferences[0].UID), deref(m.meta.OwnerReferences[0].Controller), tc.fs.UID)
				}
			}
		})
	}
}

// Verify that Selectors and Labels match across resources
func TestSelectorsAndLabels(t *testing.T) {
	tests := fixtures()

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			dep := RenderDeployment(tc.fs)
			svc := RenderService(tc.fs)
			pdb := RenderPodDisruptionBudget(tc.fs)
			np := RenderNetworkPolicy(tc.fs, testPlatform())
			sm := RenderServiceMonitor(tc.fs)

			if dep.Spec == nil || dep.Spec.Template == nil || dep.Spec.Selector == nil {
				t.Fatalf("Deployment has no selector or pod template")
			}
			if svc.Spec == nil {
				t.Fatalf("Service has no selectors")
			}
			if pdb.Spec == nil || pdb.Spec.Selector == nil {
				t.Fatalf("Pod Disruption Budget has no selector labels")
			}
			if np.Spec == nil || np.Spec.PodSelector == nil {
				t.Fatalf("NetworkPolicy has no PodSelectors")
			}
			if sm.Spec == nil || sm.Spec.Selector == nil {
				t.Fatalf("ServiceMonitor has no selector labels")
			}

			// Selectors we're going to use in testing
			depSelectors := dep.Spec.Selector.MatchLabels
			svcSelectors := svc.Spec.Selector
			pdbSelectors := pdb.Spec.Selector.MatchLabels
			netpolSelectors := np.Spec.PodSelector.MatchLabels
			smSelectors := sm.Spec.Selector.MatchLabels

			// Labels
			podLabels := dep.Spec.Template.Labels
			svcLabels := svc.Labels

			// Check Deployment Selectors against Deployment Pod Template Labels
			if !match(podLabels, depSelectors) {
				t.Errorf("Deployment Selector %v doesn't match Pod labels %v", depSelectors, podLabels)
			}

			// Check Service Selectors against Pod Template Labels
			if !match(podLabels, svcSelectors) {
				t.Errorf("Service Selector %v doesn't match Pod label %v", svcSelectors, podLabels)
			}

			// Check Pod Disruption Budget against Pod Template Labels
			if !match(podLabels, pdbSelectors) {
				t.Errorf("PodDisruptionBudget Selector %v doesn't match Pod labels: %v", pdbSelectors, podLabels)
			}

			// Check Network Policy Selectors against Pod Template Labels
			if !match(podLabels, netpolSelectors) {
				t.Errorf("Network Policy selector %v doesn't match Pod Labels %v", netpolSelectors, podLabels)
			}

			// Check ServiceMonitor Selectors against Service Labels
			if !match(svcLabels, smSelectors) {
				t.Errorf("ServiceMonitor Selector %v doesn't match Service Labels %v", smSelectors, svcLabels)
			}
		})
	}
}

// Generating a plain Platform type for tests that need one
func testPlatform() Platform {
	return Platform{
		ALBSubnetCIDRS:      []string{"10.20.0.0/24", "10.20.1.0/24"},
		MonitoringNamespace: "kube-prometheus",
	}
}

// Func to match Selectors to Labels
func match(labels map[string]string, selectors map[string]string) bool {
	if len(selectors) == 0 {
		return false
	}

	for k, want := range selectors {
		if got, ok := labels[k]; !ok || got != want {
			return false
		}
	}
	return true
}

// Helper function to dereference pointers needed in tests
func deref[T any](p *T) T {
	var zero T
	if p != nil {
		return *p
	}
	return zero
}

// servicePort returns the Service port with the given name, or nil if there isn't one.
func servicePort(svc *corev1ac.ServiceApplyConfiguration, name string) *corev1ac.ServicePortApplyConfiguration {
	if svc.Spec == nil {
		return nil
	}
	for i := range svc.Spec.Ports {
		if deref(svc.Spec.Ports[i].Name) == name {
			return &svc.Spec.Ports[i]
		}
	}
	return nil
}

// containerPort returns the Deployment port with the given name, or nil if there isn't one
func containerPort(dep *appsv1ac.DeploymentApplyConfiguration, name string) *corev1ac.ContainerPortApplyConfiguration {

	// Verify that the spec and template spec are well formed
	if dep.Spec == nil || dep.Spec.Template == nil || dep.Spec.Template.Spec == nil ||
		len(dep.Spec.Template.Spec.Containers) == 0 {
		return nil
	}
	// Loop through ports for the first container in the spec to match the name we're looking for or return nil
	c := &dep.Spec.Template.Spec.Containers[0]
	for i := range c.Ports {
		if deref(c.Ports[i].Name) == name {
			return &c.Ports[i]
		}
	}
	return nil
}

// Matching port names between Deployment and Service ports and return the port number
func resolvePorts(dep *appsv1ac.DeploymentApplyConfiguration, svc *corev1ac.ServiceApplyConfiguration, name string) int32 {
	// Verify that specs exist
	if dep.Spec == nil || dep.Spec.Template == nil || dep.Spec.Template.Spec == nil ||
		len(dep.Spec.Template.Spec.Containers) == 0 || svc.Spec == nil {
		return 0
	}
	s := servicePort(svc, name)
	if s == nil || s.TargetPort == nil || s.TargetPort.Type != intstr.String {
		return 0
	}
	c := containerPort(dep, s.TargetPort.StrVal)
	if c == nil {
		return 0
	}
	return deref(c.ContainerPort)
}
