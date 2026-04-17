package main

import (
	"os"

	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"

	examplev1alpha1 "example.com/simple-operator/api/v1alpha1"
	"example.com/simple-operator/controllers"
)

var scheme = runtime.NewScheme()

func init() {
	utilruntime.Must(examplev1alpha1.AddToScheme(scheme))
}

func main() {
	mgr, err := ctrl.NewManager(ctrl.GetConfigOrDie(), ctrl.Options{Scheme: scheme})
	if err != nil {
		os.Exit(1)
	}

	if err := (&controllers.FooReconciler{}).SetupWithManager(mgr); err != nil {
		os.Exit(1)
	}

	if err := mgr.Start(ctrl.SetupSignalHandler()); err != nil {
		os.Exit(1)
	}
}
