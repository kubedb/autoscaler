/*
Copyright 2017 The Kubernetes Authors.

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

package updater

import (
	"context"
	"os"
	"time"

	vpa_clientset "kubedb.dev/apimachinery/client/clientset/versioned"
	"kubedb.dev/autoscaler/common"
	"kubedb.dev/autoscaler/pkg/target"
	updater "kubedb.dev/autoscaler/pkg/updater/logic"
	"kubedb.dev/autoscaler/pkg/updater/priority"
	"kubedb.dev/autoscaler/pkg/utils/limitrange"
	"kubedb.dev/autoscaler/pkg/utils/metrics"
	metrics_updater "kubedb.dev/autoscaler/pkg/utils/metrics/updater"
	"kubedb.dev/autoscaler/pkg/utils/status"
	vpa_api_util "kubedb.dev/autoscaler/pkg/utils/vpa"

	"github.com/spf13/cobra"
	core "k8s.io/api/core/v1"
	"k8s.io/client-go/informers"
	kube_client "k8s.io/client-go/kubernetes"
	kube_restclient "k8s.io/client-go/rest"
	kube_flag "k8s.io/component-base/cli/flag"
	"k8s.io/klog"
)

const defaultResyncPeriod time.Duration = 10 * time.Minute

func NewCmdUpdater() *cobra.Command {
	var (
		updaterInterval              time.Duration
		minReplicas                  int
		evictionToleranceFraction    float64
		evictionRateLimit            float64
		evictionRateBurst            int
		address                      string
		useAdmissionControllerStatus bool
		namespace                    = os.Getenv("NAMESPACE")
		vpaObjectNamespace           string
	)
	cmd := &cobra.Command{
		Use:               "updater",
		Short:             "Updater decides which pods should be restarted based on resources allocation recommendation calculated by Recommender.",
		DisableAutoGenTag: true,
		Run: func(cmd *cobra.Command, args []string) {
			klog.InitFlags(nil)
			kube_flag.InitFlags()
			klog.V(1).Infof("Vertical Pod Autoscaler %s Updater", common.VerticalAutoscalerVersion)

			healthCheck := metrics.NewHealthCheck(updaterInterval*5, true)
			metrics.Initialize(address, healthCheck)
			metrics_updater.Register()

			config, err := kube_restclient.InClusterConfig()
			if err != nil {
				klog.Fatalf("Failed to build Kubernetes client : fail to create config: %v", err)
			}
			kubeClient := kube_client.NewForConfigOrDie(config)
			vpaClient := vpa_clientset.NewForConfigOrDie(config)
			factory := informers.NewSharedInformerFactory(kubeClient, defaultResyncPeriod)
			targetSelectorFetcher := target.NewVpaTargetSelectorFetcher(config, kubeClient, factory)
			var limitRangeCalculator limitrange.LimitRangeCalculator
			limitRangeCalculator, err = limitrange.NewLimitsRangeCalculator(factory)
			if err != nil {
				klog.Errorf("Failed to create limitRangeCalculator, falling back to not checking limits. Error message: %s", err)
				limitRangeCalculator = limitrange.NewNoopLimitsCalculator()
			}
			admissionControllerStatusNamespace := status.AdmissionControllerStatusNamespace
			if namespace != "" {
				admissionControllerStatusNamespace = namespace
			}
			// TODO: use SharedInformerFactory in updater
			updater, err := updater.NewUpdater(
				kubeClient,
				vpaClient,
				minReplicas,
				evictionRateLimit,
				evictionRateBurst,
				evictionToleranceFraction,
				useAdmissionControllerStatus,
				admissionControllerStatusNamespace,
				vpa_api_util.NewCappingRecommendationProcessor(limitRangeCalculator),
				nil,
				targetSelectorFetcher,
				priority.NewProcessor(),
				vpaObjectNamespace,
			)
			if err != nil {
				klog.Fatalf("Failed to create updater: %v", err)
			}
			ticker := time.Tick(updaterInterval)
			for range ticker {
				ctx, cancel := context.WithTimeout(context.Background(), updaterInterval)
				defer cancel()
				updater.RunOnce(ctx)
				healthCheck.UpdateLastActivity()
			}
		},
	}
	cmd.Flags().DurationVar(&updaterInterval, "updater-interval", 1*time.Minute, `How often updater should run`)
	cmd.Flags().IntVar(&minReplicas, "min-replicas", 2, `Minimum number of replicas to perform update`)
	cmd.Flags().Float64Var(&evictionToleranceFraction, "eviction-tolerance", 0.5, `Fraction of replica count that can be evicted for update, if more than one pod can be evicted.`)
	cmd.Flags().Float64Var(&evictionRateLimit, "eviction-rate-limit", -1, `Number of pods that can be evicted per seconds. A rate limit set to 0 or -1 will disable the rate limiter.`)
	cmd.Flags().IntVar(&evictionRateBurst, "eviction-rate-burst", 1, `Burst of pods that can be evicted.`)
	cmd.Flags().StringVar(&address, "address", ":8943", "The address to expose Prometheus metrics.")
	cmd.Flags().BoolVar(&useAdmissionControllerStatus, "use-admission-controller-status", true, "If true, updater will only evict pods when admission controller status is valid.")
	cmd.Flags().StringVar(&vpaObjectNamespace, "vpa-object-namespace", core.NamespaceAll, "Namespace to search for VPA objects. Empty means all namespaces will be used.")
	return cmd
}
