package services

import (
	databasev1beta1 "github.com/oracle/oci-service-operator/api/database/v1beta1"
	databasecontrollers "github.com/oracle/oci-service-operator/controllers/database"
	"github.com/oracle/oci-service-operator/pkg/core"
	"github.com/oracle/oci-service-operator/pkg/loggerutil"
	"github.com/oracle/oci-service-operator/pkg/manager"
	"github.com/oracle/oci-service-operator/pkg/servicemanager/autonomousdatabases/adb"
	ctrl "sigs.k8s.io/controller-runtime"
)

// Database registers the Autonomous Database reconcilers and webhooks with the shared manager.
func Database() manager.RegisterFunc {
	return func(mgr ctrl.Manager, deps *manager.Dependencies) error {
		reconciler := &core.BaseReconciler{
			Client:             mgr.GetClient(),
			OSOKServiceManager: adb.NewAdbServiceManager(deps.Provider, deps.CredClient, deps.Scheme, loggerutil.OSOKLogger{Logger: ctrl.Log.WithName("service-manager").WithName("AutonomousDatabases")}),
			Finalizer:          core.NewBaseFinalizer(mgr.GetClient(), ctrl.Log),
			Log:                loggerutil.OSOKLogger{Logger: ctrl.Log.WithName("controllers").WithName("AutonomousDatabases")},
			Metrics:            deps.Metrics,
			Recorder:           mgr.GetEventRecorderFor("AutonomousDatabases"),
			Scheme:             deps.Scheme,
		}

		if err := (&databasecontrollers.AutonomousDatabasesReconciler{
			Reconciler: reconciler,
		}).SetupWithManager(mgr); err != nil {
			return err
		}
		if err := (&databasev1beta1.AutonomousDatabases{}).SetupWebhookWithManager(mgr); err != nil {
			return err
		}
		return nil
	}
}
