package main

import (
	"flag"
	"os"
	"os/signal"

	"github.com/linkerd/linkerd2/controller/k8s"
	injector "github.com/linkerd/linkerd2/controller/proxy-injector"
	"github.com/linkerd/linkerd2/controller/proxy-injector/tmpl"
	"github.com/linkerd/linkerd2/pkg/admin"
	"github.com/linkerd/linkerd2/pkg/flags"
	k8sPkg "github.com/linkerd/linkerd2/pkg/k8s"
	"github.com/linkerd/linkerd2/pkg/tls"
	"github.com/linkerd/linkerd2/pkg/webhook"
	log "github.com/sirupsen/logrus"
)

func main() {
	metricsAddr := flag.String("metrics-addr", ":9995", "address to serve scrapable metrics on")
	addr := flag.String("addr", ":8443", "address to serve on")
	kubeconfig := flag.String("kubeconfig", "", "path to kubeconfig")
	controllerNamespace := flag.String("controller-namespace", "linkerd", "namespace in which Linkerd is installed")
	webhookServiceName := flag.String("webhook-service", "linkerd-proxy-injector.linkerd.io", "name of the admission webhook")
	flags.ConfigureAndParse()

	stop := make(chan os.Signal, 1)
	defer close(stop)
	signal.Notify(stop, os.Interrupt, os.Kill)

	k8sClient, err := k8s.NewClientSet(*kubeconfig)
	if err != nil {
		log.Fatalf("failed to initialize Kubernetes client: %s", err)
	}

	rootCA, err := tls.GenerateRootCAWithDefaults("Proxy Injector Mutating Webhook Admission Controller CA")
	if err != nil {
		log.Fatalf("failed to create root CA: %s", err)
	}

	webhookConfig := &webhook.Config{
		ControllerNamespace: *controllerNamespace,
		WebhookConfigName:   k8sPkg.ProxyInjectorWebhookConfig,
		WebhookServiceName:  *webhookServiceName,
		RootCA:              rootCA,
		TemplateStr:         tmpl.MutatingWebhookConfigurationSpec,
		Ops:                 injector.NewOps(k8sClient),
	}
	selfLink, err := webhookConfig.Create()
	if err != nil {
		log.Fatalf("failed to create the mutating webhook configurations resource: %s", err)
	}
	log.Infof("created mutating webhook configuration: %s", selfLink)

	s, err := webhook.NewServer(k8sClient, *addr, "linkerd-proxy-injector", *controllerNamespace, rootCA, injector.Inject)
	if err != nil {
		log.Fatalf("failed to initialize the webhook server: %s", err)
	}

	go s.Start()
	go admin.StartServer(*metricsAddr)

	<-stop
	log.Info("shutting down webhook server")
	if err := s.Shutdown(); err != nil {
		log.Error(err)
	}
}
