package ecs

// Template defines a CloudFormation template that will be rendered by the Forge CLI
type Template struct {
	AWSTemplateFormatVersion string               `json:"AWSTemplateFormatVersion"`
	Description              string               `json:"Description,omitempty"`
	Parameters               map[string]Parameter `json:"Parameters"`
	Conditions               map[string]any       `json:"Conditions,omitempty"`
	Resources                map[string]Resource  `json:"Resources"`
	Outputs                  map[string]Output    `json:"Outputs,omitempty"`
}

// Parameter defines the params used in Template. These are built in params.go
type Parameter struct {
	Type          string   `json:"Type"`
	Description   string   `json:"Description,omitempty"`
	Default       string   `json:"Default"`
	AllowedValues []string `json:"AllowedValues,omitempty"`
}

type Tag struct {
	Key   string `json:"Key"`
	Value string `json:"Value"`
}

// Resource defines the Resources used by the CloudFormation template
type Resource struct {
	Type       string   `json:"Type"`
	Properties any      `json:"Properties"`
	DependsOn  []string `json:"DependsOn,omitempty"`
}

// Outputs define outputs used to display various information by the CLI
type Output struct {
	Description string `json:"Description"`
	Value       any    `json:"Value"`
}

func ref(name string) map[string]string {
	return map[string]string{"Ref": name}
}

func sub(s string) map[string]string {
	return map[string]string{"Fn::Sub": s}
}

func ResourceName(name, env string) string {
	return "forge-" + name + "-" + env
}

// discoveryName is the Service Connect name: unique per environment in CloudWatch, which has no namespace dimension.
func discoveryName(name, env string) string {
	return name + "-" + env
}

func serviceTags(name, environment, owner string) []Tag {
	return []Tag{
		{
			Key:   "Project",
			Value: "forge",
		},
		{
			Key:   "Backend",
			Value: "ecs",
		},
		{
			Key:   "Service",
			Value: name,
		},
		{
			Key:   "Environment",
			Value: environment,
		},
		{
			Key:   "Owner",
			Value: owner,
		},
	}
}

// Conditions helpers
func equals(a, b any) map[string]any {
	return map[string]any{"Fn::Equals": []any{a, b}}
}

func ifThen(cond string, yes, no any) map[string]any {
	return map[string]any{"Fn::If": []any{cond, yes, no}}
}
