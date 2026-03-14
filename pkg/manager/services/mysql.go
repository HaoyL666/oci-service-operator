package services

import (
	mysqlcontrollers "github.com/oracle/oci-service-operator/controllers/mysql"
	"github.com/oracle/oci-service-operator/pkg/core"
	"github.com/oracle/oci-service-operator/pkg/loggerutil"
	"github.com/oracle/oci-service-operator/pkg/manager"
	"github.com/oracle/oci-service-operator/pkg/servicemanager/mysql/dbsystem"
	ctrl "sigs.k8s.io/controller-runtime"
)

// MySQL registers the MySQL DB System reconciler with the shared manager.
func MySQL() manager.RegisterFunc {
	return func(mgr ctrl.Manager, deps *manager.Dependencies) error {
		if err := (&mysqlcontrollers.MySqlDBsystemReconciler{
			Reconciler: &core.BaseReconciler{
				Client:             mgr.GetClient(),
				OSOKServiceManager: dbsystem.NewDbSystemServiceManager(deps.Provider, deps.CredClient, deps.Scheme, loggerutil.OSOKLogger{Logger: ctrl.Log.WithName("service-manager").WithName("MySqlDbSystem")}),
				Finalizer:          core.NewBaseFinalizer(mgr.GetClient(), ctrl.Log),
				Log:                loggerutil.OSOKLogger{Logger: ctrl.Log.WithName("controllers").WithName("MySqlDbSystem")},
				Metrics:            deps.Metrics,
				Recorder:           mgr.GetEventRecorderFor("MySqlDbSystem"),
				Scheme:             deps.Scheme,
			},
		}).SetupWithManager(mgr); err != nil {
			return err
		}
		return nil
	}
}
