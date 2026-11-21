package ecs

// Template defines a CloudFormation template that will be rendered by the Forge CLI
type Template struct {
	AWSTemplateFormatVersion string               `json:"AWSTemplateFormatVersion"`
	Description              string               `json:"Description,omitempty"`
	Parameters               map[string]Parameter `json:"Parameters"`
	Resources                map[string]Resource  `json:"Resources"`
	Outputs                  map[string]Output    `json:"Outputs,omitempty"`
}

// Parameter defines the params used in Template. These are built in params.go
type Parameter struct {
	Type        string `json:"Type"`
	Description string `json:"Description,omitempty"`
	Default     string `json:"Default"`
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
	Value       string `json:"Value"`
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
