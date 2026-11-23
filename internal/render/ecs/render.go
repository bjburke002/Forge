package ecs

import (
	"bytes"
	"encoding/json"
	"fmt"

	"github.com/bjburke002/forge/pkg/contract/v1alpha1"
)

const useAlternateTargetGroup = "UseAlternateTargetGroup"

func Render(fsvc *v1alpha1.ForgeService, envName string) (*Template, error) {
	if envName != "dev" && envName != "prod" {
		return nil, fmt.Errorf("currently only dev or prod is accepted, received: %q", envName)
	}
	taskDef, err := taskDefinition(fsvc, envName)
	if err != nil {
		return nil, fmt.Errorf("%s: task definition: %w", fsvc.Name, err)
	}

	svcDef, err := serviceDefinition(fsvc, envName)
	if err != nil {
		return nil, fmt.Errorf("%s: service: %w", fsvc.Name, err)
	}

	ingDef, err := ingress(fsvc, envName)
	if err != nil {
		return nil, fmt.Errorf("%s: ingress: %w", fsvc.Name, err)
	}

	resources := map[string]Resource{}

	for _, group := range []map[string]Resource{taskDef, svcDef, ingDef} {
		for id, res := range group {
			if _, exists := resources[id]; exists {
				return nil, fmt.Errorf("%s: duplicate logical id %q", fsvc.Name, id)
			}
			resources[id] = res
		}
	}

	params := platformParameters()
	conditions := map[string]any{}
	outputs := map[string]Output{}
	if fsvc.Spec.Ingress != nil {
		outputs["URL"] = Output{
			Value:       "https://" + fsvc.Spec.Ingress.Host + fsvc.Spec.Ingress.Path,
			Description: "ForgeService Domain name",
		}

		for id, p := range canaryParameters() {
			params[id] = p
		}

		conditions[useAlternateTargetGroup] = equals(ref(paramActiveTargetGroup), "alternate")
	}

	return &Template{
		AWSTemplateFormatVersion: "2010-09-09",
		Description:              "Forge service " + ResourceName(fsvc.Name, envName),
		Parameters:               params,
		Conditions:               conditions,
		Resources:                resources,
		Outputs:                  outputs,
	}, nil
}

// JSON returns the template as forge render prints it: indented, with < and > unescaped.
func (t *Template) JSON() ([]byte, error) {
	var buf bytes.Buffer
	enc := json.NewEncoder(&buf)
	enc.SetEscapeHTML(false)
	enc.SetIndent("", "  ")
	if err := enc.Encode(t); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}
