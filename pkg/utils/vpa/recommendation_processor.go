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

package api

import (
	vpa_types "kubedb.dev/apimachinery/apis/autoscaling/v1alpha1"

	core "k8s.io/api/core/v1"
	kmapi "kmodules.xyz/client-go/api/v1"
)

// ContainerToAnnotationsMap contains annotations per container.
type ContainerToAnnotationsMap = map[string][]string

// RecommendationProcessor post-processes recommendation adjusting it to limits and environment context
type RecommendationProcessor interface {
	// Apply processes and updates recommendation for given pod, based on container limits,
	// VPA policy and possibly other internal RecommendationProcessor context.
	// Must return a non-nil pointer to RecommendedPodResources or error.
	Apply(podRecommendation *vpa_types.RecommendedPodResources,
		policy *vpa_types.PodResourcePolicy,
		conditions []kmapi.Condition,
		pod *core.Pod) (*vpa_types.RecommendedPodResources, ContainerToAnnotationsMap, error)
}
