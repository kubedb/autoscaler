/*
Copyright 2019 The Kubernetes Authors.

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

package input

import (
	"fmt"
	"testing"

	vpa_types "kubedb.dev/apimachinery/apis/autoscaling/v1alpha1"
	controllerfetcher "kubedb.dev/autoscaler/pkg/recommender/input/controller_fetcher"
	"kubedb.dev/autoscaler/pkg/recommender/input/spec"
	"kubedb.dev/autoscaler/pkg/recommender/model"
	target_mock "kubedb.dev/autoscaler/pkg/target/mock"
	"kubedb.dev/autoscaler/pkg/utils/test"

	"github.com/golang/mock/gomock"
	"github.com/stretchr/testify/assert"
	core "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
)

type fakeControllerFetcher struct {
	key *controllerfetcher.ControllerKeyWithAPIGroup
	err error
}

func (f *fakeControllerFetcher) FindTopMostWellKnownOrScalable(controller *controllerfetcher.ControllerKeyWithAPIGroup) (*controllerfetcher.ControllerKeyWithAPIGroup, error) {
	return f.key, f.err
}

func parseLabelSelector(selector string) labels.Selector {
	labelSelector, _ := metav1.ParseToLabelSelector(selector)
	parsedSelector, _ := metav1.LabelSelectorAsSelector(labelSelector)
	return parsedSelector
}

var (
	unsupportedConditionNoLongerSupported = "Label selector is no longer supported, please migrate to targetRef"
	unsupportedConditionTextFromFetcher   = "Cannot read targetRef. Reason: targetRef not defined"
	unsupportedConditionNoExtraText       = "Cannot read targetRef"
	unsupportedConditionBothDefined       = "Both targetRef and label selector defined. Please remove label selector"
	unsupportedConditionNoTargetRef       = "Cannot read targetRef"
	unsupportedConditionMudaMudaMuda      = "Error checking if target is a topmost well-known or scalable controller: muda muda muda"
	unsupportedTargetRefHasParent         = "The targetRef controller has a parent but it should point to a topmost well-known or scalable controller"
)

const (
	kind      = "dodokind"
	name1     = "dotaro"
	name2     = "doseph"
	namespace = "testNamespace"
)

var (
	apiGroup  = "stardust"
	apiGroup2 = "taxonomy"
)

func TestLoadPods(t *testing.T) {

	type testCase struct {
		name                                string
		selector                            labels.Selector
		fetchSelectorError                  error
		targetRef                           *core.TypedLocalObjectReference
		topMostWellKnownOrScalableKey       *controllerfetcher.ControllerKeyWithAPIGroup
		findTopMostWellKnownOrScalableError error
		expectedSelector                    labels.Selector
		expectedConfigUnsupported           *string
		expectedConfigDeprecated            *string
	}

	testCases := []testCase{
		{
			name:                      "no selector",
			selector:                  nil,
			fetchSelectorError:        fmt.Errorf("targetRef not defined"),
			expectedSelector:          labels.Nothing(),
			expectedConfigUnsupported: &unsupportedConditionTextFromFetcher,
			expectedConfigDeprecated:  nil,
		},
		{
			name:                      "also no selector but no error",
			selector:                  nil,
			fetchSelectorError:        nil,
			expectedSelector:          labels.Nothing(),
			expectedConfigUnsupported: &unsupportedConditionNoExtraText,
			expectedConfigDeprecated:  nil,
		},
		{
			name:               "targetRef selector",
			selector:           parseLabelSelector("app = test"),
			fetchSelectorError: nil,
			targetRef: &core.TypedLocalObjectReference{
				Kind:     kind,
				Name:     name1,
				APIGroup: &apiGroup,
			},
			topMostWellKnownOrScalableKey: &controllerfetcher.ControllerKeyWithAPIGroup{
				ControllerKey: controllerfetcher.ControllerKey{
					Kind:      kind,
					Name:      name1,
					Namespace: namespace,
				},
				ApiGroup: apiGroup,
			},
			expectedSelector:          parseLabelSelector("app = test"),
			expectedConfigUnsupported: nil,
			expectedConfigDeprecated:  nil,
		},
		{
			name:                      "no targetRef",
			selector:                  parseLabelSelector("app = test"),
			fetchSelectorError:        nil,
			expectedSelector:          labels.Nothing(),
			expectedConfigUnsupported: nil,
			expectedConfigDeprecated:  nil,
		},
		{
			name:               "can't decide if top-level-ref",
			selector:           nil,
			fetchSelectorError: nil,
			expectedSelector:   labels.Nothing(),
			targetRef: &core.TypedLocalObjectReference{
				Kind:     kind,
				Name:     name1,
				APIGroup: &apiGroup,
			},
			expectedConfigUnsupported: &unsupportedConditionNoTargetRef,
		},
		{
			name:               "non-top-level targetRef",
			selector:           parseLabelSelector("app = test"),
			fetchSelectorError: nil,
			expectedSelector:   labels.Nothing(),
			targetRef: &core.TypedLocalObjectReference{
				Kind:     kind,
				Name:     name1,
				APIGroup: &apiGroup,
			},
			topMostWellKnownOrScalableKey: &controllerfetcher.ControllerKeyWithAPIGroup{
				ControllerKey: controllerfetcher.ControllerKey{
					Kind:      kind,
					Name:      name2,
					Namespace: namespace,
				},
				ApiGroup: apiGroup,
			},
			expectedConfigUnsupported: &unsupportedTargetRefHasParent,
		},
		{
			name:               "error checking if top-level-ref",
			selector:           parseLabelSelector("app = test"),
			fetchSelectorError: nil,
			expectedSelector:   labels.Nothing(),
			targetRef: &core.TypedLocalObjectReference{
				Kind:     "doestar",
				Name:     "doseph-doestar",
				APIGroup: &apiGroup2,
			},
			expectedConfigUnsupported:           &unsupportedConditionMudaMudaMuda,
			findTopMostWellKnownOrScalableError: fmt.Errorf("muda muda muda"),
		},
		{
			name:               "top-level target ref",
			selector:           parseLabelSelector("app = test"),
			fetchSelectorError: nil,
			expectedSelector:   parseLabelSelector("app = test"),
			targetRef: &core.TypedLocalObjectReference{
				Kind:     kind,
				Name:     name1,
				APIGroup: &apiGroup,
			},
			topMostWellKnownOrScalableKey: &controllerfetcher.ControllerKeyWithAPIGroup{
				ControllerKey: controllerfetcher.ControllerKey{
					Kind:      kind,
					Name:      name1,
					Namespace: namespace,
				},
				ApiGroup: apiGroup,
			},
			expectedConfigUnsupported: nil,
		},
	}

	for _, tc := range testCases {

		t.Run(tc.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			vpa := test.VerticalAutoscaler().WithName("testVpa").WithContainer("container").WithNamespace("testNamespace").WithTargetRef(tc.targetRef).Get()
			vpaLister := &test.VerticalAutoscalerListerMock{}
			vpaLister.On("List").Return([]*vpa_types.VerticalAutoscaler{vpa}, nil)

			targetSelectorFetcher := target_mock.NewMockVpaTargetSelectorFetcher(ctrl)

			clusterState := model.NewClusterState()

			clusterStateFeeder := clusterStateFeeder{
				vpaLister:       vpaLister,
				clusterState:    clusterState,
				selectorFetcher: targetSelectorFetcher,
				controllerFetcher: &fakeControllerFetcher{
					key: tc.topMostWellKnownOrScalableKey,
					err: tc.findTopMostWellKnownOrScalableError,
				},
			}

			targetSelectorFetcher.EXPECT().Fetch(vpa).Return(tc.selector, tc.fetchSelectorError)
			clusterStateFeeder.LoadVPAs()

			vpaID := model.VpaID{
				Namespace: vpa.Namespace,
				VpaName:   vpa.Name,
			}

			assert.Contains(t, clusterState.Vpas, vpaID)
			storedVpa := clusterState.Vpas[vpaID]
			if tc.expectedSelector != nil {
				assert.NotNil(t, storedVpa.PodSelector)
				assert.Equal(t, tc.expectedSelector.String(), storedVpa.PodSelector.String())
			} else {
				assert.Nil(t, storedVpa.PodSelector)
			}

			if tc.expectedConfigDeprecated != nil {
				assert.Contains(t, storedVpa.Conditions, vpa_types.ConfigDeprecated)
				assert.Equal(t, *tc.expectedConfigDeprecated, storedVpa.Conditions[vpa_types.ConfigDeprecated].Message)
			} else {
				assert.NotContains(t, storedVpa.Conditions, vpa_types.ConfigDeprecated)
			}

			if tc.expectedConfigUnsupported != nil {
				assert.Contains(t, storedVpa.Conditions, vpa_types.ConfigUnsupported)
				assert.Equal(t, *tc.expectedConfigUnsupported, storedVpa.Conditions[vpa_types.ConfigUnsupported].Message)
			} else {
				assert.NotContains(t, storedVpa.Conditions, vpa_types.ConfigUnsupported)
			}

		})
	}
}

type testSpecClient struct {
	pods []*spec.BasicPodSpec
}

func (c *testSpecClient) GetPodSpecs() ([]*spec.BasicPodSpec, error) {
	return c.pods, nil
}

func makeTestSpecClient(podLabels []map[string]string) spec.SpecClient {
	pods := make([]*spec.BasicPodSpec, len(podLabels))
	for i, l := range podLabels {
		pods[i] = &spec.BasicPodSpec{
			ID:        model.PodID{Namespace: "default", PodName: fmt.Sprintf("pod-%d", i)},
			PodLabels: l,
		}
	}
	return &testSpecClient{
		pods: pods,
	}
}

func TestClusterStateFeeder_LoadPods(t *testing.T) {
	for _, tc := range []struct {
		Name              string
		VPALabelSelectors []string
		PodLabels         []map[string]string
		TrackedPods       int
	}{
		{
			Name:              "simple",
			VPALabelSelectors: []string{"name=vpa-pod"},
			PodLabels: []map[string]string{
				{"name": "vpa-pod"},
				{"type": "stateful"},
			},
			TrackedPods: 1,
		},
		{
			Name:              "multiple",
			VPALabelSelectors: []string{"name=vpa-pod,type=stateful"},
			PodLabels: []map[string]string{
				{"name": "vpa-pod", "type": "stateful"},
				{"type": "stateful"},
				{"name": "vpa-pod"},
			},
			TrackedPods: 1,
		},
		{
			Name:              "no matches",
			VPALabelSelectors: []string{"name=vpa-pod"},
			PodLabels: []map[string]string{
				{"name": "non-vpa-pod", "type": "stateful"},
			},
			TrackedPods: 0,
		},
		{
			Name:              "set based",
			VPALabelSelectors: []string{"environment in (staging, qa),name=vpa-pod"},
			PodLabels: []map[string]string{
				{"name": "vpa-pod", "environment": "staging"},
				{"name": "vpa-pod", "environment": "production"},
				{"name": "non-vpa-pod", "environment": "staging"},
				{"name": "non-vpa-pod", "environment": "production"},
			},
			TrackedPods: 1,
		},
	} {
		t.Run(tc.Name, func(t *testing.T) {
			clusterState := model.NewClusterState()
			for i, selector := range tc.VPALabelSelectors {
				vpaLabel, err := labels.Parse(selector)
				assert.NoError(t, err)
				clusterState.Vpas = map[model.VpaID]*model.Vpa{
					{VpaName: fmt.Sprintf("test-vpa-%d", i), Namespace: "default"}: {PodSelector: vpaLabel},
				}
			}

			feeder := clusterStateFeeder{
				specClient:     makeTestSpecClient(tc.PodLabels),
				memorySaveMode: true,
				clusterState:   clusterState,
			}

			feeder.LoadPods()
			assert.Len(t, feeder.clusterState.Pods, tc.TrackedPods, "number of pods is not %d", tc.TrackedPods)

			feeder = clusterStateFeeder{
				specClient:     makeTestSpecClient(tc.PodLabels),
				memorySaveMode: false,
				clusterState:   clusterState,
			}

			feeder.LoadPods()
			assert.Len(t, feeder.clusterState.Pods, len(tc.PodLabels), "number of pods is not %d", len(tc.PodLabels))
		})
	}
}
