/*
Copyright 2018 The Kubernetes Authors.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package admissioncontroller

import (
	"fmt"
	"net/http"
	"os"
	"time"

	vpa_clientset "kubedb.dev/apimachinery/client/clientset/versioned"
	"kubedb.dev/autoscaler/common"
	"kubedb.dev/autoscaler/pkg/admission-controller/logic"
	"kubedb.dev/autoscaler/pkg/admission-controller/resource/pod"
	"kubedb.dev/autoscaler/pkg/admission-controller/resource/pod/patch"
	"kubedb.dev/autoscaler/pkg/admission-controller/resource/pod/recommendation"
	"kubedb.dev/autoscaler/pkg/admission-controller/resource/vpa"
	"kubedb.dev/autoscaler/pkg/target"
	"kubedb.dev/autoscaler/pkg/utils/limitrange"
	"kubedb.dev/autoscaler/pkg/utils/metrics"
	metrics_admission "kubedb.dev/autoscaler/pkg/utils/metrics/admission"
	"kubedb.dev/autoscaler/pkg/utils/status"
	vpa_api_util "kubedb.dev/autoscaler/pkg/utils/vpa"

	"github.com/spf13/cobra"
	core "k8s.io/api/core/v1"
	"k8s.io/client-go/informers"
	kube_client "k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	kube_flag "k8s.io/component-base/cli/flag"
	"k8s.io/klog"
)

const (
	defaultResyncPeriod  = 10 * time.Minute
	statusUpdateInterval = 10 * time.Second
)

func NewCmdAdmissionController() *cobra.Command {
	var (
		certsConfiguration certsConfig
		port               int
		address            string
		namespace          = os.Getenv("NAMESPACE")
		serviceName        string
		webhookAddress     string
		webhookPort        string
		webhookTimeout     int
		registerWebhook    bool
		registerByURL      bool
		vpaObjectNamespace string
	)
	cmd := &cobra.Command{
		Use:   "admission-controller",
		Short: "Mutating admission webhook controller for pods",
		Long: `admission-controller registers itself as a Mutating Admission Webhook and for each pod creation, it will get a request from the apiserver and it will either decide there's no matching VPA configuration or find the corresponding
one and use current recommendation to set resource requests in the pod.`,
		DisableAutoGenTag: true,
		Run: func(cmd *cobra.Command, args []string) {
			klog.InitFlags(nil)
			kube_flag.InitFlags()
			klog.V(1).Infof("Vertical Pod Autoscaler %s Admission Controller", common.VerticalAutoscalerVersion)

			healthCheck := metrics.NewHealthCheck(time.Minute, false)
			metrics.Initialize(address, healthCheck)
			metrics_admission.Register()

			certs := initCerts(certsConfiguration)

			config, err := rest.InClusterConfig()
			if err != nil {
				klog.Fatal(err)
			}

			vpaClient := vpa_clientset.NewForConfigOrDie(config)
			vpaLister := vpa_api_util.NewVpasLister(vpaClient, make(chan struct{}), vpaObjectNamespace)
			kubeClient := kube_client.NewForConfigOrDie(config)
			factory := informers.NewSharedInformerFactory(kubeClient, defaultResyncPeriod)
			targetSelectorFetcher := target.NewVpaTargetSelectorFetcher(config, kubeClient, factory)
			podPreprocessor := pod.NewDefaultPreProcessor()
			vpaPreprocessor := vpa.NewDefaultPreProcessor()
			var limitRangeCalculator limitrange.LimitRangeCalculator
			limitRangeCalculator, err = limitrange.NewLimitsRangeCalculator(factory)
			if err != nil {
				klog.Errorf("Failed to create limitRangeCalculator, falling back to not checking limits. Error message: %s", err)
				limitRangeCalculator = limitrange.NewNoopLimitsCalculator()
			}
			recommendationProvider := recommendation.NewProvider(limitRangeCalculator, vpa_api_util.NewCappingRecommendationProcessor(limitRangeCalculator))
			vpaMatcher := vpa.NewMatcher(vpaLister, targetSelectorFetcher)

			hostname, err := os.Hostname()
			if err != nil {
				klog.Fatalf("Unable to get hostname: %v", err)
			}

			statusNamespace := status.AdmissionControllerStatusNamespace
			if namespace != "" {
				statusNamespace = namespace
			}
			stopCh := make(chan struct{})
			statusUpdater := status.NewUpdater(
				kubeClient,
				status.AdmissionControllerStatusName,
				statusNamespace,
				statusUpdateInterval,
				hostname,
			)
			defer close(stopCh)

			calculators := []patch.Calculator{patch.NewResourceUpdatesCalculator(recommendationProvider), patch.NewObservedContainersCalculator()}
			as := logic.NewAdmissionServer(podPreprocessor, vpaPreprocessor, limitRangeCalculator, vpaMatcher, calculators)
			http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
				as.Serve(w, r)
				healthCheck.UpdateLastActivity()
			})
			clientset := getClient()
			server := &http.Server{
				Addr:      fmt.Sprintf(":%d", port),
				TLSConfig: configTLS(certs.serverCert, certs.serverKey),
			}
			url := fmt.Sprintf("%v:%v", webhookAddress, webhookPort)
			go func() {
				if registerWebhook {
					selfRegistration(clientset, certs.caCert, namespace, serviceName, url, registerByURL, int32(webhookTimeout))
				}
				// Start status updates after the webhook is initialized.
				statusUpdater.Run(stopCh)
			}()
			err = server.ListenAndServeTLS("", "")
			if err != nil {
				klog.Fatal(err)
			}
		},
	}
	cmd.Flags().StringVar(&certsConfiguration.clientCaFile, "client-ca-file", "/etc/tls-certs/caCert.pem", "Path to CA PEM file.")
	cmd.Flags().StringVar(&certsConfiguration.tlsCertFile, "tls-cert-file", "/etc/tls-certs/serverCert.pem", "Path to server certificate PEM file.")
	cmd.Flags().StringVar(&certsConfiguration.tlsPrivateKey, "tls-private-key", "/etc/tls-certs/serverKey.pem", "Path to server certificate key PEM file.")

	cmd.Flags().IntVar(&port, "port", 8000, "The port to listen on.")
	cmd.Flags().StringVar(&address, "address", ":8944", "The address to expose Prometheus metrics.")
	cmd.Flags().StringVar(&serviceName, "webhook-service", "vpa-webhook", "Kubernetes service under which webhook is registered. Used when registerByURL is set to false.")
	cmd.Flags().StringVar(&webhookAddress, "webhook-address", "", "Address under which webhook is registered. Used when registerByURL is set to true.")
	cmd.Flags().StringVar(&webhookPort, "webhook-port", "", "Server Port for Webhook")
	cmd.Flags().IntVar(&webhookTimeout, "webhook-timeout-seconds", 30, "Timeout in seconds that the API server should wait for this webhook to respond before failing.")
	cmd.Flags().BoolVar(&registerWebhook, "register-webhook", true, "If set to true, admission webhook object will be created on start up to register with the API server.")
	cmd.Flags().BoolVar(&registerByURL, "register-by-url", false, "If set to true, admission webhook will be registered by URL (webhookAddress:webhookPort) instead of by service name")
	cmd.Flags().StringVar(&vpaObjectNamespace, "vpa-object-namespace", core.NamespaceAll, "Namespace to search for VPA objects. Empty means all namespaces will be used.")
	return cmd
}
