package resources

import (
	"github.com/bjburke002/forge/pkg/contract/v1alpha1"
	networkingv1 "k8s.io/api/networking/v1"
	networkingv1ac "k8s.io/client-go/applyconfigurations/networking/v1"
)

// RenderIngress builds an Ingress spec from fields that belong to the ForgeService spec + defaults that are not
// exposed from the ForgeService
func RenderIngress(fs *v1alpha1.ForgeService) *networkingv1ac.IngressApplyConfiguration {
	return networkingv1ac.Ingress(fs.Name+"-ingress", fs.Namespace).
		WithLabels(labels(fs)).
		WithOwnerReferences(ownerRef(fs)).
		WithAnnotations(ingressAnnotations(fs)).
		WithSpec(networkingv1ac.IngressSpec().
			WithIngressClassName("alb").
			WithRules(networkingv1ac.IngressRule().
				WithHost(fs.Spec.Ingress.Host).
				WithHTTP(networkingv1ac.HTTPIngressRuleValue().
					WithPaths(networkingv1ac.HTTPIngressPath().
						WithPath(fs.Spec.Ingress.Path).
						WithPathType(networkingv1.PathTypePrefix).
						WithBackend(networkingv1ac.IngressBackend().
							WithService(networkingv1ac.IngressServiceBackend().
								WithName(fs.Name + "-service").
								WithPort(networkingv1ac.ServiceBackendPort().WithName(portHTTP)),
							),
						),
					),
				),
			),
		)
}
