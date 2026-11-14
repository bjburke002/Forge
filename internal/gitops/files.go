package gitops

import (
	"errors"
	"fmt"
	"path"

	"github.com/bjburke002/forge/pkg/contract/v1alpha1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	utiljson "k8s.io/apimachinery/pkg/util/json"
	"sigs.k8s.io/yaml"
)

const (
	kustomizeAPIVersion = "kustomize.config.k8s.io/v1beta1"
	kustomizeKind       = "Kustomization"
	// kustomizeConfig is universal between all services in all environments, so we can store this as a const
	kustomizeConfig = `images:
  - path: spec/image
    kind: ForgeService
`
)

// Files generates the Kustomize base manifest and overlays for newly created services
func Files(data []byte, domain string) (name string, files map[string][]byte, err error) {
	manifest, err := decodeCR(data)
	if err != nil {
		return "", nil, fmt.Errorf("decoding manifest: %w", err)
	}

	name, found, err := unstructured.NestedString(manifest, "metadata", "name")
	if err != nil {
		return "", nil, fmt.Errorf("reading metadata.name: %w", err)
	}
	if name == "" || !found {
		return "", nil, errors.New("metadata.name is required")
	}

	image, found, err := unstructured.NestedString(manifest, "spec", "image")
	if err != nil {
		return "", nil, fmt.Errorf("reading spec.image: %w", err)
	}
	if image == "" || !found {
		return "", nil, errors.New("spec.image is required")
	}

	ref, err := parseImage(image)
	if err != nil {
		return "", nil, err
	}

	if err := unstructured.SetNestedField(manifest, ref.placeholder, "spec", "image"); err != nil {
		return "", nil, fmt.Errorf("setting spec.image placeholder: %w", err)
	}

	baseCR, err := yaml.Marshal(manifest)
	if err != nil {
		return "", nil, fmt.Errorf("marshaling manifest: %w", err)
	}

	files = make(map[string][]byte)
	files[path.Join("services", name, "base", "forgeservice.yaml")] = baseCR

	baseK := kustomization{
		APIVersion:     kustomizeAPIVersion,
		Kind:           kustomizeKind,
		Resources:      []string{"forgeservice.yaml"},
		Configurations: []string{"kustomizeconfig.yaml"},
	}

	baseKustomization, err := yaml.Marshal(baseK)
	if err != nil {
		return "", nil, fmt.Errorf("marshaling base kustomization: %w", err)
	}
	files[path.Join("services", name, "base", "kustomization.yaml")] = baseKustomization
	files[path.Join("services", name, "base", "kustomizeconfig.yaml")] = []byte(kustomizeConfig)

	devK := kustomization{
		APIVersion: kustomizeAPIVersion,
		Kind:       kustomizeKind,
		Namespace:  name + "-dev",
		Resources:  []string{"../base"},
		Images:     []kustomizeImage{{Name: ref.placeholder, NewName: ref.name, NewTag: ref.tag}},
	}

	// If an ingress exists, we patch that in so that dev and prod can't use the same hostname
	_, found, err = unstructured.NestedMap(manifest, "spec", "ingress")
	if err != nil {
		return "", nil, fmt.Errorf("spec.ingress must be a map: %w", err)
	}
	if found {
		if domain == "" {
			return "", nil, errors.New("spec.ingress is set but no domain is configured")
		}

		patchObj := map[string]any{
			"apiVersion": v1alpha1.GroupVersion.String(),
			"kind":       "ForgeService",
			"metadata": map[string]any{
				"name": name,
			},
			"spec": map[string]any{
				"ingress": map[string]any{
					"host": fmt.Sprintf("%s-dev.%s", name, domain),
				},
			},
		}

		patchBytes, err := yaml.Marshal(patchObj)
		if err != nil {
			return "", nil, fmt.Errorf("marshaling kustomization patch: %w", err)
		}
		devK.Patches = []kustomizePatch{{Patch: string(patchBytes)}}
	}

	devKustomization, err := yaml.Marshal(devK)
	if err != nil {
		return "", nil, fmt.Errorf("marshaling dev kustomization: %w", err)
	}

	files[path.Join("services", name, "dev", "kustomization.yaml")] = devKustomization

	return name, files, nil
}

type kustomization struct {
	APIVersion     string           `json:"apiVersion"`
	Kind           string           `json:"kind"`
	Namespace      string           `json:"namespace,omitempty"`
	Resources      []string         `json:"resources"`
	Configurations []string         `json:"configurations,omitempty"`
	Images         []kustomizeImage `json:"images,omitempty"`
	Patches        []kustomizePatch `json:"patches,omitempty"`
}

type kustomizeImage struct {
	Name    string `json:"name"`
	NewName string `json:"newName"`
	NewTag  string `json:"newTag"`
}

type kustomizePatch struct {
	Patch string `json:"patch"`
}

func decodeCR(data []byte) (map[string]any, error) {
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
	return obj, nil
}
