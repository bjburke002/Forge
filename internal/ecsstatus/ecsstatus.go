package ecsstatus

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/cloudformation"
	cftypes "github.com/aws/aws-sdk-go-v2/service/cloudformation/types"
	ecsv2 "github.com/aws/aws-sdk-go-v2/service/ecs"
	ecstypes "github.com/aws/aws-sdk-go-v2/service/ecs/types"
	"github.com/aws/aws-sdk-go-v2/service/ssm"
	ssmtypes "github.com/aws/aws-sdk-go-v2/service/ssm/types"
	"github.com/aws/smithy-go"
	"github.com/bjburke002/forge/internal/render/ecs"
	"github.com/bjburke002/forge/internal/status"
)

func Get(ctx context.Context, cfg aws.Config, name string) ([]status.Result, error) {
	// Set up clients for all 3 services to use
	cfClient := cloudformation.NewFromConfig(cfg)
	ecsClient := ecsv2.NewFromConfig(cfg)
	ssmClient := ssm.NewFromConfig(cfg)

	p, err := ssmClient.GetParameter(ctx, &ssm.GetParameterInput{
		Name: aws.String("/forge/ecs/cluster-name"),
	})
	if err != nil {
		var notFound *ssmtypes.ParameterNotFound
		if errors.As(err, &notFound) {
			return nil, errors.New("ECS foundation is down (no /forge/ecs/cluster-name)")
		}
		return nil, fmt.Errorf("reading /forge/ecs/cluster-name: %w", err)
	}
	envs := []string{"dev", "prod"}
	clusterName := aws.ToString(p.Parameter.Value)
	results := []status.Result{}

	for _, env := range envs {
		stackName := ecs.ResourceName(name, env)
		cfInput := &cloudformation.DescribeStacksInput{
			StackName: aws.String(stackName),
		}
		stackOut, err := cfClient.DescribeStacks(ctx, cfInput)
		if err != nil {
			if stackMissing(err) {
				r := result(env, nil, nil, nil)
				results = append(results, r)
				continue
			}
			return nil, fmt.Errorf("describing stack %s: %w", stackName, err)
		}

		ecsInput := &ecsv2.DescribeServicesInput{
			Services: []string{stackName},
			Cluster:  aws.String(clusterName),
		}
		ecsService, err := ecsClient.DescribeServices(ctx, ecsInput)
		if err != nil {
			return nil, fmt.Errorf("reading ecs service %s: %w", stackName, err)
		}
		if len(ecsService.Services) == 0 {
			r := result(env, &stackOut.Stacks[0], nil, nil)
			results = append(results, r)
			continue
		}
		svc := &ecsService.Services[0]
		taskDefInput := ecsv2.DescribeTaskDefinitionInput{
			TaskDefinition: svc.TaskDefinition,
		}

		tdOut, err := ecsClient.DescribeTaskDefinition(ctx, &taskDefInput)
		if err != nil {
			return nil, fmt.Errorf("describing task definition for %s: %w", stackName, err)
		}

		r := result(env, &stackOut.Stacks[0], svc, tdOut.TaskDefinition)
		results = append(results, r)
	}
	return results, nil
}

func stackMissing(err error) bool {
	var apiErr smithy.APIError
	if errors.As(err, &apiErr) {
		if apiErr.ErrorCode() == "ValidationError" && strings.Contains(apiErr.ErrorMessage(), "does not exist") {
			return true
		}
	}
	return false
}

func result(env string, stack *cftypes.Stack, service *ecstypes.Service, taskDef *ecstypes.TaskDefinition) status.Result {
	res := status.Result{
		Environment: env,
		Backend:     "ecs",
	}

	if stack == nil {
		return res
	}

	res.Deployed = true
	res.Sync = string(stack.StackStatus)
	for _, out := range stack.Outputs {
		if aws.ToString(out.OutputKey) == "URL" {
			res.URL = aws.ToString(out.OutputValue)
			break
		}
	}

	if service == nil {
		res.Health = "service missing"
		return res
	}

	for _, d := range service.Deployments {
		if aws.ToString(d.Status) == "PRIMARY" {
			res.Available = int32(service.RunningCount)
			if service.RunningCount == service.DesiredCount && d.RolloutState == ecstypes.DeploymentRolloutStateCompleted {
				res.Ready = "True"
			}
			res.Health = string(d.RolloutState)
			if d.RolloutState == ecstypes.DeploymentRolloutStateFailed {
				res.Conditions = append(res.Conditions, aws.ToString(d.RolloutStateReason))
			}
			break
		}
	}

	if taskDef != nil && len(taskDef.ContainerDefinitions) > 0 {
		res.Image = aws.ToString(taskDef.ContainerDefinitions[0].Image)
	}

	return res
}
