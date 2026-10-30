package resources

import (
	"github.com/bjburke002/forge/pkg/contract/v1alpha1"
	autoscalingv2 "k8s.io/api/autoscaling/v2"
	corev1 "k8s.io/api/core/v1"
	autoscalingv2ac "k8s.io/client-go/applyconfigurations/autoscaling/v2"
)

// RenderAutoscaler puts together an HPA spec based on values that get passed in from the ForgeService Spec + defaults in
// fields that aren't part of the contract
func RenderAutoscaler(fs *v1alpha1.ForgeService) *autoscalingv2ac.HorizontalPodAutoscalerApplyConfiguration {
	return autoscalingv2ac.HorizontalPodAutoscaler(fs.Name+"-hpa", fs.Namespace).
		WithLabels(labels(fs)).
		WithOwnerReferences(ownerRef(fs)).
		WithSpec(autoscalingv2ac.HorizontalPodAutoscalerSpec().
			WithScaleTargetRef(autoscalingv2ac.CrossVersionObjectReference().
				WithAPIVersion("apps/v1").
				WithName(fs.Name + "-deployment").
				WithKind("Deployment"),
			).
			WithMinReplicas(fs.Spec.Scaling.MinReplicas).
			WithMaxReplicas(fs.Spec.Scaling.MaxReplicas).
			WithMetrics(autoscalingv2ac.MetricSpec().
				WithType(autoscalingv2.ResourceMetricSourceType).
				WithResource(autoscalingv2ac.ResourceMetricSource().
					WithName(corev1.ResourceCPU).
					WithTarget(autoscalingv2ac.MetricTarget().
						WithType(autoscalingv2.UtilizationMetricType).
						WithAverageUtilization(fs.Spec.Scaling.TargetCPU),
					),
				),
			),
		)
}
