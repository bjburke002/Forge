package v1alpha1

// Condition types
const (
	ConditionReady       = "Ready"
	ConditionProgressing = "Progressing"
	ConditionDegraded    = "Degraded"
)

// Condition reasons
const (
	ReasonAvailable           = "Available"
	ReasonReplicasUnavailable = "ReplicasUnavailable"
	ReasonRollingOut          = "RollingOut"
	ReasonRolloutComplete     = "RolloutComplete"
	ReasonRolloutStalled      = "RolloutStalled"
	ReasonApplyFailed         = "ApplyFailed"
	ReasonAsExpected          = "AsExpected"
)
