package gitops

import (
	"errors"
	"fmt"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"sigs.k8s.io/yaml"
)

func PromoteFiles(dev, prod []byte, name string) (tag string, files map[string][]byte, err error) {
	var devK kustomization
	if err := yaml.Unmarshal(dev, &devK); err != nil {
		return "", nil, fmt.Errorf("parsing dev kustomization: %w", err)
	}

	if devK.Images == nil {
		return "", nil, errors.New("dev overlay has no images entry")
	}
	if devK.Images[0].NewTag == "" {
		return "", nil, fmt.Errorf("no image tag for %v", devK.Images[0])
	}
	ref := imageRef{
		name:        devK.Images[0].NewName,
		tag:         devK.Images[0].NewTag,
		placeholder: devK.Images[0].Name,
	}

	// If prod overlay doesn't exist, create it here
	if len(prod) == 0 {
		newProdK := overlay(name, "prod", ref)

		prodKustomization, err := yaml.Marshal(newProdK)
		if err != nil {
			return "", nil, fmt.Errorf("marshaling prod kustomization: %w", err)
		}
		files := make(map[string][]byte)
		files[ServicePath(TargetEKS, name, "prod", "kustomization.yaml")] = prodKustomization
		return ref.tag, files, nil
	}

	prodK, err := decodeCR(prod)
	if err != nil {
		return "", nil, fmt.Errorf("decoding prod kustomization: %w", err)
	}

	images, found, err := unstructured.NestedSlice(prodK, "images")
	if err != nil {
		return "", nil, fmt.Errorf("reading prod overlay image tag: %w", err)
	}
	if !found {
		return "", nil, errors.New("prod overlay has no images entry")
	}

	matched := false
	for _, i := range images {
		entry, ok := i.(map[string]interface{})
		if !ok {
			continue
		}

		name, _ := entry["name"].(string)
		if name != ref.placeholder {
			continue
		}
		matched = true
		currentTag, _ := entry["newTag"].(string)

		if currentTag == ref.tag {
			return ref.tag, nil, nil
		}
		entry["newTag"] = ref.tag
	}
	if !matched {
		return "", nil, fmt.Errorf("prod overlay has no image named %q", ref.placeholder)
	}
	if err := unstructured.SetNestedSlice(prodK, images, "images"); err != nil {
		return "", nil, fmt.Errorf("writing dev tag to prod kustomization: %w", err)
	}

	prodManifest, err := yaml.Marshal(prodK)
	if err != nil {
		return "", nil, fmt.Errorf("writing prod kustomization: %w", err)
	}

	f := make(map[string][]byte)
	f[ServicePath(TargetEKS, name, "prod", "kustomization.yaml")] = prodManifest

	return ref.tag, f, nil
}
