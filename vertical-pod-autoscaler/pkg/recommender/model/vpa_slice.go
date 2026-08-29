/*
Copyright 2026 The Kubernetes Authors.

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

package model

import (
	"time"

	vpa_types "k8s.io/autoscaler/vertical-pod-autoscaler/pkg/apis/autoscaling.k8s.io/v1"
)

// VpaSlice holds the model state for a single VerticalPodAutoscalerSlice object.
type VpaSlice struct {
	ID VpaSliceID
	// VPAName is the name of the parent VPA object.
	VPAName string
	// NodeSelector identifies which nodes this slice covers.
	NodeSelector map[string]string
	// conditions is the map of status conditions.
	conditions vpaConditionsMap
	// recommendation is the most recently computed recommendation for this slice.
	recommendation *vpa_types.RecommendedPodResources
	// All container aggregations that contribute to this VPASlice.
	aggregateContainerStates aggregateContainerStatesMap
	// Initial checkpoints of AggregateContainerStates for containers.
	ContainersInitialAggregateState ContainerNameToAggregateStateMap
	// CheckpointWritten indicates when last checkpoint for this slice was stored.
	CheckpointWritten time.Time
	// Created denotes timestamp of the original VPASlice object creation.
	Created time.Time
}

// NewVpaSlice returns a new VpaSlice with a given ID.
func NewVpaSlice(id VpaSliceID, vpaName string, nodeSelector map[string]string, created time.Time) *VpaSlice {
	return &VpaSlice{
		ID:                              id,
		VPAName:                         vpaName,
		NodeSelector:                    nodeSelector,
		conditions:                      make(vpaConditionsMap),
		aggregateContainerStates:        make(aggregateContainerStatesMap),
		ContainersInitialAggregateState: make(ContainerNameToAggregateStateMap),
		Created:                         created,
	}
}
