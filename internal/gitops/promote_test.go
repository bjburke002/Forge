package gitops

import (
	"strings"
	"testing"
)

const testDev = `apiVersion: kustomize.config.k8s.io/v1beta1
kind: Kustomization
namespace: payments-dev
resources:
- ../base
images:
- name: payments-api
  newName: ` + testImage + `
  newTag: newtag
`

// A hand-written prod overlay on an older tag, with an env patch that must survive.
const testProd = `apiVersion: kustomize.config.k8s.io/v1beta1
kind: Kustomization
namespace: payments-prod
resources:
- ../base
images:
- name: payments-api
  newName: ` + testImage + `
  newTag: oldtag
patches:
- patch: |-
    apiVersion: forge.dev/v1alpha1
    kind: ForgeService
    metadata:
      name: payments
    spec:
      env:
        - name: LOG_LEVEL
          value: info
`

const prodPath = "eks/services/payments/prod/kustomization.yaml"

func TestPromoteFilesFirstPromotion(t *testing.T) {
	tag, files, err := PromoteFiles([]byte(testDev), nil, "payments", TargetEKS)
	if err != nil {
		t.Fatalf("PromoteFiles: %v", err)
	}
	if tag != "newtag" || len(files) != 1 {
		t.Fatalf("tag = %q, files = %d; want newtag and 1 file", tag, len(files))
	}
	prod := unmarshal[kustomization](t, files[prodPath])
	if prod.Namespace != "payments-prod" || strings.Join(prod.Resources, ",") != "../base" {
		t.Errorf("prod namespace/resources = %q/%v", prod.Namespace, prod.Resources)
	}
	want := kustomizeImage{Name: "payments-api", NewName: testImage, NewTag: "newtag"}
	if len(prod.Images) != 1 || prod.Images[0] != want {
		t.Errorf("prod images = %+v, want [%+v]", prod.Images, want)
	}
	if len(prod.Patches) != 0 {
		t.Errorf("prod patches = %+v; the base already has the prod host", prod.Patches)
	}
}

func TestPromoteFilesUpdatesTagOnly(t *testing.T) {
	tag, files, err := PromoteFiles([]byte(testDev), []byte(testProd), "payments", TargetEKS)
	if err != nil {
		t.Fatalf("PromoteFiles: %v", err)
	}
	if tag != "newtag" {
		t.Errorf("tag = %q, want newtag", tag)
	}
	prod := unmarshal[kustomization](t, files[prodPath])
	if len(prod.Images) != 1 || prod.Images[0].NewTag != "newtag" {
		t.Errorf("prod images = %+v, want newTag newtag", prod.Images)
	}
	if prod.Namespace != "payments-prod" {
		t.Errorf("namespace = %q, want it unchanged", prod.Namespace)
	}
	if len(prod.Patches) != 1 || !strings.Contains(prod.Patches[0].Patch, "LOG_LEVEL") {
		t.Errorf("the hand-written env patch was lost: %+v", prod.Patches)
	}
}

func TestPromoteFilesAlreadyOnTag(t *testing.T) {
	prod := strings.Replace(testProd, "oldtag", "newtag", 1)
	tag, files, err := PromoteFiles([]byte(testDev), []byte(prod), "payments", TargetEKS)
	if err != nil {
		t.Fatalf("PromoteFiles: %v", err)
	}
	if tag != "newtag" || len(files) != 0 {
		t.Errorf("tag = %q, files = %d; want newtag and nothing to commit", tag, len(files))
	}
}

func TestPromoteFilesErrors(t *testing.T) {
	devNoImages := testDev[:strings.Index(testDev, "images:")]
	tests := map[string]struct{ dev, prod string }{
		"dev has no images":          {devNoImages, ""},
		"dev image has no tag":       {strings.Replace(testDev, "  newTag: newtag\n", "", 1), ""},
		"prod has no matching image": {testDev, strings.Replace(testProd, "name: payments-api", "name: other-api", 1)},
		"prod has no images":         {testDev, testProd[:strings.Index(testProd, "images:")]},
	}
	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			var prod []byte
			if tc.prod != "" {
				prod = []byte(tc.prod)
			}
			if _, _, err := PromoteFiles([]byte(tc.dev), prod, "payments", TargetEKS); err == nil {
				t.Fatal("expected an error, got nil")
			}
		})
	}
}
