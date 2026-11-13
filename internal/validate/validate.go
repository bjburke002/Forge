package validate

import (
	"context"
	"errors"
	"fmt"

	"github.com/bjburke002/forge/pkg/contract/v1alpha1"
	apiextensionsinternal "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions"
	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	structuralschema "k8s.io/apiextensions-apiserver/pkg/apiserver/schema"
	cel "k8s.io/apiextensions-apiserver/pkg/apiserver/schema/cel"
	"k8s.io/apiextensions-apiserver/pkg/apiserver/schema/defaulting"
	pruning "k8s.io/apiextensions-apiserver/pkg/apiserver/schema/pruning"
	validation "k8s.io/apiextensions-apiserver/pkg/apiserver/validation"
	unstructured "k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	utiljson "k8s.io/apimachinery/pkg/util/json"
	"k8s.io/apimachinery/pkg/util/validation/field"
	celconfig "k8s.io/apiserver/pkg/apis/cel"
	"sigs.k8s.io/yaml"
)

const forgeServiceKind = "ForgeService"

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

// New creates a new Validator based on a synced Forge Service CRD
func New(crdYAML []byte) (*Validator, error) {
	crd, err := decodeCRD(crdYAML)
	if err != nil {
		return nil, err
	}

	v1Schema, err := schemaFor(crd, v1alpha1.GroupVersion.Version)
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

// Validate method compares passed in ForgeService with well-defined Validators and returns any issues
// Returns an ErrorList for an invalid manifest, and an error for an unreadable manifest
func (v *Validator) Validate(ctx context.Context, data []byte) (field.ErrorList, error) {
	// sigs.k8s.io/yaml converts YAML to JSON and then uses the standard library's json.Unmarshal which
	// turns every number into a float64. Schema and CEL expect int64, so decoding in 2 steps to make sure
	// that's what we get
	jsonData, err := yaml.YAMLToJSON(data)
	if err != nil {
		return nil, fmt.Errorf("converting YAML to JSON: %w", err)
	}

	var obj map[string]any

	if err := utiljson.Unmarshal(jsonData, &obj); err != nil {
		return nil, fmt.Errorf("parsing manifest: %w", err)
	}

	if obj == nil {
		return nil, errors.New("manifest is empty")
	}

	// Checking type and name manually as the validators don't do that for us
	var errs field.ErrorList

	apiVersion, found, err := unstructured.NestedString(obj, "apiVersion")
	avPath := field.NewPath("apiVersion")
	groupVersion := v1alpha1.GroupVersion.String()

	if err != nil {
		errs = append(errs, field.Invalid(avPath, obj["apiVersion"], "must be a string"))
	} else if !found {
		errs = append(errs, field.Required(avPath, ""))
	} else if apiVersion != groupVersion {
		errs = append(errs, field.Invalid(avPath, apiVersion, fmt.Sprintf("must be %s", groupVersion)))
	}

	kind, found, err := unstructured.NestedString(obj, "kind")
	kPath := field.NewPath("kind")

	if err != nil {
		errs = append(errs, field.Invalid(kPath, obj["kind"], "must be a string"))
	} else if !found {
		errs = append(errs, field.Required(kPath, ""))
	} else if kind != forgeServiceKind {
		errs = append(errs, field.Invalid(kPath, kind, fmt.Sprintf("must be a %s", forgeServiceKind)))
	}

	name, found, err := unstructured.NestedString(obj, "metadata", "name")
	nPath := field.NewPath("metadata", "name")

	if err != nil {
		raw, _, _ := unstructured.NestedFieldNoCopy(obj, "metadata", "name")
		errs = append(errs, field.Invalid(nPath, raw, "must be a string"))
	} else if !found || name == "" {
		errs = append(errs, field.Required(nPath, ""))
	}

	// Early return if any errors are appended above
	if len(errs) > 0 {
		return errs, nil
	}

	// Pruning fields that aren't declared in the Validator's structural schema
	unknownFields := pruning.PruneWithOptions(obj, v.structural, true, structuralschema.UnknownFieldPathOptions{TrackUnknownFieldPaths: true})

	for _, p := range unknownFields {
		fieldPath := field.NewPath(p)
		errs = append(errs, field.Forbidden(fieldPath, "unknown field"))
	}

	// Defaulting needs to happen here so the schema and CEL checks can see what the API server would store
	// Updates obj in place
	defaulting.Default(obj, v.structural)

	// Schema validation to catch types, required fields, enums, patterns, mins and max
	schemaErrs := validation.ValidateCustomResource(nil, obj, v.schemaValidator)

	errs = append(errs, schemaErrs...)

	if v.celValidator != nil {
		celErrs, _ := v.celValidator.Validate(ctx, nil, v.structural, obj, nil, celconfig.RuntimeCELCostBudget)
		errs = append(errs, celErrs...)

	}

	return errs, nil
}
