package resources

import (
	"github.com/bjburke002/forge/pkg/contract/v1alpha1"
	corev1 "k8s.io/api/core/v1"
	networkingv1 "k8s.io/api/networking/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
	metav1ac "k8s.io/client-go/applyconfigurations/meta/v1"
	networkingv1ac "k8s.io/client-go/applyconfigurations/networking/v1"
)

func RenderNetworkPolicy(fs *v1alpha1.ForgeService, p Platform) *networkingv1ac.NetworkPolicyApplyConfiguration {
	return networkingv1ac.NetworkPolicy(fs.Name+"-networkpolicy", fs.Namespace).
		WithLabels(labels(fs)).
		WithOwnerReferences(ownerRef(fs)).
		WithSpec(networkingv1ac.NetworkPolicySpec().
			WithPodSelector(metav1ac.LabelSelector().
				WithMatchLabels(selectorLabels(fs)),
			).
			WithPolicyTypes(networkingv1.PolicyTypeIngress).
			WithIngress(networkingv1ac.NetworkPolicyIngressRule().
				WithFrom(netPolCIDRs(p.ALBSubnetCIDRS)...).
				WithPorts(networkingv1ac.NetworkPolicyPort().
					WithProtocol(corev1.ProtocolTCP).
					WithPort(intstr.FromString(portHTTP)),
				),
				networkingv1ac.NetworkPolicyIngressRule().
					WithFrom(networkingv1ac.NetworkPolicyPeer().
						WithNamespaceSelector(metav1ac.LabelSelector().
							WithMatchLabels(map[string]string{
								"kubernetes.io/metadata.name": p.MonitoringNamespace,
							}),
						),
					).
					WithPorts(networkingv1ac.NetworkPolicyPort().
						WithProtocol(corev1.ProtocolTCP).
						WithPort(intstr.FromString(portMetrics)),
					),
			),
		)
}
