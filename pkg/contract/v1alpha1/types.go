package v1alpha1

import (
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:resource:shortName=fsvc
// +kubebuilder:printcolumn:name="Host",type="string",JSONPath=".spec.ingress.host"
// +kubebuilder:printcolumn:name="Ready",type="string",JSONPath=`.status.conditions[?(@.type=="Ready")].status`
// +kubebuilder:printcolumn:name="Available",type="integer",JSONPath=".status.availableReplicas"
// +kubebuilder:printcolumn:name="Age",type="date",JSONPath=".metadata.creationTimestamp"
type ForgeService struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`
	Spec              ForgeServiceSpec   `json:"spec,omitempty"`
	Status            ForgeServiceStatus `json:"status,omitempty"`
}

// +kubebuilder:validation:XValidation:rule="self.observability.metricsPort != self.runtime.port",message="Observability must use a different port than the runtime"
type ForgeServiceSpec struct {
	// Owner is the team responsible for the service. Alerts get routed to this.
	// +required
	// +kubebuilder:validation:MinLength=1
	Owner string `json:"owner"`
	// Image is the container image that gets run
	// +required
	// +kubebuilder:validation:MinLength=1
	Image string `json:"image"`
	// Runtime describes how the container runs
	// +required
	Runtime RuntimeSpec `json:"runtime"`
	// Env is a list of environment variables passed to the manifest
	// +optional
	// +listType=map
	// +listMapKey=name
	Env []EnvVar `json:"env,omitempty"`
	// Scaling is responsible for the autoscaling requirements on the pod
	// +required
	Scaling ScalingSpec `json:"scaling"`
	// Resources defines resource reservations and limits
	// +required
	Resources ResourceSpec `json:"resources"`
	// Ingress defines any needed ingress routes for the ForgeService
	// +optional
	Ingress *IngressSpec `json:"ingress,omitempty"`
	// Health defines paths for liveness and readiness probes
	// +required
	Health HealthSpec `json:"health"`
	// Observability defines whether a Prometheus ServiceMonitor is created for this ForgeService, and pathing information related to it
	// +required
	Observability ObservabilitySpec `json:"observability"`
	// SLO optionally defines Service Level Objectives for availability and p99 latency for this ForgeService
	// +optional
	SLO *SLOSpec `json:"slo,omitempty"`
}

type ForgeServiceStatus struct {
	// Conditions are Ready, Progressing and Degraded
	// +optional
	// +listType=map
	// +listMapKey=type
	Conditions []metav1.Condition `json:"conditions,omitempty"`
	// ObservedGeneration is the spec generation last reconciled
	// +optional
	ObservedGeneration int64 `json:"observedGeneration,omitempty"`
	// AvailableReplicas states the current number of available replicas for the ForgeService
	// +optional
	AvailableReplicas int32 `json:"availableReplicas,omitempty"`
	// URL is the ForgeService's public address, set when an Ingress is defined
	// +optional
	URL string `json:"url,omitempty"`
	// ErrorBudget is an optional definition of the remaining Error Budget for the service
	// +optional
	ErrorBudget *ErrorBudget `json:"errorBudget,omitempty"`
}

// +kubebuilder:object:root=true
type ForgeServiceList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []ForgeService `json:"items"`
}

type RuntimeSpec struct {
	// Port is the port number the application listens on
	// +required
	// +kubebuilder:validation:Minimum=1
	// +kubebuilder:validation:Maximum=65535
	Port int32 `json:"port"`
}

type EnvVar struct {
	// Name is the name of the env
	// +required
	// +kubebuilder:validation:Pattern=`^[A-Za-z_][A-Za-z0-9_]*$`
	Name string `json:"name"`
	// Value is the value to set the Env
	// +optional
	Value string `json:"value,omitempty"`
}

type ErrorBudget struct {
	// RemainingMinutes defines how many minutes are left in your error budget
	// +required
	RemainingMinutes resource.Quantity `json:"remainingMinutes"`
	// ConsumedPercent is the percentage of Error Budget that has been used to this point
	// +required
	ConsumedPercent resource.Quantity `json:"consumedPercent"`
}
type IngressSpec struct {
	// Host is the hostname for the ingress route
	// +required
	// +kubebuilder:validation:MaxLength=256
	// +kubebuilder:validation:Pattern=`^(?:[a-z0-9](?:[a-z0-9-]{0,61}[a-z0-9])?\.)*[a-z0-9](?:[a-z0-9-]{0,61}[a-z0-9])?$`
	Host string `json:"host"`
	// Path is the ingress path aka the route to take once in the app. Default is '/'
	// +optional
	// +kubebuilder:validation:MaxLength=256
	// +kubebuilder:default="/"
	// +kubebuilder:validation:Pattern=`^/[A-Za-z0-9/._~-]*$`
	Path string `json:"path,omitempty"`
}

type HealthSpec struct {
	// Readiness is the path for the workload's readiness probe
	// +required
	// +kubebuilder:validation:MaxLength=256
	// +kubebuilder:validation:Pattern=`^/[A-Za-z0-9/._~-]*$`
	Readiness string `json:"readiness"`
	//Liveness is the path for the workload's liveness probe
	// +required
	// +kubebuilder:validation:MaxLength=256
	// +kubebuilder:validation:Pattern=`^/[A-Za-z0-9/._~-]*$`
	Liveness string `json:"liveness"`
}

type ObservabilitySpec struct {
	// MetricsPath defines the path used for Prometheus metrics scrapes
	// +optional
	// +kubebuilder:validation:MaxLength=256
	// +kubebuilder:validation:Pattern=`^/[A-Za-z0-9/._~-]*$`
	// +kubebuilder:default="/metrics"
	MetricsPath string `json:"metricsPath,omitempty"`
	// MetricsPort sets the port that Prometheus metrics scrapes happen on
	// +required
	// +kubebuilder:validation:Minimum=1
	// +kubebuilder:validation:Maximum=65535
	MetricsPort int32 `json:"metricsPort"`
}

// +kubebuilder:validation:XValidation:rule="quantity(string(self.limits.memory)).compareTo(quantity(string(self.requests.memory))) >= 0",message="memory limit must be at least the memory request"
type ResourceSpec struct {
	// Requests defines memory and CPU requests for the workload
	// +required
	Requests ResourceRequests `json:"requests"`
	// Limits defines memory limits for the workload
	// +required
	Limits ResourceLimits `json:"limits"`
}

type ResourceRequests struct {
	// CPU is the number of requested CPU for the workload
	// +required
	CPU resource.Quantity `json:"cpu"`
	// Memory is the requested amount of memory for the workload
	// +required
	Memory resource.Quantity `json:"memory"`
}

type ResourceLimits struct {
	// Memory defines the memory limit for the workload
	// +required
	Memory resource.Quantity `json:"memory"`
}

// +kubebuilder:validation:XValidation:rule="self.maxReplicas >= self.minReplicas",message="maxReplicas must be greater than or equal to minReplicas"
type ScalingSpec struct {
	// minReplicas defines the minimum number of replicas that can run at one time
	// +required
	// +kubebuilder:validation:Minimum=1
	MinReplicas int32 `json:"minReplicas"`
	// MaxReplicas defines the maximum number of replicas that can run at one time.
	// +required
	// +kubebuilder:validation:Minimum=1
	MaxReplicas int32 `json:"maxReplicas"`
	// TargetCPU defines the HPA autoscaling threshold, i.e. scaling on 70% utilization
	// +optional
	// +kubebuilder:default=75
	// +kubebuilder:validation:Minimum=1
	// +kubebuilder:validation:Maximum=100
	TargetCPU int32 `json:"targetCPUUtilization,omitempty"`
}

type SLOSpec struct {
	// Availability defines uptime requirements aka 99.999%
	// +required
	// +kubebuilder:validation:Pattern=`^[0-9]{1,2}(\.[0-9]{1,3})?$`
	Availability string `json:"availability"`
	// LatencyP99MS defines P99 latency tresholds for the ForgeService
	// +optional
	// +kubebuilder:validation:Minimum=1
	LatencyP99MS int32 `json:"latencyP99Ms,omitempty"`
}

func init() {
	SchemeBuilder.Register(&ForgeService{}, &ForgeServiceList{})
}
