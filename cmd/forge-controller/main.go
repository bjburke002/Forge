package main

import (
	"flag"
	"net/netip"
	"os"
	"strings"

	"github.com/bjburke002/forge/internal/controller"
	"github.com/bjburke002/forge/internal/controller/resources"
	forgev1alpha1 "github.com/bjburke002/forge/pkg/contract/v1alpha1"
	monitoringv1 "github.com/prometheus-operator/prometheus-operator/pkg/apis/monitoring/v1"
	"k8s.io/apimachinery/pkg/runtime"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/healthz"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"
	metricsserver "sigs.k8s.io/controller-runtime/pkg/metrics/server"
)

func main() {
	// Defining flags
	var (
		metricsAddr          string
		probeAddr            string
		enableLeaderElection bool
		leaderElectionNS     string
	)

	// Flag to set the Prometheus endpoint for the manager
	flag.StringVar(&metricsAddr, "metrics-bind-address", ":8080",
		"Address which the metrics endpoint binds to. Use 0 to disable.")

	// Flag to set /healthz and /readyz for the Deployment's probes
	flag.StringVar(&probeAddr, "health-probe-bind-address", ":8081",
		"Address which the health and readiness probes bind to.")

	// Flag will pass --leader-elect=true
	flag.BoolVar(&enableLeaderElection, "leader-elect", false,
		"Enable Leader election, so only one replica reconciles at any given time.")

	// Inside the cluster, the manager is going to read its own namespace from the pod. This works around locally.
	flag.StringVar(&leaderElectionNS, "leader-election-namespace", "forge-system",
		"Namespace that holds the leader election lease.")

	// Flags to set ALB CIDR ranges and Monitoring namespace
	var albSubnetCIDRs, monitoringNS string
	flag.StringVar(&albSubnetCIDRs, "alb-subnet-cidrs", "",
		"Comma-separated CIDRs of the ALB subnets. Required.")
	flag.StringVar(&monitoringNS, "monitoring-namespace", "",
		"Namespace Prometheus scrapes from. Required.")

	// Logging flags for zap
	opts := zap.Options{Development: true}
	opts.BindFlags(flag.CommandLine)
	flag.Parse()

	// Set logger to use zap with the bound flags
	ctrl.SetLogger(zap.New(zap.UseFlagOptions(&opts)))

	// Creating the actual logger
	setupLog := ctrl.Log.WithName("setup")

	// Validating that ALB CIDRS and monitoring NS have been passed and are valid
	var cidrs []string
	for _, c := range strings.Split(albSubnetCIDRs, ",") {
		c = strings.TrimSpace(c)
		if _, err := netip.ParsePrefix(c); err != nil {
			setupLog.Error(err, "Invalid --alb-subnet-cidrs", "cidr", c)
			os.Exit(1)
		}
		cidrs = append(cidrs, c)
	}
	if monitoringNS == "" {
		setupLog.Error(nil, "--monitoring-namespace is required")
		os.Exit(1)
	}

	scheme := runtime.NewScheme()

	// Add built-in client-go types to scheme
	if err := clientgoscheme.AddToScheme(scheme); err != nil {
		setupLog.Error(err, "Unable to add Kubernetes types to scheme")
		os.Exit(1)
	}

	// Adding Prom monitoring types to scheme
	if err := monitoringv1.AddToScheme(scheme); err != nil {
		setupLog.Error(err, "Unable to add Prometheus types to scheme")
		os.Exit(1)
	}

	// Adding Forge types to the scheme
	if err := forgev1alpha1.AddToScheme(scheme); err != nil {
		setupLog.Error(err, "Unable to add Forge types to the scheme.")
		os.Exit(1)
	}

	// Creating the client manager
	mgr, err := ctrl.NewManager(ctrl.GetConfigOrDie(), ctrl.Options{
		Scheme:                        scheme,
		Metrics:                       metricsserver.Options{BindAddress: metricsAddr},
		HealthProbeBindAddress:        probeAddr,
		LeaderElection:                enableLeaderElection,
		LeaderElectionNamespace:       leaderElectionNS,
		LeaderElectionID:              "forge-controller.forge.dev",
		LeaderElectionReleaseOnCancel: true,
	})
	if err != nil {
		setupLog.Error(err, "Unable to create the manager")
		os.Exit(1)
	}

	reconciler := &controller.ForgeServiceReconciler{
		Client: mgr.GetClient(),
		Platform: resources.Platform{
			ALBSubnetCIDRS:      cidrs,
			MonitoringNamespace: monitoringNS,
		},
	}
	if err := reconciler.SetupWithManager(mgr); err != nil {
		setupLog.Error(err, "Unable to set up the controller")
		os.Exit(1)
	}

	if err := mgr.AddHealthzCheck("healthz", healthz.Ping); err != nil {
		setupLog.Error(err, "Unable to set up health check")
		os.Exit(1)
	}

	if err := mgr.AddReadyzCheck("readyz", healthz.Ping); err != nil {
		setupLog.Error(err, "Unable to set up Readiness check")
		os.Exit(1)
	}

	setupLog.Info("Starting manager...")

	if err := mgr.Start(ctrl.SetupSignalHandler()); err != nil {
		setupLog.Error(err, "Unable to start the manager..")
		os.Exit(1)
	}
}
