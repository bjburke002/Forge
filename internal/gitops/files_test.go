package gitops

import (
	"strings"
	"testing"

	"sigs.k8s.io/yaml"
)

const testImage = "123456789012.dkr.ecr.us-east-1.amazonaws.com/forge/payments-api"

const testCR = `apiVersion: forge.dev/v1alpha1
kind: ForgeService
metadata:
  name: payments
spec:
  owner: payments-team
  image: ` + testImage + `:abc123
  runtime:
    port: 8080
  ingress:
    host: payments.example.com
`

// unmarshal decodes generated YAML, failing the test if it doesn't parse.
func unmarshal[T any](t *testing.T, data []byte) T {
	t.Helper()
	var v T
	if err := yaml.Unmarshal(data, &v); err != nil {
		t.Fatalf("generated YAML doesn't parse: %v\n%s", err, data)
	}
	return v
}

func TestFiles(t *testing.T) {
	name, files, err := Files([]byte(testCR), "example.com")
	if err != nil {
		t.Fatalf("Files: %v", err)
	}
	if name != "payments" {
		t.Errorf("name = %q, want payments", name)
	}
	if len(files) != 4 {
		t.Errorf("got %d files, want 4", len(files))
	}

	// Base CR: image swapped for the placeholder, everything else kept.
	cr := unmarshal[map[string]any](t, files["eks/services/payments/base/forgeservice.yaml"])
	spec, _ := cr["spec"].(map[string]any)
	if spec["image"] != "payments-api" {
		t.Errorf("base spec.image = %v, want the placeholder payments-api", spec["image"])
	}
	if spec["owner"] != "payments-team" {
		t.Errorf("base spec.owner = %v, want it unchanged", spec["owner"])
	}

	base := unmarshal[kustomization](t, files["eks/services/payments/base/kustomization.yaml"])
	if strings.Join(base.Resources, ",") != "forgeservice.yaml" || strings.Join(base.Configurations, ",") != "kustomizeconfig.yaml" {
		t.Errorf("base kustomization = %+v", base)
	}
	if string(files["eks/services/payments/base/kustomizeconfig.yaml"]) != kustomizeConfig {
		t.Error("kustomizeconfig.yaml doesn't match the constant")
	}

	dev := unmarshal[kustomization](t, files["eks/services/payments/dev/kustomization.yaml"])
	if dev.Namespace != "payments-dev" || strings.Join(dev.Resources, ",") != "../base" {
		t.Errorf("dev namespace/resources = %q/%v", dev.Namespace, dev.Resources)
	}
	wantImage := kustomizeImage{Name: "payments-api", NewName: testImage, NewTag: "abc123"}
	if len(dev.Images) != 1 || dev.Images[0] != wantImage {
		t.Errorf("dev images = %+v, want [%+v]", dev.Images, wantImage)
	}
	if len(dev.Patches) != 1 || !strings.Contains(dev.Patches[0].Patch, "host: payments-dev.example.com") {
		t.Errorf("dev patches = %+v, want one patch setting the dev host", dev.Patches)
	}
}

func TestFilesWithoutIngress(t *testing.T) {
	cr := strings.Replace(testCR, "  ingress:\n    host: payments.example.com\n", "", 1)
	// No ingress, so no domain is needed and no host patch is written.
	_, files, err := Files([]byte(cr), "")
	if err != nil {
		t.Fatalf("Files: %v", err)
	}
	dev := unmarshal[kustomization](t, files["eks/services/payments/dev/kustomization.yaml"])
	if len(dev.Patches) != 0 {
		t.Errorf("dev patches = %+v, want none", dev.Patches)
	}
}

func TestFilesErrors(t *testing.T) {
	tests := map[string]struct {
		cr, domain string
	}{
		"ingress without a domain": {testCR, ""},
		"image without a tag":      {strings.Replace(testCR, ":abc123", "", 1), "example.com"},
		"missing name":             {strings.Replace(testCR, "  name: payments\n", "", 1), "example.com"},
		"empty manifest":           {"", "example.com"},
	}
	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			if _, _, err := Files([]byte(tc.cr), tc.domain); err == nil {
				t.Fatal("expected an error, got nil")
			}
		})
	}
}
