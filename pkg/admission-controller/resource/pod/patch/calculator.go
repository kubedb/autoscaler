/*
Copyright 2020 The Kubernetes Authors.

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

package patch

import (
	vpa_types "kubedb.dev/apimachinery/apis/autoscaling/v1alpha1"
	"kubedb.dev/autoscaler/pkg/admission-controller/resource"

	core "k8s.io/api/core/v1"
)

// Calculator is capable of calculating required patches for pod.
type Calculator interface {
	CalculatePatches(pod *core.Pod, vpa *vpa_types.VerticalAutoscaler) ([]resource.PatchRecord, error)
}
