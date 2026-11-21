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
	testURL   = "https://billing-dev.forgeappservices.com"
	testImage = "241197444395.dkr.ecr.us-east-1.amazonaws.com/forge/payments-api:abc123"
)

func stack(st cftypes.StackStatus, withURL bool) *cftypes.Stack {
	s := &cftypes.Stack{StackStatus: st}
	if withURL {
		s.Outputs = []cftypes.Output{{OutputKey: aws.String("URL"), OutputValue: aws.String(testURL)}}
	}
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

func TestResult(t *testing.T) {
	tests := []struct {
		name    string
		stack   *cftypes.Stack
		service *ecstypes.Service
		taskDef *ecstypes.TaskDefinition
		want    status.Result
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
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := result("dev", tt.stack, tt.service, tt.taskDef)
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("got  %+v\nwant %+v", got, tt.want)
			}
		})
	}
}
