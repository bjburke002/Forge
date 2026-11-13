package validate

import (
	"testing"

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
