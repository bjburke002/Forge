package ecs

const (
	paramClusterName           = "ClusterName"
	paramHTTPSListenerArn      = "HTTPSListenerArn"
	paramTaskSecurityGroupId   = "TaskSecurityGroupId"
	paramPrivateSubnetIds      = "PrivateSubnetIds"
	paramALBDNSName            = "ALBDNSName"
	paramALBZoneId             = "ALBZoneId"
	paramExecutionRoleArn      = "ExecutionRoleArn"
	paramHostedZoneId          = "HostedZoneId"
	paramVPCId                 = "VPCId"
	paramTestListenerArn       = "TestListenerArn"
	paramInfrastructureRoleArn = "InfrastructureRoleArn"
	paramHookInvokeRoleArn     = "HookInvokeRoleArn"
	paramCanaryHookArn         = "CanaryHookArn"
	paramActiveTargetGroup     = "ActiveTargetGroup"
	paramALBFullName           = "ALBFullName"
)

const ssmString = "AWS::SSM::Parameter::Value<String>"

func platformParameters() map[string]Parameter {
	return map[string]Parameter{
		paramClusterName: {
			Type:        ssmString,
			Description: "ECS ClusterName",
			Default:     "/forge/ecs/cluster-name",
		},
		paramHTTPSListenerArn: {
			Type:        ssmString,
			Description: "ECS ALB HTTPS Listener ARN",
			Default:     "/forge/ecs/https-listener-arn",
		},
		paramTaskSecurityGroupId: {
			Type:        "AWS::SSM::Parameter::Value<AWS::EC2::SecurityGroup::Id>",
			Description: "ID for the ECS Task Security Group",
			Default:     "/forge/ecs/task-security-group-id",
		},
		paramPrivateSubnetIds: {
			Type:        "AWS::SSM::Parameter::Value<List<AWS::EC2::Subnet::Id>>",
			Description: "List of IDs for the ECS private subnets",
			Default:     "/forge/ecs/private-subnet-ids",
		},
		paramALBDNSName: {
			Type:        ssmString,
			Description: "DNS Name for the ALB",
			Default:     "/forge/ecs/alb-dns-name",
		},
		paramALBZoneId: {
			Type:        ssmString,
			Description: "Route53 Zone ID for ECS ALB",
			Default:     "/forge/ecs/alb-zone-id",
		},
		paramExecutionRoleArn: {
			Type:        ssmString,
			Description: "ARN for the ECS Execution Role",
			Default:     "/forge/shared/ecs-task-execution-role-arn",
		},
		paramHostedZoneId: {
			Type:        "AWS::SSM::Parameter::Value<AWS::Route53::HostedZone::Id>",
			Description: "Route53 Hosted Zone for forgeappservices.com",
			Default:     "/forge/shared/hosted-zone-id",
		},
		paramVPCId: {
			Type:        "AWS::SSM::Parameter::Value<AWS::EC2::VPC::Id>",
			Description: "The VPC ID of the ECS VPC",
			Default:     "/forge/ecs/vpc-id",
		},
	}
}

func canaryParameters() map[string]Parameter {
	return map[string]Parameter{
		paramTestListenerArn: {
			Type:        ssmString,
			Description: "The ARN of the canary test listener",
			Default:     "/forge/ecs/test-listener-arn",
		},
		paramInfrastructureRoleArn: {
			Type:        ssmString,
			Description: "The ARN of the ECS Infrastructure Role",
			Default:     "/forge/shared/ecs-infrastructure-role-arn",
		},
		paramHookInvokeRoleArn: {
			Type:        ssmString,
			Description: "The ARN of the Hook Invoke ARN used to invoke Lambda",
			Default:     "/forge/shared/ecs-hook-invoke-role-arn",
		},
		paramCanaryHookArn: {
			Type:        ssmString,
			Description: "Canary Hook ARN",
			Default:     "/forge/ecs/canary-hook-arn",
		},
		paramActiveTargetGroup: {
			Type:          "String",
			Description:   "Target group that production currently forwards to",
			Default:       "primary",
			AllowedValues: []string{"primary", "alternate"},
		},
		paramALBFullName: {
			Type:        ssmString,
			Description: "Full ARN suffix of the ALB",
			Default:     "/forge/ecs/alb-full-name",
		},
	}
}
