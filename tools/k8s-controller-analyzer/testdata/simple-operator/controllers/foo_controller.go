package controllers

import (
	"context"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	apimeta "k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	examplev1alpha1 "example.com/simple-operator/api/v1alpha1"
)

// FooReconciler reconciles a Foo object.
type FooReconciler struct {
	client.Client
}

// +kubebuilder:rbac:groups=example.com,resources=foos,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=example.com,resources=foos/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=apps,resources=deployments,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups="",resources=services,verbs=get;list;watch;create;update;patch;delete

func (r *FooReconciler) Reconcile(ctx context.Context, req reconcile.Request) (reconcile.Result, error) {
	var foo examplev1alpha1.Foo
	if err := r.Get(ctx, req.NamespacedName, &foo); err != nil {
		return reconcile.Result{}, err
	}

	// Add finalizer before creating external resources
	if !controllerutil.ContainsFinalizer(&foo, "example.com/cleanup") {
		controllerutil.AddFinalizer(&foo, "example.com/cleanup")
		if err := r.Update(ctx, &foo); err != nil {
			return reconcile.Result{}, err
		}
	}

	// Handle deletion
	if !foo.DeletionTimestamp.IsZero() {
		if err := r.Delete(ctx, &appsv1.Deployment{}); err != nil {
			return reconcile.Result{}, err
		}
		controllerutil.RemoveFinalizer(&foo, "example.com/cleanup")
		if err := r.Update(ctx, &foo); err != nil {
			return reconcile.Result{}, err
		}
		return reconcile.Result{}, nil
	}

	// Create deployment
	deploy := &appsv1.Deployment{}
	if err := r.Create(ctx, deploy); err != nil {
		apimeta.SetStatusCondition(&foo.Status.Conditions, metav1.Condition{
			Type:   "Degraded",
			Status: metav1.ConditionTrue,
		})
		return reconcile.Result{}, err
	}

	// Set ready condition
	apimeta.SetStatusCondition(&foo.Status.Conditions, metav1.Condition{
		Type:   "Ready",
		Status: metav1.ConditionTrue,
	})
	if err := r.Status().Update(ctx, &foo); err != nil {
		return reconcile.Result{}, err
	}

	return reconcile.Result{RequeueAfter: 30}, nil
}

func (r *FooReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&examplev1alpha1.Foo{}).
		Owns(&appsv1.Deployment{}).
		Owns(&corev1.Service{}).
		Complete(r)
}
