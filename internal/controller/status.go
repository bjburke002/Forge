package controller

import (
	"fmt"

	"github.com/bjburke002/forge/pkg/contract/v1alpha1"
	appsv1 "k8s.io/api/apps/v1"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func updateStatus(fs *v1alpha1.ForgeService, deploy *appsv1.Deployment, applyErr error) {
	fs.Status.ObservedGeneration = fs.Generation
	fs.Status.AvailableReplicas = deploy.Status.AvailableReplicas
	fs.Status.URL = ""
	if fs.Spec.Ingress != nil {
		fs.Status.URL = "https://" + fs.Spec.Ingress.Host + fs.Spec.Ingress.Path
	}

	// Ready condition
	// Start with ConditionFalse so we can check as things come up
	ready := metav1.Condition{
		Type:               v1alpha1.ConditionReady,
		Status:             metav1.ConditionFalse,
		Reason:             v1alpha1.ReasonReplicasUnavailable,
		Message:            fmt.Sprintf("%d of %d replicas available", deploy.Status.AvailableReplicas, fs.Spec.Scaling.MinReplicas),
		ObservedGeneration: fs.Generation,
	}
	// Checking status
	if deploy.Status.AvailableReplicas >= fs.Spec.Scaling.MinReplicas {
		ready.Status = metav1.ConditionTrue
		ready.Reason = v1alpha1.ReasonAvailable
	}
	meta.SetStatusCondition(&fs.Status.Conditions, ready)

	// Checking RollOut status
	// Set desired number of replicas
	desired := int32(1)
	if deploy.Spec.Replicas != nil {
		desired = *deploy.Spec.Replicas
	}

	// RollOut status conditionals
	rollingOut := deploy.Generation != deploy.Status.ObservedGeneration ||
		deploy.Status.UpdatedReplicas < desired ||
		deploy.Status.Replicas > deploy.Status.UpdatedReplicas ||
		deploy.Status.AvailableReplicas < deploy.Status.UpdatedReplicas

	// Progressing condition
	progressing := metav1.Condition{
		Type:               v1alpha1.ConditionProgressing,
		Status:             metav1.ConditionFalse,
		Reason:             v1alpha1.ReasonRolloutComplete,
		ObservedGeneration: fs.Generation,
	}
	if rollingOut {
		progressing.Status = metav1.ConditionTrue
		progressing.Reason = v1alpha1.ReasonRollingOut
		progressing.Message = fmt.Sprintf("RollOut in progress for: %s", deploy.Name)
	}
	meta.SetStatusCondition(&fs.Status.Conditions, progressing)

	// Checking for Degraded condition
	degraded := metav1.Condition{
		Type:               v1alpha1.ConditionDegraded,
		Status:             metav1.ConditionFalse,
		Reason:             v1alpha1.ReasonAsExpected,
		ObservedGeneration: fs.Generation,
	}
	// Catching manifest application errors first so that we don't hang
	if applyErr != nil {
		degraded.Status = metav1.ConditionTrue
		degraded.Reason = v1alpha1.ReasonApplyFailed
		degraded.Message = applyErr.Error()
	} else {
		for _, cond := range deploy.Status.Conditions {
			if cond.Type == appsv1.DeploymentProgressing {
				if cond.Reason == "ProgressDeadlineExceeded" {
					degraded.Status = metav1.ConditionTrue
					degraded.Reason = v1alpha1.ReasonRolloutStalled
					degraded.Message = cond.Message
				}
				break

			}
		}
	}
	meta.SetStatusCondition(&fs.Status.Conditions, degraded)
}
