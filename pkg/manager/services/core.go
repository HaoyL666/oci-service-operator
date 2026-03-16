package services

import (
	corecontrollers "github.com/oracle/oci-service-operator/controllers/core"
	"github.com/oracle/oci-service-operator/pkg/core"
	"github.com/oracle/oci-service-operator/pkg/loggerutil"
	"github.com/oracle/oci-service-operator/pkg/manager"
	"github.com/oracle/oci-service-operator/pkg/servicemanager/core/vcn"
	ctrl "sigs.k8s.io/controller-runtime"
)

// Core registers the Core VCN reconciler with the shared manager.
func Core() manager.RegisterFunc {
	return func(mgr ctrl.Manager, deps *manager.Dependencies) error {
		if err := (&corecontrollers.VcnReconciler{
			Reconciler: &core.BaseReconciler{
				Client: mgr.GetClient(),
				OSOKServiceManager: vcn.NewVcnServiceManager(deps.Provider, deps.CredClient, deps.Scheme,
					loggerutil.OSOKLogger{Logger: ctrl.Log.WithName("service-manager").WithName("Vcn")}, deps.Metrics),
				Finalizer: core.NewBaseFinalizer(mgr.GetClient(), ctrl.Log),
				Log:       loggerutil.OSOKLogger{Logger: ctrl.Log.WithName("controllers").WithName("Vcn")},
				Metrics:   deps.Metrics,
				Recorder:  mgr.GetEventRecorderFor("Vcn"),
				Scheme:    deps.Scheme,
			},
		}).SetupWithManager(mgr); err != nil {
			return err
		}
		return nil
	}
}
