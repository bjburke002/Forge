package controller

import (
	"context"
	"testing"
	"time"

	"github.com/bjburke002/forge/internal/controller/resources"
	"github.com/bjburke002/forge/pkg/contract/v1alpha1"
	monitoringv1 "github.com/prometheus-operator/prometheus-operator/pkg/apis/monitoring/v1"
	appsv1 "k8s.io/api/apps/v1"
	autoscalingv2 "k8s.io/api/autoscaling/v2"
	corev1 "k8s.io/api/core/v1"
	networkingv1 "k8s.io/api/networking/v1"
	policyv1 "k8s.io/api/policy/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// Verifies that a reconcile creates every child, each controlled by the ForgeService, and writes status.
func TestReconcileCreatesChildren(t *testing.T) {
	ctx := testContext(t)
	ns := newNamespace(t)

	fs := validFS(ns)
	if err := k8sClient.Create(ctx, fs); err != nil {
		t.Fatalf("creating ForgeService: %v", err)
	}

	reconcile(t, ctx, fs)

	for _, c := range children(fs.Name) {
		key := client.ObjectKey{Namespace: ns, Name: c.name}
		if err := k8sClient.Get(ctx, key, c.obj); err != nil {
			t.Errorf("%T %s: %v", c.obj, c.name, err)
			continue
		}
		if !metav1.IsControlledBy(c.obj, fs) {
			t.Errorf("%T %s is not controlled by the ForgeService", c.obj, c.name)
		}
	}

	got := getForgeService(t, ctx, fs)
	if got.Status.ObservedGeneration != got.Generation {
		t.Errorf("observedGeneration = %d, want %d", got.Status.ObservedGeneration, got.Generation)
	}
	if want := "https://payments.example.com/"; got.Status.URL != want {
		t.Errorf("url = %q, want %q", got.Status.URL, want)
	}
	expectCondition(t, got, v1alpha1.ConditionReady, metav1.ConditionFalse)
	expectCondition(t, got, v1alpha1.ConditionProgressing, metav1.ConditionTrue)
	expectCondition(t, got, v1alpha1.ConditionDegraded, metav1.ConditionFalse)
}

// Verifies that a deleted child is recreated on the next reconcile.
func TestReconcileRestoresDeletedChild(t *testing.T) {
	ctx := testContext(t)
	ns := newNamespace(t)

	fs := validFS(ns)
	if err := k8sClient.Create(ctx, fs); err != nil {
		t.Fatalf("creating ForgeService: %v", err)
	}
	reconcile(t, ctx, fs)

	deleted := []client.Object{
		&appsv1.Deployment{ObjectMeta: metav1.ObjectMeta{Name: fs.Name + "-deployment", Namespace: ns}},
		&networkingv1.NetworkPolicy{ObjectMeta: metav1.ObjectMeta{Name: fs.Name + "-networkpolicy", Namespace: ns}},
	}
	for _, obj := range deleted {
		if err := k8sClient.Delete(ctx, obj); err != nil {
			t.Fatalf("deleting %T %s: %v", obj, obj.GetName(), err)
		}
	}

	reconcile(t, ctx, fs)

	for _, obj := range deleted {
		if err := k8sClient.Get(ctx, client.ObjectKeyFromObject(obj), obj); err != nil {
			t.Errorf("%T %s was not restored: %v", obj, obj.GetName(), err)
		}
	}
}

// Verifies that a hand edit to a field Forge owns is reverted
func TestReconcileRevertsDrift(t *testing.T) {
	ctx := testContext(t)
	ns := newNamespace(t)

	fs := validFS(ns)
	if err := k8sClient.Create(ctx, fs); err != nil {
		t.Fatalf("creating ForgeService: %v", err)
	}
	reconcile(t, ctx, fs)

	var deploy appsv1.Deployment
	key := client.ObjectKey{Namespace: ns, Name: fs.Name + "-deployment"}
	if err := k8sClient.Get(ctx, key, &deploy); err != nil {
		t.Fatalf("getting Deployment: %v", err)
	}
	deploy.Spec.Template.Spec.Containers[0].Image = "registry.example.com/tampered:latest"
	if err := k8sClient.Update(ctx, &deploy); err != nil {
		t.Fatalf("editing Deployment: %v", err)
	}

	reconcile(t, ctx, fs)

	if err := k8sClient.Get(ctx, key, &deploy); err != nil {
		t.Fatalf("getting Deployment: %v", err)
	}
	if got := deploy.Spec.Template.Spec.Containers[0].Image; got != fs.Spec.Image {
		t.Errorf("image = %q after reconcile, want %q", got, fs.Spec.Image)
	}
}

// Verifies that removing the ingress block deletes the Ingress and clears status.url.
func TestReconcileRemovesIngress(t *testing.T) {
	ctx := testContext(t)
	ns := newNamespace(t)

	fs := validFS(ns)
	if err := k8sClient.Create(ctx, fs); err != nil {
		t.Fatalf("creating ForgeService: %v", err)
	}
	reconcile(t, ctx, fs)

	// The status write changed the resourceVersion, so update from a fresh copy.
	latest := getForgeService(t, ctx, fs)
	latest.Spec.Ingress = nil
	if err := k8sClient.Update(ctx, latest); err != nil {
		t.Fatalf("removing ingress block: %v", err)
	}

	reconcile(t, ctx, fs)

	var ing networkingv1.Ingress
	key := client.ObjectKey{Namespace: ns, Name: fs.Name + "-ingress"}
	if err := k8sClient.Get(ctx, key, &ing); !apierrors.IsNotFound(err) {
		t.Errorf("Ingress still present after removing the ingress block (err = %v)", err)
	}
	if got := getForgeService(t, ctx, fs); got.Status.URL != "" {
		t.Errorf("url = %q after removing the ingress block, want empty", got.Status.URL)
	}
}

// Each row breaks one rule in an otherwise valid ForgeService, so a rejection
// can only come from that rule.
func TestCRDValidation(t *testing.T) {
	tests := []struct {
		name   string
		mutate func(*v1alpha1.ForgeService)
	}{
		{"maxReplicas below minReplicas", func(fs *v1alpha1.ForgeService) { fs.Spec.Scaling.MaxReplicas = 1 }},
		{"metrics port equals app port", func(fs *v1alpha1.ForgeService) { fs.Spec.Observability.MetricsPort = 8080 }},
		{"memory limit below request", func(fs *v1alpha1.ForgeService) {
			fs.Spec.Resources.Limits.Memory = resource.MustParse("16Mi")
		}},
		{"ingress path without leading slash", func(fs *v1alpha1.ForgeService) { fs.Spec.Ingress.Path = "api" }},
	}

	ns := newNamespace(t)
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			ctx := testContext(t)
			fs := validFS(ns)
			tc.mutate(fs)

			err := k8sClient.Create(ctx, fs)
			if err == nil {
				t.Fatalf("ForgeService was accepted, want it rejected")
			}
			if !apierrors.IsInvalid(err) {
				t.Errorf("got %v, want an Invalid error", err)
			}
		})
	}
}

// children lists every resource a ForgeService with an ingress block renders,
// with an empty object of the right type for Get to fill in.
func children(name string) []struct {
	name string
	obj  client.Object
} {
	return []struct {
		name string
		obj  client.Object
	}{
		{name + "-sa", &corev1.ServiceAccount{}},
		{name + "-service", &corev1.Service{}},
		{name + "-deployment", &appsv1.Deployment{}},
		{name + "-hpa", &autoscalingv2.HorizontalPodAutoscaler{}},
		{name + "-pdb", &policyv1.PodDisruptionBudget{}},
		{name + "-networkpolicy", &networkingv1.NetworkPolicy{}},
		{name + "-servicemonitor", &monitoringv1.ServiceMonitor{}},
		{name + "-ingress", &networkingv1.Ingress{}},
	}
}

// expectCondition checks one condition's status. Reasons are covered by the
// updateStatus unit tests.
func expectCondition(t *testing.T, fs *v1alpha1.ForgeService, condType string, want metav1.ConditionStatus) {
	t.Helper()
	c := meta.FindStatusCondition(fs.Status.Conditions, condType)
	if c == nil {
		t.Errorf("%s: condition missing", condType)
		return
	}
	if c.Status != want {
		t.Errorf("%s: status %q, want %q", condType, c.Status, want)
	}
}

// reconcile runs one reconcile of fs and fails the test on error.
func reconcile(t *testing.T, ctx context.Context, fs *v1alpha1.ForgeService) {
	t.Helper()
	req := ctrl.Request{NamespacedName: client.ObjectKeyFromObject(fs)}
	if _, err := newReconciler().Reconcile(ctx, req); err != nil {
		t.Fatalf("reconcile: %v", err)
	}
}

// getForgeService returns the stored copy of fs, status included.
func getForgeService(t *testing.T, ctx context.Context, fs *v1alpha1.ForgeService) *v1alpha1.ForgeService {
	t.Helper()
	var got v1alpha1.ForgeService
	if err := k8sClient.Get(ctx, client.ObjectKeyFromObject(fs), &got); err != nil {
		t.Fatalf("getting ForgeService: %v", err)
	}
	return &got
}

// newReconciler returns a reconciler with the same platform values as the local run.
func newReconciler() *ForgeServiceReconciler {
	return &ForgeServiceReconciler{
		Client: k8sClient,
		Platform: resources.Platform{
			ALBSubnetCIDRS:      []string{"10.20.0.0/24", "10.20.1.0/24"},
			MonitoringNamespace: "kube-prometheus",
		},
	}
}

// testContext returns a context that ends with the test, or after 30 seconds
func testContext(t *testing.T) context.Context {
	t.Helper()
	ctx, cancel := context.WithTimeout(t.Context(), 30*time.Second)
	t.Cleanup(cancel)
	return ctx
}

// validFS returns a ForgeService the API server accepts. CRD defaults
// (ingress.path, metricsPath, targetCPUUtilization) are left for the server to fill in.
func validFS(namespace string) *v1alpha1.ForgeService {
	return &v1alpha1.ForgeService{
		ObjectMeta: metav1.ObjectMeta{Name: "payments", Namespace: namespace},
		Spec: v1alpha1.ForgeServiceSpec{
			Owner:   "payments-team",
			Image:   "registry.example.com/payments:test",
			Runtime: v1alpha1.RuntimeSpec{Port: 8080},
			Scaling: v1alpha1.ScalingSpec{MinReplicas: 2, MaxReplicas: 5},
			Resources: v1alpha1.ResourceSpec{
				Requests: v1alpha1.ResourceRequests{
					CPU:    resource.MustParse("25m"),
					Memory: resource.MustParse("32Mi"),
				},
				Limits: v1alpha1.ResourceLimits{Memory: resource.MustParse("128Mi")},
			},
			Ingress:       &v1alpha1.IngressSpec{Host: "payments.example.com"},
			Health:        v1alpha1.HealthSpec{Readiness: "/health/ready", Liveness: "/health/live"},
			Observability: v1alpha1.ObservabilitySpec{MetricsPort: 9090},
		},
	}
}

// newNamespace creates a namespace with a generated name
func newNamespace(t *testing.T) string {
	t.Helper()
	ns := &corev1.Namespace{ObjectMeta: metav1.ObjectMeta{GenerateName: "fsvc-test-"}}
	if err := k8sClient.Create(t.Context(), ns); err != nil {
		t.Fatalf("creating namespace: %v", err)
	}
	return ns.Name
}
