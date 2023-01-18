package main

import (
	"flag"
	"os"

	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/kubernetes"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/healthz"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"
	"sigs.k8s.io/controller-runtime/pkg/webhook"

	"github.com/bebc/webhook-tls/pkg"
)

var (
	probeAddr            string
	certDir              string
	namespace            string
	metricsAddr          string
	enableLeaderElection bool
	scheme               = runtime.NewScheme()
	setupLog             = ctrl.Log.WithName("monitoring-controller")

)

func main() {
	flag.StringVar(&metricsAddr, "metrics-bind-address", ":8080", "The address the metric endpoint binds to.")
	flag.StringVar(&probeAddr, "health-probe-bind-address", ":8081", "The address the probe endpoint binds to.")
	flag.StringVar(&certDir, "cert-dir", "/certs", "The directory where certs are stored, defaults to /certs")
	flag.StringVar(&namespace, "namespace", "transsion-monitoring", "The namespace of the operator")
	flag.BoolVar(&enableLeaderElection, "leader-elect", false,
		"Enable leader election for controller manager. "+
			"Enabling this will ensure there is only one active controller manager.")
	flag.BoolVar(&pkg.SelfSignedCa, "self-signed-ca", true, "generate a self-signed certificate")
	opts := zap.Options{
		Development: true,
	}
	opts.BindFlags(flag.CommandLine)
	flag.Parse()

	//configLog := uzap.NewProductionEncoderConfig()
	//configLog.EncodeTime = func(ts time.Time, encoder zapcore.PrimitiveArrayEncoder) {
	//	encoder.AppendString(ts.UTC().Format(time.RFC3339Nano))
	//}
	//logFmtEncoder := zaplogfmt.NewEncoder(configLog)
	//
	//ctrl.SetLogger(zap.New(zap.UseDevMode(true), zap.WriteTo(os.Stdout), zap.Encoder(logFmtEncoder)))
	//setupLog = ctrl.Log.WithName("monitoring-controller")
	ctrl.SetLogger(zap.New(zap.UseDevMode(true)))
	clientSet, err := kubernetes.NewForConfig(ctrl.GetConfigOrDie())
	if err != nil {
		setupLog.Error(err, "unable to init clientSet")
		os.Exit(1)
	}

	//ca
	webhookTls := pkg.NewWebHookTls(namespace, clientSet, certDir)
	err = webhookTls.RunWebHookTls()
	if err != nil {
		setupLog.Error(err, "unable to init webhookTls")
		os.Exit(1)
	}

	//server
	mgr, err := ctrl.NewManager(ctrl.GetConfigOrDie(), ctrl.Options{
		Scheme:                 scheme,
		MetricsBindAddress:     metricsAddr,
		Port:                   9443,
		LeaderElection:         enableLeaderElection,
		HealthProbeBindAddress: probeAddr,
		LeaderElectionID:       "3f08ca71.tmc-gitlab.bebc.com",
	})
	if err != nil {
		setupLog.Error(err, "unable to start manager")
		os.Exit(1)
	}

	if err = mgr.AddHealthzCheck("healthz", healthz.Ping); err != nil {
		setupLog.Error(err, "unable to set up health check")
		os.Exit(1)
	}
	if err = mgr.AddReadyzCheck("readyz", healthz.Ping); err != nil {
		setupLog.Error(err, "unable to set up ready check")
		os.Exit(1)
	}

	mgr.GetWebhookServer().CertDir = certDir
	mgr.GetWebhookServer().Register("/mutate", &webhook.Admission{Handler: &pkg.PodLabels{Client: mgr.GetClient(),
		Log: setupLog.WithName("webhook")}})

	setupLog.Info("starting manager")

	if err = mgr.Start(ctrl.SetupSignalHandler()); err != nil {
		setupLog.Error(err, "problem running manager")
		os.Exit(1)
	}
}
