package controller

import (
	"context"

	"github.com/bjburke002/forge/pkg/contract/v1alpha1"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"
)

type ForgeServiceReconciler struct {
	client.Client
}

// +kubebuilder:rbac:groups=forge.dev,resources=forgeservices,verbs=get;list;watch
// +kubebuilder:rbac:groups=forge.dev,resources=forgeservices/status,verbs=get;update;patch
// +kubebuilder:rbac:groups="";events.k8s.io,resources=events,verbs=create;patch

func (r *ForgeServiceReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	logger := log.FromContext(ctx)

	var fs v1alpha1.ForgeService
	if err := r.Get(ctx, req.NamespacedName, &fs); err != nil {
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}

	logger.Info("reconciling", "name", fs.Name, "image", fs.Spec.Image)
	return ctrl.Result{}, nil
}

func (r *ForgeServiceReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).For(&v1alpha1.ForgeService{}).Complete(r)
}
