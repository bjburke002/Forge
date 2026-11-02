package controller

import (
	"errors"
	"testing"

	"github.com/bjburke002/forge/pkg/contract/v1alpha1"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type fsOption func(*v1alpha1.ForgeService)

// newFS defines a ForgeSvc with a standard config. Mirrors payment sample
func newFS(name string, opts ...fsOption) *v1alpha1.ForgeService {
	fs := &v1alpha1.ForgeService{
		ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: "forge-test", Generation: 3},
		Spec: v1alpha1.ForgeServiceSpec{
			Scaling: v1alpha1.ScalingSpec{MinReplicas: 2},
			Ingress: &v1alpha1.IngressSpec{Host: "payments.example.com", Path: "/"},
		},
	}

	for _, opt := range opts {
		opt(fs)
	}
	return fs
}

type deployOption func(*appsv1.Deployment)

// newDeploy returns a healthy Deployment. Options make it unhealthy in one way each.
func newDeploy(opts ...deployOption) *appsv1.Deployment {
	desired := int32(2)
	d := &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{Name: "payments-deployment", Namespace: "forge-test", Generation: 5},
		Spec:       appsv1.DeploymentSpec{Replicas: &desired},
		Status: appsv1.DeploymentStatus{
			ObservedGeneration: 5,
			Replicas:           2,
			UpdatedReplicas:    2,
			AvailableReplicas:  2,
		},
	}
	for _, opt := range opts {
		opt(d)
	}
	return d
}

// withAvailable sets how many pods are available (Ready long enough to count).
func withAvailable(n int32) deployOption {
	return func(d *appsv1.Deployment) { d.Status.AvailableReplicas = n }
}

// withUpdated sets how many pods run the current version, as during a rollout.
func withUpdated(n int32) deployOption {
	return func(d *appsv1.Deployment) { d.Status.UpdatedReplicas = n }
}

// withGenerationLag leaves the Deployment controller one change behind the spec.
func withGenerationLag() deployOption {
	return func(d *appsv1.Deployment) { d.Status.ObservedGeneration = d.Generation - 1 }
}

// withoutIngress clears the FS ingress
func withoutIngress() fsOption {
	return func(fs *v1alpha1.ForgeService) {
		fs.Spec.Ingress = nil
	}
}

// withStalled adds the condition the Deployment controller sets when a rollout
// exceeds its progress deadline.
func withStalled() deployOption {
	return func(d *appsv1.Deployment) {
		d.Status.Conditions = append(d.Status.Conditions, appsv1.DeploymentCondition{
			Type:    appsv1.DeploymentProgressing,
			Status:  corev1.ConditionFalse,
			Reason:  "ProgressDeadlineExceeded",
			Message: `ReplicaSet "payments-deployment-abc123" has timed out progressing.`,
		})
	}
}

// wantCondition is the expected status and reason for one condition
type wantCondition struct {
	status metav1.ConditionStatus
	reason string
}

func TestUpdateStatus(t *testing.T) {
	tests := []struct {
		name     string
		fs       *v1alpha1.ForgeService
		deploy   *appsv1.Deployment
		applyErr error

		ready       wantCondition
		progressing wantCondition
		degraded    wantCondition
		url         string
	}{
		{
			name:        "healthy",
			fs:          newFS("payments"),
			deploy:      newDeploy(),
			ready:       wantCondition{metav1.ConditionTrue, v1alpha1.ReasonAvailable},
			progressing: wantCondition{metav1.ConditionFalse, v1alpha1.ReasonRolloutComplete},
			degraded:    wantCondition{metav1.ConditionFalse, v1alpha1.ReasonAsExpected},
			url:         "https://payments.example.com/",
		},
		{
			name:        "too few available",
			fs:          newFS("payments"),
			deploy:      newDeploy(withAvailable(1)),
			ready:       wantCondition{metav1.ConditionFalse, v1alpha1.ReasonReplicasUnavailable},
			progressing: wantCondition{metav1.ConditionTrue, v1alpha1.ReasonRollingOut},
			degraded:    wantCondition{metav1.ConditionFalse, v1alpha1.ReasonAsExpected},
			url:         "https://payments.example.com/",
		},
		{
			name:        "rolling out, pods not updated",
			fs:          newFS("payments"),
			deploy:      newDeploy(withUpdated(1)),
			ready:       wantCondition{metav1.ConditionTrue, v1alpha1.ReasonAvailable},
			progressing: wantCondition{metav1.ConditionTrue, v1alpha1.ReasonRollingOut},
			degraded:    wantCondition{metav1.ConditionFalse, v1alpha1.ReasonAsExpected},
			url:         "https://payments.example.com/",
		},
		{
			name:        "rolling out, deployment controller behind",
			fs:          newFS("payments"),
			deploy:      newDeploy(withGenerationLag()),
			ready:       wantCondition{metav1.ConditionTrue, v1alpha1.ReasonAvailable},
			progressing: wantCondition{metav1.ConditionTrue, v1alpha1.ReasonRollingOut},
			degraded:    wantCondition{metav1.ConditionFalse, v1alpha1.ReasonAsExpected},
			url:         "https://payments.example.com/",
		},
		{
			name:        "stalled",
			fs:          newFS("payments"),
			deploy:      newDeploy(withStalled(), withUpdated(1)),
			ready:       wantCondition{metav1.ConditionTrue, v1alpha1.ReasonAvailable},
			progressing: wantCondition{metav1.ConditionTrue, v1alpha1.ReasonRollingOut},
			degraded:    wantCondition{metav1.ConditionTrue, v1alpha1.ReasonRolloutStalled},
			url:         "https://payments.example.com/",
		},
		{
			name:        "apply failed while stalled",
			fs:          newFS("payments"),
			deploy:      newDeploy(withStalled()),
			applyErr:    errors.New("simulated apply failure"),
			ready:       wantCondition{metav1.ConditionTrue, v1alpha1.ReasonAvailable},
			progressing: wantCondition{metav1.ConditionFalse, v1alpha1.ReasonRolloutComplete},
			degraded:    wantCondition{metav1.ConditionTrue, v1alpha1.ReasonApplyFailed},
			url:         "https://payments.example.com/",
		},
		{
			name:        "deployment not found",
			fs:          newFS("payments"),
			deploy:      &appsv1.Deployment{},
			ready:       wantCondition{metav1.ConditionFalse, v1alpha1.ReasonReplicasUnavailable},
			progressing: wantCondition{metav1.ConditionTrue, v1alpha1.ReasonRollingOut},
			degraded:    wantCondition{metav1.ConditionFalse, v1alpha1.ReasonAsExpected},
			url:         "https://payments.example.com/",
		},
		{
			name:        "without ingress",
			fs:          newFS("payments", withoutIngress()),
			deploy:      newDeploy(),
			ready:       wantCondition{metav1.ConditionTrue, v1alpha1.ReasonAvailable},
			progressing: wantCondition{metav1.ConditionFalse, v1alpha1.ReasonRolloutComplete},
			degraded:    wantCondition{metav1.ConditionFalse, v1alpha1.ReasonAsExpected},
			url:         "",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			updateStatus(tc.fs, tc.deploy, tc.applyErr)

			checkCondition(t, tc.fs, v1alpha1.ConditionReady, tc.ready)
			checkCondition(t, tc.fs, v1alpha1.ConditionProgressing, tc.progressing)
			checkCondition(t, tc.fs, v1alpha1.ConditionDegraded, tc.degraded)

			// Check URLs against one another
			if tc.fs.Status.URL != tc.url {
				t.Errorf("URLs don't match. Got %q, expected %q", tc.fs.Status.URL, tc.url)
			}
			// Verify that ObservedGeneration matches
			if tc.fs.Status.ObservedGeneration != tc.fs.Generation {
				t.Errorf("ObservedGeneration: %d but expected %d", tc.fs.Status.ObservedGeneration, tc.fs.Generation)
			}

			// Verify that AvailableReplicas match
			if tc.fs.Status.AvailableReplicas != tc.deploy.Status.AvailableReplicas {
				t.Errorf("Available replicas %d doesn't match available replicas in deploy: %d",
					tc.fs.Status.AvailableReplicas, tc.deploy.Status.AvailableReplicas)
			}
		})
	}
}

// Check underlying conditions for passed in status
func checkCondition(t *testing.T, fs *v1alpha1.ForgeService, condType string, want wantCondition) {
	t.Helper() // Flags this as a helper function in case tests fail

	c := meta.FindStatusCondition(fs.Status.Conditions, condType)

	if c == nil {
		t.Errorf("%s: condition missing", condType)
		return
	}

	if c.Status != want.status {
		t.Errorf("%s: status %q, want %q", condType, c.Status, want.status)
	}
	if c.Reason != want.reason {
		t.Errorf("%s: reason %q, want %q", condType, c.Reason, want.reason)
	}
}
