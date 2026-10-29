package resources

import (
	"github.com/bjburke002/forge/pkg/contract/v1alpha1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
	appsv1ac "k8s.io/client-go/applyconfigurations/apps/v1"
	corev1ac "k8s.io/client-go/applyconfigurations/core/v1"
	metav1ac "k8s.io/client-go/applyconfigurations/meta/v1"
)

func RenderDeployment(fs *v1alpha1.ForgeService) *appsv1ac.DeploymentApplyConfiguration {

	// Setting up the container spec, will pass this to the pod spec, then returning a full render
	container := corev1ac.Container().
		WithName(fs.Name).
		WithImage(fs.Spec.Image).
		WithPorts(
			corev1ac.ContainerPort().
				WithName(portHTTP).
				WithProtocol(corev1.ProtocolTCP).
				WithContainerPort(fs.Spec.Runtime.Port),
			corev1ac.ContainerPort().
				WithName(portMetrics).
				WithProtocol(corev1.ProtocolTCP).
				WithContainerPort(fs.Spec.Observability.MetricsPort),
		).
		WithResources(corev1ac.ResourceRequirements().
			WithRequests(corev1.ResourceList{
				corev1.ResourceCPU:    fs.Spec.Resources.Requests.CPU,
				corev1.ResourceMemory: fs.Spec.Resources.Requests.Memory,
			}).
			WithLimits(corev1.ResourceList{
				corev1.ResourceMemory: fs.Spec.Resources.Limits.Memory,
			}),
		).
		WithLivenessProbe(
			corev1ac.Probe().
				WithHTTPGet(
					corev1ac.HTTPGetAction().
						WithPath(fs.Spec.Health.Liveness).
						WithPort(intstr.FromString(portHTTP)),
				).
				WithInitialDelaySeconds(livenessProbeInitialDelaySeconds).
				WithPeriodSeconds(livenessProbePeriodSeconds).
				WithTimeoutSeconds(livenessProbeTimeoutSeconds).
				WithFailureThreshold(livenessProbeFailureThreshold),
		).
		WithReadinessProbe(
			corev1ac.Probe().
				WithHTTPGet(
					corev1ac.HTTPGetAction().
						WithPath(fs.Spec.Health.Readiness).
						WithPort(intstr.FromString(portHTTP)),
				).
				WithInitialDelaySeconds(readinessProbeInitialDelaySeconds).
				WithPeriodSeconds(readinessProbePeriodSeconds).
				WithTimeoutSeconds(readinessProbeTimeoutSeconds).
				WithFailureThreshold(readinessProbeFailureThreshold),
		).
		WithSecurityContext(corev1ac.SecurityContext().
			WithAllowPrivilegeEscalation(false).
			WithReadOnlyRootFilesystem(true).
			WithCapabilities(corev1ac.Capabilities().
				WithDrop(corev1.Capability("ALL")),
			),
		).
		WithLifecycle(corev1ac.Lifecycle().
			WithPreStop(corev1ac.LifecycleHandler().
				WithSleep(corev1ac.SleepAction().
					WithSeconds(15),
				),
			),
		).
		WithVolumeMounts(corev1ac.VolumeMount().
			WithMountPath("/tmp").
			WithName(tempVolumeName),
		).
		WithEnv(envVars(fs)...)

	// Setting up the pod spec. Uses the container spec above
	// Breaking them up mostly to keep this readable
	podSpec := corev1ac.PodSpec().
		WithServiceAccountName(fs.Name + "-sa").
		WithSecurityContext(corev1ac.PodSecurityContext().
			WithRunAsNonRoot(true).
			WithRunAsUser(65532).
			WithSeccompProfile(corev1ac.SeccompProfile().
				WithType(corev1.SeccompProfileTypeRuntimeDefault),
			),
		).
		WithTopologySpreadConstraints(
			corev1ac.TopologySpreadConstraint().
				WithMaxSkew(1).
				WithTopologyKey("kubernetes.io/hostname").
				WithWhenUnsatisfiable(corev1.ScheduleAnyway).
				WithLabelSelector(metav1ac.LabelSelector().
					WithMatchLabels(selectorLabels(fs))).
				WithMatchLabelKeys("pod-template-hash"),
		).
		WithVolumes(corev1ac.Volume().
			WithName(tempVolumeName).
			WithEmptyDir(corev1ac.EmptyDirVolumeSource()),
		).
		WithContainers(container)

	// Returns a Deployment Spec
	return appsv1ac.Deployment(fs.Name+"-deployment", fs.Namespace).
		WithLabels(labels(fs)).
		WithOwnerReferences(ownerRef(fs)).
		WithSpec(appsv1ac.DeploymentSpec().
			WithSelector(metav1ac.LabelSelector().WithMatchLabels(selectorLabels(fs))).
			WithTemplate(corev1ac.PodTemplateSpec().
				WithLabels(labels(fs)).
				WithSpec(podSpec)))
}
