package controller

import (
	"context"

	"github.com/bjburke002/forge/internal/controller/resources"
	"github.com/bjburke002/forge/pkg/contract/v1alpha1"
	monitoringv1 "github.com/prometheus-operator/prometheus-operator/pkg/apis/monitoring/v1"
	appsv1 "k8s.io/api/apps/v1"
	autoscalingv2 "k8s.io/api/autoscaling/v2"
	corev1 "k8s.io/api/core/v1"
	networkingv1 "k8s.io/api/networking/v1"
	policyv1 "k8s.io/api/policy/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"
)

type ForgeServiceReconciler struct {
	client.Client
	Platform resources.Platform
}

// +kubebuilder:rbac:groups=forge.dev,resources=forgeservices,verbs=get;list;watch
// +kubebuilder:rbac:groups=forge.dev,resources=forgeservices/status,verbs=get;update;patch
// +kubebuilder:rbac:groups="";events.k8s.io,resources=events,verbs=create;patch
// +kubebuilder:rbac:groups="",resources=serviceaccounts;services,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=apps,resources=deployments,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=networking.k8s.io,resources=ingresses,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=autoscaling,resources=horizontalpodautoscalers,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=policy,resources=poddisruptionbudgets,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=networking.k8s.io,resources=networkpolicies,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=monitoring.coreos.com,resources=servicemonitors,verbs=get;list;watch;create;update;patch;delete

func (r *ForgeServiceReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	logger := log.FromContext(ctx)

	var fs v1alpha1.ForgeService
	if err := r.Get(ctx, req.NamespacedName, &fs); err != nil {
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}

	logger.Info("reconciling", "name", fs.Name, "image", fs.Spec.Image)

	// Apply the child resources in order
	objs := []runtime.ApplyConfiguration{
		resources.RenderServiceAccount(&fs),
		resources.RenderService(&fs),
		resources.RenderDeployment(&fs),
		resources.RenderAutoscaler(&fs),
		resources.RenderPodDisruptionBudget(&fs),
		resources.RenderNetworkPolicy(&fs, r.Platform),
		resources.RenderServiceMonitor(&fs),
	}

	// Handling empty or removed Ingresses
	// If the Ingress is provided in the fsvc Spec, render it
	if fs.Spec.Ingress != nil {
		objs = append(objs, resources.RenderIngress(&fs))
	} else {
		var ing networkingv1.Ingress
		// Lookup the Ingress in the ForgeService's namespace
		key := client.ObjectKey{Namespace: fs.Namespace, Name: fs.Name + "-ingress"}
		// If no Ingress found, move on. If found, proceed into the block
		if err := r.Get(ctx, key, &ing); err == nil {
			// Delete the Ingress but ONLY if it's controlled by the ForgeService
			if metav1.IsControlledBy(&ing, &fs) {
				if err := r.Delete(ctx, &ing); client.IgnoreNotFound(err) != nil {
					return ctrl.Result{}, err
				}
			}
		} else if !apierrors.IsNotFound(err) { // Any other error: return it so reconcile is retried
			return ctrl.Result{}, err
		}
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
		Owns(&autoscalingv2.HorizontalPodAutoscaler{}).
		Owns(&policyv1.PodDisruptionBudget{}).
		Owns(&networkingv1.Ingress{}).
		Owns(&networkingv1.NetworkPolicy{}).
		Owns(&monitoringv1.ServiceMonitor{}).
		Complete(r)
}
