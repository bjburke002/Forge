package ecs

import (
	"reflect"
	"testing"

	"github.com/bjburke002/forge/pkg/contract/v1alpha1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

const testImage = "241197444395.dkr.ecr.us-east-1.amazonaws.com/forge/payments-api:abc123"

func testService(env ...v1alpha1.EnvVar) *v1alpha1.ForgeService {
	return &v1alpha1.ForgeService{
		ObjectMeta: metav1.ObjectMeta{Name: "payments"},
		Spec: v1alpha1.ForgeServiceSpec{
			Owner:   "payments-team",
			Image:   testImage,
			Runtime: v1alpha1.RuntimeSpec{Port: 8080},
			Env:     env,
		},
	}
}

// render calls taskDefinition and returns its two resources' typed properties.
func render(t *testing.T, fsvc *v1alpha1.ForgeService, env string) (LogGroupProperties, TaskDefinitionProperties) {
	t.Helper()
	res, err := taskDefinition(fsvc, env)
	if err != nil {
		t.Fatalf("taskDefinition: %v", err)
	}
	if len(res) != 2 {
		t.Fatalf("got %d resources, want 2", len(res))
	}
	lg, ok := res[logGroupId]
	if !ok || lg.Type != "AWS::Logs::LogGroup" {
		t.Fatalf("missing %s or wrong type: %+v", logGroupId, lg)
	}
	td, ok := res[taskDefinitionId]
	if !ok || td.Type != "AWS::ECS::TaskDefinition" {
		t.Fatalf("missing %s or wrong type: %+v", taskDefinitionId, td)
	}
	return lg.Properties.(LogGroupProperties), td.Properties.(TaskDefinitionProperties)
}

func TestTaskDefinition(t *testing.T) {
	lg, td := render(t, testService(v1alpha1.EnvVar{Name: "LOG_LEVEL", Value: "info"}), "dev")

	if lg.LogGroupName != "forge-payments-dev" {
		t.Errorf("log group name = %q, want forge-payments-dev", lg.LogGroupName)
	}
	if td.Family != "forge-payments-dev" {
		t.Errorf("family = %q, want forge-payments-dev", td.Family)
	}
	if !reflect.DeepEqual(td.RequiresCompatibilities, []string{"FARGATE"}) || td.NetworkMode != "awsvpc" {
		t.Errorf("compatibilities %v, network mode %q; want [FARGATE], awsvpc", td.RequiresCompatibilities, td.NetworkMode)
	}
	if !reflect.DeepEqual(td.ExecutionRoleArn, ref(paramExecutionRoleArn)) {
		t.Errorf("execution role = %v, want a Ref to %s", td.ExecutionRoleArn, paramExecutionRoleArn)
	}
	if len(td.ContainerDefinitions) != 1 {
		t.Fatalf("got %d containers, want 1", len(td.ContainerDefinitions))
	}

	c := td.ContainerDefinitions[0]
	if c.Image != testImage {
		t.Errorf("image = %q, want %q", c.Image, testImage)
	}
	if !reflect.DeepEqual(c.PortMappings, []PortMapping{{ContainerPort: 8080, Protocol: "tcp"}}) {
		t.Errorf("port mappings = %+v", c.PortMappings)
	}
	if !reflect.DeepEqual(c.Environment, []KeyValuePair{{Name: "LOG_LEVEL", Value: "info"}}) {
		t.Errorf("environment = %+v", c.Environment)
	}

	// Security defaults.
	if !c.ReadonlyRootFilesystem || c.User != "65532" || c.Privileged {
		t.Errorf("read-only root %v, user %q, privileged %v; want true, 65532, false",
			c.ReadonlyRootFilesystem, c.User, c.Privileged)
	}
	if !reflect.DeepEqual(c.MountPoints, []MountPoint{{SourceVolume: "tmp", ContainerPath: "/tmp"}}) {
		t.Errorf("mount points = %+v, want a writable /tmp", c.MountPoints)
	}
	if !reflect.DeepEqual(td.Volumes, []Volume{{Name: "tmp"}}) {
		t.Errorf("volumes = %+v, want tmp", td.Volumes)
	}

	// Logs go to this stack's log group.
	if c.LogConfiguration.LogDriver != "awslogs" {
		t.Errorf("log driver = %q, want awslogs", c.LogConfiguration.LogDriver)
	}
	if got := c.LogConfiguration.Options["awslogs-group"]; !reflect.DeepEqual(got, ref(logGroupId)) {
		t.Errorf("awslogs-group = %v, want a Ref to %s", got, logGroupId)
	}

	// Both resources carry the tags ecs-down and cost reports rely on.
	wantTags := serviceTags("payments", "dev", "payments-team")
	if !reflect.DeepEqual(lg.Tags, wantTags) || !reflect.DeepEqual(td.Tags, wantTags) {
		t.Errorf("tags: log group %+v, task definition %+v; want %+v", lg.Tags, td.Tags, wantTags)
	}
}

// Dev and prod must get separate families and log groups, and no env means no Environment.
func TestTaskDefinitionProdWithoutEnv(t *testing.T) {
	lg, td := render(t, testService(), "prod")

	if td.Family != "forge-payments-prod" || lg.LogGroupName != "forge-payments-prod" {
		t.Errorf("family %q, log group %q; want forge-payments-prod for both", td.Family, lg.LogGroupName)
	}
	if env := td.ContainerDefinitions[0].Environment; len(env) != 0 {
		t.Errorf("environment = %+v, want empty", env)
	}
}
