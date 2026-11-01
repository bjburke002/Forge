package resources

import (
	"testing"

	"github.com/bjburke002/forge/pkg/contract/v1alpha1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/intstr"
	appsv1ac "k8s.io/client-go/applyconfigurations/apps/v1"

	corev1ac "k8s.io/client-go/applyconfigurations/core/v1"
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
