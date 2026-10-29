package controller

import (
	"context"

	"github.com/bjburke002/forge/internal/controller/resources"
	"github.com/bjburke002/forge/pkg/contract/v1alpha1"
	appsv1 "k8s.io/api/apps/v1"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime"
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
// +kubebuilder:rbac:groups="",resources=serviceaccounts;services,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=apps,resources=deployments,verbs=get;list;watch;create;update;patch;delete

func (r *ForgeServiceReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	logger := log.FromContext(ctx)

	var fs v1alpha1.ForgeService
	if err := r.Get(ctx, req.NamespacedName, &fs); err != nil {
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}

	logger.Info("reconciling", "name", fs.Name, "image", fs.Spec.Image)

	// Apply the Service Account, Service and Deployment in that order
	objs := []runtime.ApplyConfiguration{
		resources.RenderServiceAccount(&fs),
		resources.RenderService(&fs),
		resources.RenderDeployment(&fs),
	}
	for _, obj := range objs {
		if err := r.Apply(ctx, obj, client.FieldOwner(resources.FieldOwner), client.ForceOwnership); err != nil {
			return ctrl.Result{}, err
		}
	}
	return ctrl.Result{}, nil

}

func (r *ForgeServiceReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		// For is the primary type, changes to this trigger a reconcile.
		// Owns is the child type. Changes to these trigger a reconcile of the parent (in For())
		For(&v1alpha1.ForgeService{}).
		Owns(&corev1.ServiceAccount{}).
		Owns(&corev1.Service{}).
		Owns(&appsv1.Deployment{}).
		Complete(r)
}
