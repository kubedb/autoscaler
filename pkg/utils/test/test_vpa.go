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

package test

import (
	"time"

	vpa_types "kubedb.dev/apimachinery/apis/autoscaling/v1alpha1"

	core "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	kmapi "kmodules.xyz/client-go/api/v1"
)

// VerticalAutoscalerBuilder helps building test instances of VerticalAutoscaler.
type VerticalAutoscalerBuilder interface {
	WithName(vpaName string) VerticalAutoscalerBuilder
	WithContainer(containerName string) VerticalAutoscalerBuilder
	WithNamespace(namespace string) VerticalAutoscalerBuilder
	WithUpdateMode(updateMode vpa_types.UpdateMode) VerticalAutoscalerBuilder
	WithCreationTimestamp(timestamp time.Time) VerticalAutoscalerBuilder
	WithMinAllowed(cpu, memory string) VerticalAutoscalerBuilder
	WithMaxAllowed(cpu, memory string) VerticalAutoscalerBuilder
	WithControlledValues(mode vpa_types.ContainerControlledValues) VerticalAutoscalerBuilder
	WithTarget(cpu, memory string) VerticalAutoscalerBuilder
	WithLowerBound(cpu, memory string) VerticalAutoscalerBuilder
	WithTargetRef(targetRef *core.TypedLocalObjectReference) VerticalAutoscalerBuilder
	WithUpperBound(cpu, memory string) VerticalAutoscalerBuilder
	WithAnnotations(map[string]string) VerticalAutoscalerBuilder
	AppendCondition(conditionType string,
		status core.ConditionStatus, reason, message string, lastTransitionTime time.Time) VerticalAutoscalerBuilder
	AppendRecommendation(vpa_types.RecommendedContainerResources) VerticalAutoscalerBuilder
	Get() *vpa_types.VerticalAutoscaler
}

// VerticalAutoscaler returns a new VerticalAutoscalerBuilder.
func VerticalAutoscaler() VerticalAutoscalerBuilder {
	return &verticalPodAutoscalerBuilder{
		recommendation:          Recommendation(),
		appendedRecommendations: []vpa_types.RecommendedContainerResources{},
		namespace:               "default",
		conditions:              []kmapi.Condition{},
	}
}

type verticalPodAutoscalerBuilder struct {
	vpaName                 string
	containerName           string
	namespace               string
	updatePolicy            *vpa_types.PodUpdatePolicy
	creationTimestamp       time.Time
	minAllowed              core.ResourceList
	maxAllowed              core.ResourceList
	ControlledValues        *vpa_types.ContainerControlledValues
	recommendation          RecommendationBuilder
	conditions              []kmapi.Condition
	annotations             map[string]string
	targetRef               *core.TypedLocalObjectReference
	appendedRecommendations []vpa_types.RecommendedContainerResources
}

func (b *verticalPodAutoscalerBuilder) WithName(vpaName string) VerticalAutoscalerBuilder {
	c := *b
	c.vpaName = vpaName
	return &c
}

func (b *verticalPodAutoscalerBuilder) WithContainer(containerName string) VerticalAutoscalerBuilder {
	c := *b
	c.containerName = containerName
	return &c
}

func (b *verticalPodAutoscalerBuilder) WithNamespace(namespace string) VerticalAutoscalerBuilder {
	c := *b
	c.namespace = namespace
	return &c
}

func (b *verticalPodAutoscalerBuilder) WithUpdateMode(updateMode vpa_types.UpdateMode) VerticalAutoscalerBuilder {
	c := *b
	if c.updatePolicy == nil {
		c.updatePolicy = &vpa_types.PodUpdatePolicy{}
	}
	c.updatePolicy.UpdateMode = &updateMode
	return &c
}

func (b *verticalPodAutoscalerBuilder) WithCreationTimestamp(timestamp time.Time) VerticalAutoscalerBuilder {
	c := *b
	c.creationTimestamp = timestamp
	return &c
}

func (b *verticalPodAutoscalerBuilder) WithMinAllowed(cpu, memory string) VerticalAutoscalerBuilder {
	c := *b
	c.minAllowed = Resources(cpu, memory)
	return &c
}

func (b *verticalPodAutoscalerBuilder) WithMaxAllowed(cpu, memory string) VerticalAutoscalerBuilder {
	c := *b
	c.maxAllowed = Resources(cpu, memory)
	return &c
}

func (b *verticalPodAutoscalerBuilder) WithControlledValues(mode vpa_types.ContainerControlledValues) VerticalAutoscalerBuilder {
	c := *b
	c.ControlledValues = &mode
	return &c
}

func (b *verticalPodAutoscalerBuilder) WithTarget(cpu, memory string) VerticalAutoscalerBuilder {
	c := *b
	c.recommendation = c.recommendation.WithTarget(cpu, memory)
	return &c
}

func (b *verticalPodAutoscalerBuilder) WithLowerBound(cpu, memory string) VerticalAutoscalerBuilder {
	c := *b
	c.recommendation = c.recommendation.WithLowerBound(cpu, memory)
	return &c
}

func (b *verticalPodAutoscalerBuilder) WithUpperBound(cpu, memory string) VerticalAutoscalerBuilder {
	c := *b
	c.recommendation = c.recommendation.WithUpperBound(cpu, memory)
	return &c
}

func (b *verticalPodAutoscalerBuilder) WithTargetRef(targetRef *core.TypedLocalObjectReference) VerticalAutoscalerBuilder {
	c := *b
	c.targetRef = targetRef
	return &c
}

func (b *verticalPodAutoscalerBuilder) WithAnnotations(annotations map[string]string) VerticalAutoscalerBuilder {
	c := *b
	c.annotations = annotations
	return &c
}

func (b *verticalPodAutoscalerBuilder) AppendCondition(conditionType string,
	status core.ConditionStatus, reason, message string, lastTransitionTime time.Time) VerticalAutoscalerBuilder {
	c := *b
	c.conditions = append(c.conditions, kmapi.Condition{
		Type:               conditionType,
		Status:             status,
		Reason:             reason,
		Message:            message,
		LastTransitionTime: metav1.NewTime(lastTransitionTime)})
	return &c
}

func (b *verticalPodAutoscalerBuilder) AppendRecommendation(recommendation vpa_types.RecommendedContainerResources) VerticalAutoscalerBuilder {
	c := *b
	c.appendedRecommendations = append(c.appendedRecommendations, recommendation)
	return &c
}

func (b *verticalPodAutoscalerBuilder) Get() *vpa_types.VerticalAutoscaler {
	if b.containerName == "" {
		panic("Must call WithContainer() before Get()")
	}
	resourcePolicy := vpa_types.PodResourcePolicy{ContainerPolicies: []vpa_types.ContainerResourcePolicy{{
		ContainerName:    b.containerName,
		MinAllowed:       b.minAllowed,
		MaxAllowed:       b.maxAllowed,
		ControlledValues: b.ControlledValues,
	}}}

	recommendation := b.recommendation.WithContainer(b.containerName).Get()
	for _, rec := range b.appendedRecommendations {
		recommendation.ContainerRecommendations = append(recommendation.ContainerRecommendations, rec)
	}

	return &vpa_types.VerticalAutoscaler{
		ObjectMeta: metav1.ObjectMeta{
			Name:              b.vpaName,
			Namespace:         b.namespace,
			Annotations:       b.annotations,
			CreationTimestamp: metav1.NewTime(b.creationTimestamp),
		},
		Spec: vpa_types.VerticalAutoscalerSpec{
			UpdatePolicy:   b.updatePolicy,
			ResourcePolicy: &resourcePolicy,
			TargetRef:      b.targetRef,
		},
		Status: vpa_types.VerticalAutoscalerStatus{
			Recommendation: recommendation,
			Conditions:     b.conditions,
		},
	}
}
