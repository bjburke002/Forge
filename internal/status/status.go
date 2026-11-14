package status

import (
	"context"
	"fmt"

	"github.com/bjburke002/forge/pkg/contract/v1alpha1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/dynamic"
)

// Result is the full status of one service in one environment. Combines its Argo Application and Forge Service
type Result struct {
	Environment   string
	Deployed      bool
	ArgoSync      string
	ArgoHealth    string
	ArgoRevision  string
	FsvcReady     string
	FsvcAvailable int32
	FsvcImage     string
	FsvcURL       string
	Stale         bool
	Conditions    []string
}

var (
	aGVR         = schema.GroupVersionResource{Group: "argoproj.io", Version: "v1alpha1", Resource: "applications"}
	fGVR         = schema.GroupVersionResource{Group: "forge.dev", Version: "v1alpha1", Resource: "forgeservices"}
	environments = []string{"dev", "prod"}
)

// Get polls Kubernetes and returns status information for Argo Applications and Kubernetes resources
func Get(ctx context.Context, c dynamic.Interface, argoNamespace, name string) ([]Result, error) {
	results := make([]Result, 0, len(environments))
	for _, e := range environments {
		envName := name + "-" + e

		r := Result{Environment: e}

		argo, err := c.Resource(aGVR).Namespace(argoNamespace).Get(ctx, envName, metav1.GetOptions{})
		switch {
		case apierrors.IsNotFound(err):
			// No ArgoCD Application. Skip
		case err != nil:
			return nil, fmt.Errorf("getting Argo application %q: %w", envName, err)
		default:
			r.ArgoSync, _, _ = unstructured.NestedString(argo.Object, "status", "sync", "status")
			r.ArgoHealth, _, _ = unstructured.NestedString(argo.Object, "status", "health", "status")
			r.ArgoRevision, _, _ = unstructured.NestedString(argo.Object, "status", "sync", "revision")
		}

		fsvc, err := c.Resource(fGVR).Namespace(envName).Get(ctx, name, metav1.GetOptions{})
		switch {
		case apierrors.IsNotFound(err):
			r.Deployed = false
			results = append(results, r)
			continue
		case err != nil:
			return nil, fmt.Errorf("getting ForgeService %s/%s: %w", envName, name, err)
		default:
			r.Deployed = true
			// We need v1alpha1.ForgeService fields to fill out the struct, so need to convert the resource
			var v v1alpha1.ForgeService
			if err := runtime.DefaultUnstructuredConverter.FromUnstructured(fsvc.Object, &v); err != nil {
				return nil, fmt.Errorf("converting %s/%s to v1alpha1.ForgeService: %w", envName, name, err)
			}
			cond := meta.FindStatusCondition(v.Status.Conditions, v1alpha1.ConditionReady)
			if cond == nil {
				r.FsvcReady = "Unknown"
			} else {
				r.FsvcReady = string(cond.Status)
			}
			r.FsvcAvailable = v.Status.AvailableReplicas
			r.FsvcImage = v.Spec.Image
			r.FsvcURL = v.Status.URL
			r.Stale = v.Generation != v.Status.ObservedGeneration

			if cond != nil && cond.Status != metav1.ConditionTrue {
				r.Conditions = append(r.Conditions, fmt.Sprintf("%s: %s: %s", cond.Type, cond.Reason, cond.Message))
			}
			degraded := meta.FindStatusCondition(v.Status.Conditions, v1alpha1.ConditionDegraded)

			if degraded != nil && degraded.Status == metav1.ConditionTrue {
				r.Conditions = append(r.Conditions, fmt.Sprintf("%s: %s: %s", degraded.Type, degraded.Reason, degraded.Message))
			}
		}
		results = append(results, r)
	}
	return results, nil
}
