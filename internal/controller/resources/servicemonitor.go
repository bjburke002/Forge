package resources

import (
	"github.com/bjburke002/forge/pkg/contract/v1alpha1"
	monitoringv1 "github.com/prometheus-operator/prometheus-operator/pkg/apis/monitoring/v1"
	monitoringv1ac "github.com/prometheus-operator/prometheus-operator/pkg/client/applyconfiguration/monitoring/v1"
	metav1ac "k8s.io/client-go/applyconfigurations/meta/v1"
)

func RenderServiceMonitor(fs *v1alpha1.ForgeService) *monitoringv1ac.ServiceMonitorApplyConfiguration {
	return monitoringv1ac.ServiceMonitor(fs.Name+"-servicemonitor", fs.Namespace).
		WithLabels(labels(fs)).
		WithOwnerReferences(ownerRef(fs)).
		WithSpec(monitoringv1ac.ServiceMonitorSpec().
			WithSelector(metav1ac.LabelSelector().
				WithMatchLabels(selectorLabels(fs)),
			).
			WithEndpoints(monitoringv1ac.Endpoint().
				WithPort(portMetrics).
				WithPath(fs.Spec.Observability.MetricsPath).
				WithInterval(monitoringv1.Duration("15s")).
				WithScrapeTimeout(monitoringv1.Duration("10s")),
			),
		)
}
