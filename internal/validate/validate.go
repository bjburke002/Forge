package validate

import (
	"fmt"

	apiextensionsinternal "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions"
	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	structuralschema "k8s.io/apiextensions-apiserver/pkg/apiserver/schema"
	cel "k8s.io/apiextensions-apiserver/pkg/apiserver/schema/cel"
	validation "k8s.io/apiextensions-apiserver/pkg/apiserver/validation"
	celconfig "k8s.io/apiserver/pkg/apis/cel"
	"sigs.k8s.io/yaml"
)

func decodeCRD(data []byte) (*apiextensionsv1.CustomResourceDefinition, error) {
	var crd apiextensionsv1.CustomResourceDefinition

	if err := yaml.Unmarshal(data, &crd); err != nil {
		return nil, fmt.Errorf("unmarshalling CRD: %w", err)
	}

	return &crd, nil
}

func schemaFor(crd *apiextensionsv1.CustomResourceDefinition, version string) (*apiextensionsv1.JSONSchemaProps, error) {
	for _, v := range crd.Spec.Versions {
		if v.Name == version {
			if v.Schema == nil || v.Schema.OpenAPIV3Schema == nil {
				return nil, fmt.Errorf("no schema found for version %q", version)
			}
			return v.Schema.OpenAPIV3Schema, nil
		}
	}
	return nil, fmt.Errorf("CRD %s has no version %q", crd.Name, version)
}

// Validator holds the schema validators that will be used in forge validate
type Validator struct {
	structural      *structuralschema.Structural
	schemaValidator validation.SchemaValidator
	celValidator    *cel.Validator
}

func New(crdYAML []byte) (*Validator, error) {
	crd, err := decodeCRD(crdYAML)
	if err != nil {
		return nil, err
	}

	v1Schema, err := schemaFor(crd, "v1alpha1")
	if err != nil {
		return nil, err
	}

	// Validation only works on internal apiextensions.JSONSchemaProps, not the V1 decoded above. Translating that here
	var internal apiextensionsinternal.JSONSchemaProps

	if err := apiextensionsv1.Convert_v1_JSONSchemaProps_To_apiextensions_JSONSchemaProps(v1Schema, &internal, nil); err != nil {
		return nil, fmt.Errorf("converting to internal JSON schema properties: %w", err)
	}

	structural, err := structuralschema.NewStructural(&internal)
	if err != nil {
		return nil, fmt.Errorf("creating new structural schema: %w", err)
	}

	schemaValidator, _, err := validation.NewSchemaValidator(&internal)
	if err != nil {
		return nil, fmt.Errorf("building schema validator: %w", err)
	}

	celValidator := cel.NewValidator(structural, true, celconfig.PerCallLimit)

	return &Validator{
		structural:      structural,
		schemaValidator: schemaValidator,
		celValidator:    celValidator,
	}, nil
}
