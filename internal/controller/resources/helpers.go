package resources

import (
	"github.com/bjburke002/forge/pkg/contract/v1alpha1"
	corev1ac "k8s.io/client-go/applyconfigurations/core/v1"
	metav1ac "k8s.io/client-go/applyconfigurations/meta/v1"
	networkingv1ac "k8s.io/client-go/applyconfigurations/networking/v1"
)

// Defining a struct to allow CIDR ranges and Prom namespace to be passed in at call time
type Platform struct {
	ALBSubnetCIDRS      []string
	MonitoringNamespace string
}

// Controller reference information
func ownerRef(fs *v1alpha1.ForgeService) *metav1ac.OwnerReferenceApplyConfiguration {
	// should return "forge.dev/v1alpha1"
	return metav1ac.OwnerReference().
		WithAPIVersion(v1alpha1.GroupVersion.String()).
		WithKind("ForgeService").
		WithName(fs.Name).
		WithUID(fs.UID).
		WithController(true).
		WithBlockOwnerDeletion(true)
}

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

// Helper to map Ingress annotations
func ingressAnnotations(fs *v1alpha1.ForgeService) map[string]string {
	return map[string]string{
		"alb.ingress.kubernetes.io/group.name":       ingressGroupName,
		"alb.ingress.kubernetes.io/scheme":           "internet-facing",
		"alb.ingress.kubernetes.io/target-type":      "ip",
		"alb.ingress.kubernetes.io/listen-ports":     `[{"HTTP":80}, {"HTTPS":443}]`,
		"alb.ingress.kubernetes.io/ssl-redirect":     "443",
		"alb.ingress.kubernetes.io/healthcheck-path": fs.Spec.Health.Readiness,
	}
}

// Helper to map CIDR ranges for the NetworkPolicy spec and any additional uses
func netPolCIDRs(cidrs []string) []*networkingv1ac.NetworkPolicyPeerApplyConfiguration {
	peers := make([]*networkingv1ac.NetworkPolicyPeerApplyConfiguration, 0, len(cidrs))
	for _, c := range cidrs {
		peers = append(peers, networkingv1ac.NetworkPolicyPeer().
			WithIPBlock(networkingv1ac.IPBlock().WithCIDR(c)))
	}
	return peers
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
	FieldOwner                        = "forge-controller"
	ingressGroupName                  = "forge-shared"
)

// Helper function to store spec environment variables
func envVars(fs *v1alpha1.ForgeService) []*corev1ac.EnvVarApplyConfiguration {
	env := make([]*corev1ac.EnvVarApplyConfiguration, 0, len(fs.Spec.Env))
	for _, e := range fs.Spec.Env {
		env = append(env, corev1ac.EnvVar().WithName(e.Name).WithValue(e.Value))
	}
	return env
}
