package resources

import (
	"github.com/bjburke002/forge/pkg/contract/v1alpha1"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
	"k8s.io/utils/ptr"
)

func RenderDeployment(fs *v1alpha1.ForgeService) *appsv1.Deployment {
	return &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      fs.Name + "-deployment",
			Namespace: fs.Namespace,
			Labels:    labels(fs),
		},
		Spec: appsv1.DeploymentSpec{
			Selector: &metav1.LabelSelector{
				MatchLabels: selectorLabels(fs),
			},
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: labels(fs),
				},
				Spec: corev1.PodSpec{
					ServiceAccountName: fs.Name + "-sa",
					SecurityContext: &corev1.PodSecurityContext{
						RunAsNonRoot: ptr.To(true),
						RunAsUser:    ptr.To(int64(65532)),
						SeccompProfile: &corev1.SeccompProfile{
							Type: corev1.SeccompProfileTypeRuntimeDefault,
						},
					},
					TopologySpreadConstraints: []corev1.TopologySpreadConstraint{
						{
							MaxSkew:           1,
							TopologyKey:       "kubernetes.io/hostname",
							WhenUnsatisfiable: corev1.ScheduleAnyway,
							LabelSelector: &metav1.LabelSelector{
								MatchLabels: selectorLabels(fs),
							},
							MatchLabelKeys: []string{"pod-template-hash"},
						},
					},
					Volumes: []corev1.Volume{
						{
							Name: tempVolumeName,
							VolumeSource: corev1.VolumeSource{
								EmptyDir: &corev1.EmptyDirVolumeSource{},
							},
						},
					},
					Containers: []corev1.Container{
						{
							Name:  fs.Name,
							Image: fs.Spec.Image,
							Ports: []corev1.ContainerPort{
								{
									Name:          portHTTP,
									ContainerPort: fs.Spec.Runtime.Port,
									Protocol:      corev1.ProtocolTCP,
								},
								{
									Name:          portMetrics,
									ContainerPort: fs.Spec.Observability.MetricsPort,
									Protocol:      corev1.ProtocolTCP,
								},
							},
							Resources: corev1.ResourceRequirements{

								Requests: corev1.ResourceList{
									corev1.ResourceCPU:    fs.Spec.Resources.Requests.CPU,
									corev1.ResourceMemory: fs.Spec.Resources.Requests.Memory,
								},
								Limits: corev1.ResourceList{
									corev1.ResourceMemory: fs.Spec.Resources.Limits.Memory,
								},
							},
							LivenessProbe: &corev1.Probe{
								ProbeHandler: corev1.ProbeHandler{
									HTTPGet: &corev1.HTTPGetAction{
										Path: fs.Spec.Health.Liveness,
										Port: intstr.FromString(portHTTP),
									},
								},
								InitialDelaySeconds: livenessProbeInitialDelaySeconds,
								PeriodSeconds:       livenessProbePeriodSeconds,
								TimeoutSeconds:      livenessProbeTimeoutSeconds,
								FailureThreshold:    livenessProbeFailureThreshold,
							},
							ReadinessProbe: &corev1.Probe{
								ProbeHandler: corev1.ProbeHandler{
									HTTPGet: &corev1.HTTPGetAction{
										Path: fs.Spec.Health.Readiness,
										Port: intstr.FromString(portHTTP),
									},
								},
								InitialDelaySeconds: readinessProbeInitialDelaySeconds,
								PeriodSeconds:       readinessProbePeriodSeconds,
								TimeoutSeconds:      readinessProbeTimeoutSeconds,
								FailureThreshold:    readinessProbeFailureThreshold,
							},
							SecurityContext: &corev1.SecurityContext{
								AllowPrivilegeEscalation: ptr.To(false),
								ReadOnlyRootFilesystem:   ptr.To(true),
								Capabilities: &corev1.Capabilities{
									Drop: []corev1.Capability{
										"ALL",
									},
								},
							},
							Lifecycle: &corev1.Lifecycle{
								PreStop: &corev1.LifecycleHandler{
									Sleep: &corev1.SleepAction{
										Seconds: 15,
									},
								},
							},
							VolumeMounts: []corev1.VolumeMount{
								{
									Name:      tempVolumeName,
									MountPath: "/tmp",
								},
							},
							Env: envVars(fs),
						},
					},
				},
			},
		},
	}
}
