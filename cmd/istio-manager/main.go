package main

import (
	"os"

	"github.com/alecthomas/kingpin"

	"k8s.io/client-go/kubernetes/scheme"
	_ "k8s.io/client-go/plugin/pkg/client/auth/gcp" // this is required to auth against GCP

	"sigs.k8s.io/controller-runtime/pkg/client/config"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/webhook"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"

	"github.com/gocardless/theatre/cmd"
	"github.com/gocardless/theatre/pkg/apis"
	"github.com/gocardless/theatre/pkg/istio"
	"github.com/gocardless/theatre/pkg/signals"
)

var (
	app            = kingpin.New("istio-manager", "Manages extended features for istio enabled pods").Version(cmd.VersionStanza())
	namespace      = app.Flag("namespace", "Kubernetes webhook service namespace").Default("theatre-system").String()
	serviceName    = app.Flag("service-name", "Name of service for webhook").Default("theatre-istio-manager").String()
	webhookName    = app.Flag("webhook-name", "Name of webhook").Default("theatre-istio").String()
	theatreImage   = app.Flag("theatre-image", "Set to the same image as current binary").Required().String()
	installPath    = app.Flag("install-path", "Location to install istio related binaries").Default("/var/run/istio").String()
	namespaceLabel = app.Flag("namespace-label", "Namespace label that enables webhook to operate on").Default("istio-injection").String()

	commonOpts = cmd.NewCommonOptions(app).WithMetrics(app)
)

func main() {
	kingpin.MustParse(app.Parse(os.Args[1:]))
	logger := commonOpts.Logger()

	if err := apis.AddToScheme(scheme.Scheme); err != nil {
		app.Fatalf("failed to add schemes: %v", err)
	}

	go func() {
		commonOpts.ListenAndServeMetrics(logger)
	}()

	ctx, cancel := signals.SetupSignalHandler()
	defer cancel()

	mgr, err := manager.New(config.GetConfigOrDie(), manager.Options{})
	if err != nil {
		app.Fatalf("failed to create manager: %v", err)
	}

	opts := webhook.ServerOptions{
		CertDir: "/tmp/theatre-istio",
		BootstrapOptions: &webhook.BootstrapOptions{
			MutatingWebhookConfigName: *webhookName,
			Service: &webhook.Service{
				Namespace: *namespace,
				Name:      *serviceName,
				Selectors: map[string]string{
					"app":   "theatre",
					"group": "istio.crd.gocardless.com",
				},
			},
		},
	}

	svr, err := webhook.NewServer("istio", mgr, opts)
	if err != nil {
		app.Fatalf("failed to create admission server: %v", err)
	}

	injectorOpts := istio.InjectorOptions{
		Image:          *theatreImage,
		InstallPath:    *installPath,
		NamespaceLabel: *namespaceLabel,
	}

	var wh *admission.Webhook
	if wh, err = istio.NewWebhook(logger, mgr, injectorOpts); err != nil {
		app.Fatalf(err.Error())
	}

	if err := svr.Register(wh); err != nil {
		app.Fatalf(err.Error())
	}

	if err := mgr.Start(ctx.Done()); err != nil {
		app.Fatalf("failed to run manager: %v", err)
	}
}
