package manager

import (
	"flag"
	"fmt"

	"github.com/oracle/oci-go-sdk/v65/common"
	"github.com/oracle/oci-service-operator/go_ensurefips"
	"github.com/oracle/oci-service-operator/pkg/authhelper"
	"github.com/oracle/oci-service-operator/pkg/config"
	"github.com/oracle/oci-service-operator/pkg/credhelper/kubesecret"
	"github.com/oracle/oci-service-operator/pkg/loggerutil"
	"github.com/oracle/oci-service-operator/pkg/metrics"
	"github.com/oracle/oci-service-operator/pkg/util"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/healthz"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"
	"sigs.k8s.io/controller-runtime/pkg/metrics/server"
)

// RegisterFunc wires controllers, webhooks, and supporting components into the shared manager instance.
type RegisterFunc func(ctrl.Manager, *Dependencies) error

// Dependencies bundles common clients initialised by Run that individual services can reuse.
type Dependencies struct {
	Provider   common.ConfigurationProvider
	CredClient *kubesecret.KubeSecretClient
	Metrics    *metrics.Metrics
	Scheme     *runtime.Scheme
}

// Options configure shared manager behaviour.
type Options struct {
	// Scheme is the runtime scheme populated by the caller with the APIs served by this binary.
	Scheme *runtime.Scheme
	// MetricsServiceName is used when initialising the metrics collector for this manager.
	MetricsServiceName string
	// LeaderElectionID identifies the leader election record used by this manager.
	LeaderElectionID string
}

const (
	defaultLeaderElectionID = "40558063.oci"
	defaultMetricsService   = "osok"
)

// Run bootstraps the shared controller-runtime manager and delegates controller registration to the supplied hooks.
func Run(opts Options, registrars ...RegisterFunc) error {
	if opts.Scheme == nil {
		return fmt.Errorf("manager: scheme must be provided")
	}
	if opts.LeaderElectionID == "" {
		opts.LeaderElectionID = defaultLeaderElectionID
	}
	if opts.MetricsServiceName == "" {
		opts.MetricsServiceName = defaultMetricsService
	}

	go_ensurefips.Compliant()
	common.EnableInstanceMetadataServiceLookup()

	var (
		configFile           string
		metricsAddr          string
		enableLeaderElection bool
		probeAddr            string
		initOSOKResources    bool
	)

	flag.StringVar(&configFile, "config", "",
		"The controller will load its initial configuration from this file. "+
			"Omit this flag to use the default configuration values. "+
			"Command-line flags override configuration from this file.")
	flag.StringVar(&metricsAddr, "metrics-bind-address", ":8080", "The address the metric endpoint binds to.")
	flag.StringVar(&probeAddr, "health-probe-bind-address", ":8081", "The address the health probe endpoint binds to.")
	flag.BoolVar(&enableLeaderElection, "leader-elect", true,
		"Enable leader election for controller manager. Enabling this will ensure there is only one active controller manager.")
	flag.BoolVar(&initOSOKResources, "init-osok-resources", false,
		"Install OSOK prerequisites like CRDs and Webhooks at manager bootup")

	zapOpts := zap.Options{
		Development: true,
	}
	zapOpts.BindFlags(flag.CommandLine)
	flag.Parse()

	ctrl.SetLogger(zap.New(zap.UseFlagOptions(&zapOpts)))

	setupLog := loggerutil.OSOKLogger{Logger: ctrl.Log.WithName("setup")}

	var (
		err     error
		options ctrl.Options
	)

	options = ctrl.Options{
		Scheme:                 opts.Scheme,
		Metrics:                server.Options{BindAddress: metricsAddr},
		HealthProbeBindAddress: probeAddr,
		LeaderElection:         enableLeaderElection,
		LeaderElectionID:       opts.LeaderElectionID,
	}

	if configFile != "" {
		setupLog.InfoLog("Loading the configuration from the ControllerManagerConfig configMap")
		options, err = options.AndFrom(ctrl.ConfigFile().AtPath(configFile))
		if err != nil {
			setupLog.ErrorLog(err, "unable to load the config file")
			return err
		}
	} else {
		setupLog.InfoLog("Loading the configuration from the command arguments")
	}

	mgr, err := ctrl.NewManager(ctrl.GetConfigOrDie(), options)
	if err != nil {
		setupLog.ErrorLog(err, "unable to start manager")
		return err
	}

	if initOSOKResources {
		util.InitOSOK(mgr.GetConfig(), loggerutil.OSOKLogger{Logger: ctrl.Log.WithName("setup").WithName("initOSOK")})
	}

	setupLog.InfoLog("Getting the config details")
	osokCfg := config.GetConfigDetails(loggerutil.OSOKLogger{Logger: ctrl.Log.WithName("setup").WithName("config")})

	authConfigProvider := &authhelper.AuthConfigProvider{
		Log: loggerutil.OSOKLogger{Logger: ctrl.Log.WithName("setup").WithName("config")},
	}

	provider, err := authConfigProvider.GetAuthProvider(osokCfg)
	if err != nil {
		setupLog.ErrorLog(err, "unable to get the oci configuration provider. Exiting setup")
		return err
	}

	metricsClient := metrics.Init(opts.MetricsServiceName, loggerutil.OSOKLogger{Logger: ctrl.Log.WithName("metrics")})

	credClient := &kubesecret.KubeSecretClient{
		Client:  mgr.GetClient(),
		Log:     loggerutil.OSOKLogger{Logger: ctrl.Log.WithName("credential-helper").WithName("KubeSecretClient")},
		Metrics: metricsClient,
	}

	deps := &Dependencies{
		Provider:   provider,
		CredClient: credClient,
		Metrics:    metricsClient,
		Scheme:     opts.Scheme,
	}

	for _, register := range registrars {
		if register == nil {
			continue
		}
		if err := register(mgr, deps); err != nil {
			setupLog.ErrorLog(err, "unable to register controller")
			return err
		}
	}

	if err := mgr.AddHealthzCheck("health", healthz.Ping); err != nil {
		setupLog.ErrorLog(err, "unable to set up health check")
		return err
	}
	if err := mgr.AddReadyzCheck("check", healthz.Ping); err != nil {
		setupLog.ErrorLog(err, "unable to set up ready check")
		return err
	}

	setupLog.InfoLog("starting manager")
	if err := mgr.Start(ctrl.SetupSignalHandler()); err != nil {
		setupLog.ErrorLog(err, "problem running manager")
		return err
	}
	return nil
}
