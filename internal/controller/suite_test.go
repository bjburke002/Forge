package controller

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"

	forgev1alpha1 "github.com/bjburke002/forge/pkg/contract/v1alpha1"
	monitoringv1 "github.com/prometheus-operator/prometheus-operator/pkg/apis/monitoring/v1"
	"k8s.io/apimachinery/pkg/runtime"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/envtest"
)

var (
	k8sClient client.Client
	testEnv   *envtest.Environment
)

func TestMain(m *testing.M) {
	testEnv = &envtest.Environment{
		CRDDirectoryPaths: []string{
			filepath.Join("..", "..", "config", "crd"),
			filepath.Join("testdata", "crds"),
		},
		ErrorIfCRDPathMissing: true,
	}

	// Start etcd and kube-apiserver
	cfg, err := testEnv.Start()
	if err != nil {
		fmt.Fprintln(os.Stderr, "starting envtest:", err)
		os.Exit(1)
	}

	// Same scheme as main.go
	scheme := runtime.NewScheme()
	for _, add := range []func(*runtime.Scheme) error{
		clientgoscheme.AddToScheme,
		forgev1alpha1.AddToScheme,
		monitoringv1.AddToScheme,
	} {
		if err := add(scheme); err != nil {
			fmt.Fprintln(os.Stderr, "building scheme:", err)
			stopAndExit(1)
		}
	}

	// Initialize a controller-runtime client
	k8sClient, err = client.New(cfg, client.Options{Scheme: scheme})
	if err != nil {
		fmt.Fprintln(os.Stderr, "creating client:", err)
		stopAndExit(1)
	}

	// Run test suite
	stopAndExit(m.Run())
}

// stopAndExit stops kube-apiserver and etcd before exiting, so a failed setup
// doesn't leave them running in the background.
func stopAndExit(code int) {
	if err := testEnv.Stop(); err != nil {
		fmt.Fprintln(os.Stderr, "stopping envtest:", err)
		if code == 0 {
			code = 1
		}
	}
	os.Exit(code)
}
