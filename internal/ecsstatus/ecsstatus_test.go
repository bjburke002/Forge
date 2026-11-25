package ecsstatus

import (
	"reflect"
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	cftypes "github.com/aws/aws-sdk-go-v2/service/cloudformation/types"
	ecstypes "github.com/aws/aws-sdk-go-v2/service/ecs/types"
	"github.com/bjburke002/forge/internal/status"
)

const (
	testURL         = "https://billing-dev.forgeappservices.com"
	testInternalURL = "http://billing"
	testImage       = "241197444395.dkr.ecr.us-east-1.amazonaws.com/forge/payments-api:abc123"
)

func stack(st cftypes.StackStatus, withURL bool) *cftypes.Stack {
	s := &cftypes.Stack{StackStatus: st}
	if withURL {
		s.Outputs = []cftypes.Output{{OutputKey: aws.String("URL"), OutputValue: aws.String(testURL)}}
	}
	return s
}

func withOutput(s *cftypes.Stack, key, value string) *cftypes.Stack {
	s.Outputs = append(s.Outputs, cftypes.Output{OutputKey: aws.String(key), OutputValue: aws.String(value)})
	return s
}

// service has an old ACTIVE deployment listed after the PRIMARY one, as during a rollout.
func service(running, desired int32, state ecstypes.DeploymentRolloutState, reason string) *ecstypes.Service {
	return &ecstypes.Service{
		RunningCount: running,
		DesiredCount: desired,
		Deployments: []ecstypes.Deployment{
			{Status: aws.String("PRIMARY"), RolloutState: state, RolloutStateReason: aws.String(reason)},
			{Status: aws.String("ACTIVE"), RolloutState: ecstypes.DeploymentRolloutStateCompleted},
		},
	}
}

var taskDef = &ecstypes.TaskDefinition{
	ContainerDefinitions: []ecstypes.ContainerDefinition{{Image: aws.String(testImage)}},
}

// deployment is the newest service deployment; stage is empty for a rolling deploy.
func deployment(st ecstypes.ServiceDeploymentStatus, stage, reason string) *ecstypes.ServiceDeployment {
	d := &ecstypes.ServiceDeployment{Status: st, LifecycleStage: ecstypes.ServiceDeploymentLifecycleStage(stage)}
	if reason != "" {
		d.StatusReason = aws.String(reason)
	}
	return d
}

func TestResult(t *testing.T) {
	tests := []struct {
		name       string
		stack      *cftypes.Stack
		service    *ecstypes.Service
		taskDef    *ecstypes.TaskDefinition
		deployment *ecstypes.ServiceDeployment
		want       status.Result
	}{
		{
			name: "not deployed",
			want: status.Result{Environment: "dev", Backend: "ecs"},
		},
		{
			name:  "stack without a service",
			stack: stack(cftypes.StackStatusCreateInProgress, true),
			want: status.Result{Environment: "dev", Backend: "ecs", Deployed: true,
				Sync: "CREATE_IN_PROGRESS", Health: "service missing", URL: testURL},
		},
		{
			name:    "healthy",
			stack:   stack(cftypes.StackStatusUpdateComplete, true),
			service: service(1, 1, ecstypes.DeploymentRolloutStateCompleted, ""),
			taskDef: taskDef,
			want: status.Result{Environment: "dev", Backend: "ecs", Deployed: true,
				Sync: "UPDATE_COMPLETE", Health: "COMPLETED", Ready: "True", Available: 1,
				Image: testImage, URL: testURL},
		},
		{
			name:    "rolling out",
			stack:   stack(cftypes.StackStatusUpdateInProgress, true),
			service: service(2, 1, ecstypes.DeploymentRolloutStateInProgress, ""),
			taskDef: taskDef,
			want: status.Result{Environment: "dev", Backend: "ecs", Deployed: true,
				Sync: "UPDATE_IN_PROGRESS", Health: "IN_PROGRESS", Available: 2,
				Image: testImage, URL: testURL},
		},
		{
			name:    "rolled back",
			stack:   stack(cftypes.StackStatusUpdateRollbackComplete, true),
			service: service(1, 1, ecstypes.DeploymentRolloutStateFailed, "circuit breaker triggered"),
			taskDef: taskDef,
			want: status.Result{Environment: "dev", Backend: "ecs", Deployed: true,
				Sync: "UPDATE_ROLLBACK_COMPLETE", Health: "FAILED", Available: 1,
				Image: testImage, URL: testURL, Conditions: []string{"circuit breaker triggered"}},
		},
		{
			name:    "worker without ingress",
			stack:   stack(cftypes.StackStatusCreateComplete, false),
			service: service(1, 1, ecstypes.DeploymentRolloutStateCompleted, ""),
			taskDef: taskDef,
			want: status.Result{Environment: "dev", Backend: "ecs", Deployed: true,
				Sync: "CREATE_COMPLETE", Health: "COMPLETED", Ready: "True", Available: 1,
				Image: testImage},
		},

		// The newest service deployment adjusts the baseline from the PRIMARY deployment.
		{
			name:       "successful deployment keeps the baseline",
			stack:      stack(cftypes.StackStatusUpdateComplete, true),
			service:    service(1, 1, ecstypes.DeploymentRolloutStateCompleted, ""),
			taskDef:    taskDef,
			deployment: deployment(ecstypes.ServiceDeploymentStatusSuccessful, "", ""),
			want: status.Result{Environment: "dev", Backend: "ecs", Deployed: true,
				Sync: "UPDATE_COMPLETE", Health: "COMPLETED", Ready: "True", Available: 1,
				Image: testImage, URL: testURL},
		},
		{
			// Old and new tasks both run mid-canary, so counts can match without the deploy being done.
			name:       "canary in progress",
			stack:      stack(cftypes.StackStatusUpdateInProgress, true),
			service:    service(2, 2, ecstypes.DeploymentRolloutStateCompleted, ""),
			taskDef:    taskDef,
			deployment: deployment(ecstypes.ServiceDeploymentStatusInProgress, "PRODUCTION_TRAFFIC_SHIFT", ""),
			want: status.Result{Environment: "dev", Backend: "ecs", Deployed: true,
				Sync: "UPDATE_IN_PROGRESS", Health: "Progressing", Available: 2,
				Image: testImage, URL: testURL, Conditions: []string{"Canary: PRODUCTION_TRAFFIC_SHIFT"}},
		},
		{
			name:       "pending deployment",
			stack:      stack(cftypes.StackStatusUpdateInProgress, true),
			service:    service(1, 1, ecstypes.DeploymentRolloutStateCompleted, ""),
			taskDef:    taskDef,
			deployment: deployment(ecstypes.ServiceDeploymentStatusPending, "", ""),
			want: status.Result{Environment: "dev", Backend: "ecs", Deployed: true,
				Sync: "UPDATE_IN_PROGRESS", Health: "Progressing", Available: 1,
				Image: testImage, URL: testURL, Conditions: []string{"Deploying"}},
		},
		{
			name:       "rolling deploy in progress (no stages)",
			stack:      stack(cftypes.StackStatusUpdateInProgress, false),
			service:    service(2, 1, ecstypes.DeploymentRolloutStateInProgress, ""),
			taskDef:    taskDef,
			deployment: deployment(ecstypes.ServiceDeploymentStatusInProgress, "", ""),
			want: status.Result{Environment: "dev", Backend: "ecs", Deployed: true,
				Sync: "UPDATE_IN_PROGRESS", Health: "Progressing", Available: 2,
				Image: testImage, Conditions: []string{"Deploying"}},
		},
		{
			// Rollbacks reuse stage names, so only the reason is shown; it replaces the rollout reason.
			name:       "rolling back",
			stack:      stack(cftypes.StackStatusUpdateInProgress, true),
			service:    service(2, 2, ecstypes.DeploymentRolloutStateFailed, "circuit breaker triggered"),
			taskDef:    taskDef,
			deployment: deployment(ecstypes.ServiceDeploymentStatusRollbackInProgress, "PRODUCTION_TRAFFIC_SHIFT", "lifecycle hook failed"),
			want: status.Result{Environment: "dev", Backend: "ecs", Deployed: true,
				Sync: "UPDATE_IN_PROGRESS", Health: "Degraded", Available: 2,
				Image: testImage, URL: testURL, Conditions: []string{"Rolling back: lifecycle hook failed"}},
		},
		{
			name:       "rolling back without a reason",
			stack:      stack(cftypes.StackStatusUpdateInProgress, true),
			service:    service(1, 1, ecstypes.DeploymentRolloutStateCompleted, ""),
			taskDef:    taskDef,
			deployment: deployment(ecstypes.ServiceDeploymentStatusRollbackInProgress, "", ""),
			want: status.Result{Environment: "dev", Backend: "ecs", Deployed: true,
				Sync: "UPDATE_IN_PROGRESS", Health: "Degraded", Available: 1,
				Image: testImage, URL: testURL, Conditions: []string{"Rolling back"}},
		},
		{
			// The old release serves normally: baseline kept, reason added.
			name:       "rolled back successfully",
			stack:      stack(cftypes.StackStatusUpdateRollbackComplete, true),
			service:    service(1, 1, ecstypes.DeploymentRolloutStateCompleted, ""),
			taskDef:    taskDef,
			deployment: deployment(ecstypes.ServiceDeploymentStatusRollbackSuccessful, "", "alarm forge-billing-dev-5xx in ALARM"),
			want: status.Result{Environment: "dev", Backend: "ecs", Deployed: true,
				Sync: "UPDATE_ROLLBACK_COMPLETE", Health: "COMPLETED", Ready: "True", Available: 1,
				Image: testImage, URL: testURL, Conditions: []string{"Last deploy rolled back: alarm forge-billing-dev-5xx in ALARM"}},
		},
		{
			name:       "rollback failed",
			stack:      stack(cftypes.StackStatusUpdateRollbackFailed, true),
			service:    service(1, 1, ecstypes.DeploymentRolloutStateCompleted, ""),
			taskDef:    taskDef,
			deployment: deployment(ecstypes.ServiceDeploymentStatusRollbackFailed, "", "tasks failed to start"),
			want: status.Result{Environment: "dev", Backend: "ecs", Deployed: true,
				Sync: "UPDATE_ROLLBACK_FAILED", Health: "Degraded", Available: 1,
				Image: testImage, URL: testURL, Conditions: []string{"ROLLBACK_FAILED: tasks failed to start"}},
		},
		{
			name:       "stopped by hand",
			stack:      stack(cftypes.StackStatusUpdateComplete, true),
			service:    service(1, 1, ecstypes.DeploymentRolloutStateCompleted, ""),
			taskDef:    taskDef,
			deployment: deployment(ecstypes.ServiceDeploymentStatusStopped, "", ""),
			want: status.Result{Environment: "dev", Backend: "ecs", Deployed: true,
				Sync: "UPDATE_COMPLETE", Health: "Degraded", Available: 1,
				Image: testImage, URL: testURL, Conditions: []string{"STOPPED"}},
		},
		{
			name:    "internal only",
			stack:   withOutput(stack(cftypes.StackStatusUpdateComplete, false), "InternalURL", testInternalURL),
			service: service(1, 1, ecstypes.DeploymentRolloutStateCompleted, ""),
			taskDef: taskDef,
			want: status.Result{Environment: "dev", Backend: "ecs", Deployed: true,
				Sync: "UPDATE_COMPLETE", Health: "COMPLETED", Ready: "True", Available: 1,
				Image: testImage, URL: testInternalURL + " (internal)"},
		},
		{
			name:    "public URL wins",
			stack:   withOutput(stack(cftypes.StackStatusUpdateComplete, true), "InternalURL", testInternalURL),
			service: service(1, 1, ecstypes.DeploymentRolloutStateCompleted, ""),
			taskDef: taskDef,
			want: status.Result{Environment: "dev", Backend: "ecs", Deployed: true,
				Sync: "UPDATE_COMPLETE", Health: "COMPLETED", Ready: "True", Available: 1,
				Image: testImage, URL: testURL},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := result("dev", tt.stack, tt.service, tt.taskDef, tt.deployment)
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("got  %+v\nwant %+v", got, tt.want)
			}
		})
	}
}
