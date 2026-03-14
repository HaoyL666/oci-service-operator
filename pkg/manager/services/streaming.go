package services

import (
	streamingcontrollers "github.com/oracle/oci-service-operator/controllers/streaming"
	"github.com/oracle/oci-service-operator/pkg/core"
	"github.com/oracle/oci-service-operator/pkg/loggerutil"
	"github.com/oracle/oci-service-operator/pkg/manager"
	"github.com/oracle/oci-service-operator/pkg/servicemanager/streams"
	ctrl "sigs.k8s.io/controller-runtime"
)

// Streaming registers the Streaming reconciler with the shared manager.
func Streaming() manager.RegisterFunc {
	return func(mgr ctrl.Manager, deps *manager.Dependencies) error {
		if err := (&streamingcontrollers.StreamReconciler{
			Reconciler: &core.BaseReconciler{
				Client: mgr.GetClient(),
				OSOKServiceManager: streams.NewStreamServiceManager(deps.Provider, deps.CredClient, deps.Scheme,
					loggerutil.OSOKLogger{Logger: ctrl.Log.WithName("service-manager").WithName("Streams")}, deps.Metrics),
				Finalizer: core.NewBaseFinalizer(mgr.GetClient(), ctrl.Log),
				Log:       loggerutil.OSOKLogger{Logger: ctrl.Log.WithName("controllers").WithName("Streams")},
				Metrics:   deps.Metrics,
				Recorder:  mgr.GetEventRecorderFor("Streams"),
				Scheme:    deps.Scheme,
			},
		}).SetupWithManager(mgr); err != nil {
			return err
		}
		return nil
	}
}
