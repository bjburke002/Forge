package validate

import (
	"context"
	"fmt"
	"slices"
	"strings"
	"testing"

	"k8s.io/apimachinery/pkg/util/validation/field"

	"github.com/bjburke002/forge/config/crd"
)

// TestNewWithEmbeddedCRD guards against a regenerated CRD or a dependency
// bump that stops the real schema from loading.
func TestNewWithEmbeddedCRD(t *testing.T) {
	v, err := New(crd.ForgeService)
	if err != nil {
		t.Fatalf("New(crd.ForgeService): %v", err)
	}
	// The CRD carries x-kubernetes-validations rules, so a nil CEL validator
	// means they were silently dropped.
	if v.celValidator == nil {
		t.Fatal("expected a CEL validator: CRD has x-kubernetes-validations rules")
	}
}

// base is a minimal valid ForgeService; each case edits one thing in it.
const base = `apiVersion: forge.dev/v1alpha1
kind: ForgeService
metadata:
  name: payments
spec:
  owner: payments-team
  image: example.com/payments:abc123
  runtime:
    port: 8080
  scaling:
    minReplicas: 2
    maxReplicas: 5
  resources:
    requests:
      cpu: 25m
      memory: 32Mi
    limits:
      memory: 128Mi
  health:
    readiness: /health/ready
    liveness: /health/live
  observability:
    metricsPort: 9090
`

func TestValidate(t *testing.T) {
	v, err := New(crd.ForgeService)
	if err != nil {
		t.Fatalf("New: %v", err)
	}

	// edit swaps one snippet of base, failing loudly if the snippet isn't there.
	edit := func(old, replacement string) string {
		t.Helper()
		if !strings.Contains(base, old) {
			t.Fatalf("test setup: %q not found in base manifest", old)
		}
		return strings.Replace(base, old, replacement, 1)
	}

	tests := []struct {
		name     string
		manifest string
		want     []string // "<ErrorType> <field path>", order doesn't matter
	}{
		{
			name:     "valid",
			manifest: base,
		},
		{
			name:     "wrong kind returns early",
			manifest: edit("kind: ForgeService", "kind: Deployment"),
			want:     []string{fmt.Sprintf("%s kind", field.ErrorTypeInvalid)},
		},
		{
			name:     "wrong apiVersion",
			manifest: edit("forge.dev/v1alpha1", "forge.dev/v1"),
			want:     []string{fmt.Sprintf("%s apiVersion", field.ErrorTypeInvalid)},
		},
		{
			name:     "missing name",
			manifest: edit("  name: payments\n", "  labels: {}\n"),
			want:     []string{fmt.Sprintf("%s metadata.name", field.ErrorTypeRequired)},
		},
		{
			name:     "unknown field is pruned and reported",
			manifest: edit("    liveness: /health/live\n", "    liveness: /health/live\n    startup: /health/start\n"),
			want:     []string{fmt.Sprintf("%s spec.health.startup", field.ErrorTypeForbidden)},
		},
		{
			name:     "wrong type",
			manifest: edit("owner: payments-team", "owner: 42"),
			want:     []string{fmt.Sprintf("%s spec.owner", field.ErrorTypeTypeInvalid)},
		},
		{
			name:     "missing required field",
			manifest: edit("  owner: payments-team\n", ""),
			want:     []string{fmt.Sprintf("%s spec.owner", field.ErrorTypeRequired)},
		},
		{
			name:     "CEL: memory limit below request",
			manifest: edit("memory: 128Mi", "memory: 16Mi"),
			want:     []string{fmt.Sprintf("%s spec.resources", field.ErrorTypeInvalid)},
		},
		{
			name:     "CEL: maxReplicas below minReplicas",
			manifest: edit("maxReplicas: 5", "maxReplicas: 1"),
			want:     []string{fmt.Sprintf("%s spec.scaling", field.ErrorTypeInvalid)},
		},
		{
			name:     "CEL: metrics port equals runtime port",
			manifest: edit("metricsPort: 9090", "metricsPort: 8080"),
			want:     []string{fmt.Sprintf("%s spec", field.ErrorTypeInvalid)},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			errs, err := v.Validate(context.Background(), []byte(tc.manifest))
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			var got []string
			for _, e := range errs {
				got = append(got, fmt.Sprintf("%s %s", e.Type, e.Field))
			}
			slices.Sort(got)
			slices.Sort(tc.want)

			if !slices.Equal(got, tc.want) {
				t.Errorf("errors mismatch\n got: %v\nwant: %v\nfull: %v", got, tc.want, errs)
			}
		})
	}
}

// Unreadable or empty input is an error, not a validation result.
func TestValidateUnparseable(t *testing.T) {
	v, err := New(crd.ForgeService)
	if err != nil {
		t.Fatalf("New: %v", err)
	}

	for name, input := range map[string]string{
		"empty":         "",
		"comments only": "# nothing here\n",
		"invalid YAML":  "spec: [unclosed",
	} {
		t.Run(name, func(t *testing.T) {
			if _, err := v.Validate(context.Background(), []byte(input)); err == nil {
				t.Fatal("expected an error, got nil")
			}
		})
	}
}
