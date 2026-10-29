package resources

import (
	"github.com/bjburke002/forge/pkg/contract/v1alpha1"
	corev1 "k8s.io/api/core/v1"
)

// Helper to map labels
func labels(fs *v1alpha1.ForgeService) map[string]string {
	l := selectorLabels(fs)
	l["app.kubernetes.io/name"] = fs.Name
	l["app.kubernetes.io/managed-by"] = "forge"
	return l
}

// Helper to map selectorLabels
func selectorLabels(fs *v1alpha1.ForgeService) map[string]string {
	return map[string]string{
		"app": fs.Name,
	}
}

// Names that are going to be re-used through multiple renderers
const (
	portHTTP                          = "http"
	portMetrics                       = "metrics"
	servicePortHTTP                   = 80
	livenessProbePeriodSeconds        = 10
	livenessProbeTimeoutSeconds       = 5
	livenessProbeInitialDelaySeconds  = 10
	livenessProbeFailureThreshold     = 3
	readinessProbeInitialDelaySeconds = 2
	readinessProbePeriodSeconds       = 10
	readinessProbeTimeoutSeconds      = 5
	readinessProbeFailureThreshold    = 3
	tempVolumeName                    = "tmp"
)

// Helper function to store spec environment variables
func envVars(fs *v1alpha1.ForgeService) []corev1.EnvVar {
	env := make([]corev1.EnvVar, 0, len(fs.Spec.Env))
	for _, e := range fs.Spec.Env {
		env = append(env, corev1.EnvVar{Name: e.Name, Value: e.Value})
	}
	return env
}
