package resources

import (
	"github.com/bjburke002/forge/pkg/contract/v1alpha1"
	corev1ac "k8s.io/client-go/applyconfigurations/core/v1"
)

func RenderServiceAccount(fs *v1alpha1.ForgeService) *corev1ac.ServiceAccountApplyConfiguration {
	return corev1ac.ServiceAccount(fs.Name+"-sa", fs.Namespace).
		WithLabels(labels(fs)).
		WithOwnerReferences(ownerRef(fs)).
		WithAutomountServiceAccountToken(false)
}
