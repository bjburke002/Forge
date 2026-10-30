package resources

import (
	"github.com/bjburke002/forge/pkg/contract/v1alpha1"
	policyv1 "k8s.io/api/policy/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
	metav1ac "k8s.io/client-go/applyconfigurations/meta/v1"
	policyv1ac "k8s.io/client-go/applyconfigurations/policy/v1"
)

// RenderPodDisruptionBudget builds a PDB spec from fields that belong to the ForgeService spec + defaults that are not
// exposed from the ForgeService
func RenderPodDisruptionBudget(fs *v1alpha1.ForgeService) *policyv1ac.PodDisruptionBudgetApplyConfiguration {
	return policyv1ac.PodDisruptionBudget(fs.Name+"-pdb", fs.Namespace).
		WithLabels(labels(fs)).
		WithOwnerReferences(ownerRef(fs)).
		WithSpec(policyv1ac.PodDisruptionBudgetSpec().
			WithMaxUnavailable(intstr.FromInt32(1)).
			WithUnhealthyPodEvictionPolicy(policyv1.AlwaysAllow).
			WithSelector(metav1ac.LabelSelector().
				WithMatchLabels(selectorLabels(fs)),
			),
		)
}
