package resources

import (
	"github.com/bjburke002/forge/pkg/contract/v1alpha1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
)

func RenderService(fs *v1alpha1.ForgeService) *corev1.Service {
	return &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:      fs.Name + "-service",
			Namespace: fs.Namespace,
			Labels:    labels(fs),
		},
		Spec: corev1.ServiceSpec{
			Selector: selectorLabels(fs),
			Type:     corev1.ServiceTypeClusterIP,
			Ports: []corev1.ServicePort{
				{
					// Port names sourced from labels.go
					Name:       portHTTP,
					Port:       servicePortHTTP,
					TargetPort: intstr.FromString(portHTTP),
					Protocol:   corev1.ProtocolTCP,
				},
				{
					Name:       portMetrics,
					Port:       fs.Spec.Observability.MetricsPort,
					TargetPort: intstr.FromString(portMetrics),
					Protocol:   corev1.ProtocolTCP,
				},
			},
		},
	}
}
