package ecs

import (
	"fmt"

	"github.com/bjburke002/forge/pkg/contract/v1alpha1"
)

const (
	logGroupId       = "LogGroup"
	taskDefinitionId = "TaskDefinition"
)

type LogGroupProperties struct {
	LogGroupName    string `json:"LogGroupName"`
	RetentionInDays int    `json:"RetentionInDays"`
	Tags            []Tag  `json:"Tags"`
}

type TaskDefinitionProperties struct {
	Family                  string                `json:"Family"`
	RequiresCompatibilities []string              `json:"RequiresCompatibilities"`
	NetworkMode             string                `json:"NetworkMode"`
	CPU                     string                `json:"Cpu"`
	Memory                  string                `json:"Memory"`
	ExecutionRoleArn        any                   `json:"ExecutionRoleArn"`
	ContainerDefinitions    []ContainerDefinition `json:"ContainerDefinitions"`
	Volumes                 []Volume              `json:"Volumes,omitempty"`
	Tags                    []Tag                 `json:"Tags"`
}

type ContainerDefinition struct {
	Name                   string           `json:"Name"`
	Image                  string           `json:"Image"`
	Essential              bool             `json:"Essential"`
	PortMappings           []PortMapping    `json:"PortMappings"`
	Environment            []KeyValuePair   `json:"Environment,omitempty"`
	ReadonlyRootFilesystem bool             `json:"ReadonlyRootFilesystem"`
	User                   string           `json:"User"`
	Privileged             bool             `json:"Privileged"`
	MountPoints            []MountPoint     `json:"MountPoints"`
	LogConfiguration       LogConfiguration `json:"LogConfiguration"`
}

type PortMapping struct {
	ContainerPort int32  `json:"ContainerPort"`
	Protocol      string `json:"Protocol"`
}

type KeyValuePair struct {
	Name  string `json:"Name"`
	Value string `json:"Value"`
}

type MountPoint struct {
	SourceVolume  string `json:"SourceVolume"`
	ContainerPath string `json:"ContainerPath"`
	ReadOnly      bool   `json:"ReadOnly"`
}

type Volume struct {
	Name string `json:"Name"`
}

type LogConfiguration struct {
	LogDriver string         `json:"LogDriver"`
	Options   map[string]any `json:"Options"`
}

func taskDefinition(fsvc *v1alpha1.ForgeService, envName string) (map[string]Resource, error) {
	name := ResourceName(fsvc.Name, envName)

	envs := make([]KeyValuePair, 0, len(fsvc.Spec.Env))

	for _, env := range fsvc.Spec.Env {
		e := KeyValuePair{
			Name:  env.Name,
			Value: env.Value,
		}
		envs = append(envs, e)
	}

	logGroup := Resource{
		Type: "AWS::Logs::LogGroup",
		Properties: LogGroupProperties{
			LogGroupName:    name,
			RetentionInDays: 7,
			Tags:            serviceTags(fsvc.Name, envName, fsvc.Spec.Owner),
		},
	}

	container := ContainerDefinition{
		Name:      fsvc.Name,
		Image:     fsvc.Spec.Image,
		Essential: true,
		PortMappings: []PortMapping{
			{
				ContainerPort: fsvc.Spec.Runtime.Port,
				Protocol:      "tcp",
			},
		},
		Environment:            envs,
		ReadonlyRootFilesystem: true,
		User:                   "65532",
		Privileged:             false,
		MountPoints: []MountPoint{
			{
				SourceVolume:  "tmp",
				ContainerPath: "/tmp",
				ReadOnly:      false,
			},
		},
		LogConfiguration: LogConfiguration{
			LogDriver: "awslogs",
			Options: map[string]any{
				"awslogs-group":         ref(logGroupId),
				"awslogs-region":        ref("AWS::Region"),
				"awslogs-stream-prefix": fsvc.Name,
			},
		},
	}

	cpuSize, memSize, err := taskSize(fsvc.Spec.Resources)
	if err != nil {
		return nil, fmt.Errorf("sizing task for %s: %w", fsvc.Name, err)
	}

	taskDef := Resource{
		Type: "AWS::ECS::TaskDefinition",
		Properties: TaskDefinitionProperties{
			Family:                  name,
			RequiresCompatibilities: []string{"FARGATE"},
			NetworkMode:             "awsvpc",
			CPU:                     cpuSize,
			Memory:                  memSize,
			ExecutionRoleArn:        ref(paramExecutionRoleArn),
			ContainerDefinitions:    []ContainerDefinition{container},
			Volumes: []Volume{
				{
					Name: "tmp",
				},
			},
			Tags: serviceTags(fsvc.Name, envName, fsvc.Spec.Owner),
		},
	}

	return map[string]Resource{
		logGroupId:       logGroup,
		taskDefinitionId: taskDef,
	}, nil
}
