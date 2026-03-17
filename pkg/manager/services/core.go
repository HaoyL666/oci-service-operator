package services

import (
	corecontrollers "github.com/oracle/oci-service-operator/controllers/core"
	"github.com/oracle/oci-service-operator/pkg/core"
	"github.com/oracle/oci-service-operator/pkg/loggerutil"
	"github.com/oracle/oci-service-operator/pkg/manager"
	"github.com/oracle/oci-service-operator/pkg/servicemanager/core/internetgateway"
	"github.com/oracle/oci-service-operator/pkg/servicemanager/core/natgateway"
	"github.com/oracle/oci-service-operator/pkg/servicemanager/core/networksecuritygroup"
	"github.com/oracle/oci-service-operator/pkg/servicemanager/core/routetable"
	"github.com/oracle/oci-service-operator/pkg/servicemanager/core/securitylist"
	"github.com/oracle/oci-service-operator/pkg/servicemanager/core/subnet"
	"github.com/oracle/oci-service-operator/pkg/servicemanager/core/vcn"
	ctrl "sigs.k8s.io/controller-runtime"
)

// Core registers the core networking reconcilers with the shared manager.
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
		if err := (&corecontrollers.SubnetReconciler{
			Reconciler: &core.BaseReconciler{
				Client: mgr.GetClient(),
				OSOKServiceManager: subnet.NewSubnetServiceManager(deps.Provider, deps.CredClient, deps.Scheme,
					loggerutil.OSOKLogger{Logger: ctrl.Log.WithName("service-manager").WithName("Subnet")}, deps.Metrics),
				Finalizer: core.NewBaseFinalizer(mgr.GetClient(), ctrl.Log),
				Log:       loggerutil.OSOKLogger{Logger: ctrl.Log.WithName("controllers").WithName("Subnet")},
				Metrics:   deps.Metrics,
				Recorder:  mgr.GetEventRecorderFor("Subnet"),
				Scheme:    deps.Scheme,
			},
		}).SetupWithManager(mgr); err != nil {
			return err
		}
		if err := (&corecontrollers.RouteTableReconciler{
			Reconciler: &core.BaseReconciler{
				Client: mgr.GetClient(),
				OSOKServiceManager: routetable.NewRouteTableServiceManager(deps.Provider, deps.CredClient, deps.Scheme,
					loggerutil.OSOKLogger{Logger: ctrl.Log.WithName("service-manager").WithName("RouteTable")}, deps.Metrics),
				Finalizer: core.NewBaseFinalizer(mgr.GetClient(), ctrl.Log),
				Log:       loggerutil.OSOKLogger{Logger: ctrl.Log.WithName("controllers").WithName("RouteTable")},
				Metrics:   deps.Metrics,
				Recorder:  mgr.GetEventRecorderFor("RouteTable"),
				Scheme:    deps.Scheme,
			},
		}).SetupWithManager(mgr); err != nil {
			return err
		}
		if err := (&corecontrollers.InternetGatewayReconciler{
			Reconciler: &core.BaseReconciler{
				Client: mgr.GetClient(),
				OSOKServiceManager: internetgateway.NewInternetGatewayServiceManager(deps.Provider, deps.CredClient, deps.Scheme,
					loggerutil.OSOKLogger{Logger: ctrl.Log.WithName("service-manager").WithName("InternetGateway")}, deps.Metrics),
				Finalizer: core.NewBaseFinalizer(mgr.GetClient(), ctrl.Log),
				Log:       loggerutil.OSOKLogger{Logger: ctrl.Log.WithName("controllers").WithName("InternetGateway")},
				Metrics:   deps.Metrics,
				Recorder:  mgr.GetEventRecorderFor("InternetGateway"),
				Scheme:    deps.Scheme,
			},
		}).SetupWithManager(mgr); err != nil {
			return err
		}
		if err := (&corecontrollers.NatGatewayReconciler{
			Reconciler: &core.BaseReconciler{
				Client: mgr.GetClient(),
				OSOKServiceManager: natgateway.NewNatGatewayServiceManager(deps.Provider, deps.CredClient, deps.Scheme,
					loggerutil.OSOKLogger{Logger: ctrl.Log.WithName("service-manager").WithName("NatGateway")}, deps.Metrics),
				Finalizer: core.NewBaseFinalizer(mgr.GetClient(), ctrl.Log),
				Log:       loggerutil.OSOKLogger{Logger: ctrl.Log.WithName("controllers").WithName("NatGateway")},
				Metrics:   deps.Metrics,
				Recorder:  mgr.GetEventRecorderFor("NatGateway"),
				Scheme:    deps.Scheme,
			},
		}).SetupWithManager(mgr); err != nil {
			return err
		}
		if err := (&corecontrollers.SecurityListReconciler{
			Reconciler: &core.BaseReconciler{
				Client: mgr.GetClient(),
				OSOKServiceManager: securitylist.NewSecurityListServiceManager(deps.Provider, deps.CredClient, deps.Scheme,
					loggerutil.OSOKLogger{Logger: ctrl.Log.WithName("service-manager").WithName("SecurityList")}, deps.Metrics),
				Finalizer: core.NewBaseFinalizer(mgr.GetClient(), ctrl.Log),
				Log:       loggerutil.OSOKLogger{Logger: ctrl.Log.WithName("controllers").WithName("SecurityList")},
				Metrics:   deps.Metrics,
				Recorder:  mgr.GetEventRecorderFor("SecurityList"),
				Scheme:    deps.Scheme,
			},
		}).SetupWithManager(mgr); err != nil {
			return err
		}
		if err := (&corecontrollers.NetworkSecurityGroupReconciler{
			Reconciler: &core.BaseReconciler{
				Client: mgr.GetClient(),
				OSOKServiceManager: networksecuritygroup.NewNetworkSecurityGroupServiceManager(deps.Provider, deps.CredClient, deps.Scheme,
					loggerutil.OSOKLogger{Logger: ctrl.Log.WithName("service-manager").WithName("NetworkSecurityGroup")}, deps.Metrics),
				Finalizer: core.NewBaseFinalizer(mgr.GetClient(), ctrl.Log),
				Log:       loggerutil.OSOKLogger{Logger: ctrl.Log.WithName("controllers").WithName("NetworkSecurityGroup")},
				Metrics:   deps.Metrics,
				Recorder:  mgr.GetEventRecorderFor("NetworkSecurityGroup"),
				Scheme:    deps.Scheme,
			},
		}).SetupWithManager(mgr); err != nil {
			return err
		}
		return nil
	}
}
