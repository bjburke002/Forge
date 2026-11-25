package ecs

import (
	"bytes"
	"flag"
	"os"
	"path/filepath"
	"testing"

	"github.com/bjburke002/forge/pkg/contract/v1alpha1"
)

var update = flag.Bool("update", false, "rewrite testdata golden files")

func TestRenderGolden(t *testing.T) {
	for env, host := range map[string]string{
		"dev":  "payments-dev.forgeappservices.com",
		"prod": "payments.forgeappservices.com",
	} {
		t.Run(env, func(t *testing.T) {
			tmpl, err := Render(withIngress(host, "/"), env)
			if err != nil {
				t.Fatal(err)
			}
			got, err := tmpl.JSON()
			if err != nil {
				t.Fatal(err)
			}

			golden := filepath.Join("testdata", "payments-"+env+".json")
			if *update {
				if err := os.WriteFile(golden, got, 0o644); err != nil {
					t.Fatal(err)
				}
				return
			}
			if want, err := os.ReadFile(golden); err != nil || !bytes.Equal(got, want) {
				t.Errorf("%s doesn't match; rerun with -update if intended\n%s", golden, got)
			}
		})
	}
}

func TestRenderUnsupportedEnvironment(t *testing.T) {
	if _, err := Render(withIngress("payments-staging.forgeappservices.com", "/"), "staging"); err == nil {
		t.Fatal("want an error for staging")
	}
}

func TestServiceConnectNamespaceParameter(t *testing.T) {
	// Each environment resolves its own namespace.
	for _, env := range []string{"dev", "prod"} {
		want := "/forge/ecs/service-connect-namespace-" + env
		if got := platformParameters(env)[paramServiceConnectNamespace].Default; got != want {
			t.Errorf("%s: default = %v, want %s", env, got, want)
		}
	}
}

func TestRenderOutputs(t *testing.T) {
	// Every service gets its Service Connect URL but only a service with ingress gets a public one.
	for name, tt := range map[string]struct {
		fsvc    *v1alpha1.ForgeService
		wantURL bool
	}{
		"ingress":    {withIngress(testHost, "/"), true},
		"no ingress": {testService(), false},
	} {
		t.Run(name, func(t *testing.T) {
			tmpl, err := Render(tt.fsvc, "dev")
			if err != nil {
				t.Fatal(err)
			}
			if got := tmpl.Outputs["InternalURL"].Value; got != "http://payments" {
				t.Errorf("InternalURL = %v, want http://payments", got)
			}
			if _, ok := tmpl.Outputs["URL"]; ok != tt.wantURL {
				t.Errorf("URL output present = %v, want %v", ok, tt.wantURL)
			}
		})
	}
}
