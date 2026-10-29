package resources

import (
	"github.com/bjburke002/forge/pkg/contract/v1alpha1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
	corev1ac "k8s.io/client-go/applyconfigurations/core/v1"
)

func RenderService(fs *v1alpha1.ForgeService) *corev1ac.ServiceApplyConfiguration {
	return corev1ac.Service(fs.Name+"-service", fs.Namespace).
		WithLabels(labels(fs)).
		WithOwnerReferences(ownerRef(fs)).
		WithSpec(corev1ac.ServiceSpec().
			WithSelector(selectorLabels(fs)).
			WithType(corev1.ServiceTypeClusterIP).
			WithPorts(
				corev1ac.ServicePort().
					WithName(portHTTP).
					WithProtocol(corev1.ProtocolTCP).
					WithPort(servicePortHTTP).
					WithTargetPort(intstr.FromString(portHTTP)),
				corev1ac.ServicePort().
					WithName(portMetrics).
					WithProtocol(corev1.ProtocolTCP).
					WithPort(fs.Spec.Observability.MetricsPort).
					WithTargetPort(intstr.FromString(portMetrics)),
			),
		)
}
