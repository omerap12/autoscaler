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
	vpaslices_types "k8s.io/autoscaler/vertical-pod-autoscaler/pkg/apis/autoscaling.k8s.io/v1alpha1"
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

// UseAggregationIfMatching checks if the given aggregation belongs to this slice
// (same parent VPA namespace and matching node label value) and links it.
func (s *VpaSlice) UseAggregationIfMatching(aggregationKey AggregateStateKey, aggregation *AggregateContainerState) {
	if _, exists := s.aggregateContainerStates[aggregationKey]; exists {
		return
	}
	if s.matchesAggregation(aggregationKey) {
		s.aggregateContainerStates[aggregationKey] = aggregation
	}
}

// matchesAggregation returns true if the aggregation key's nodeLabelValue
// matches one of this slice's NodeSelector values. The slice's NodeSelector
// typically has a single key-value pair (e.g. {"node.kubernetes.io/instance-type": "m5.xlarge"}),
// and the aggregation key carries just the value (e.g. "m5.xlarge").
func (s *VpaSlice) matchesAggregation(aggregationKey AggregateStateKey) bool {
	if s.ID.Namespace != aggregationKey.Namespace() {
		return false
	}
	nodeLabelValue := aggregationKey.NodeLabelValue()
	if nodeLabelValue == "" {
		return false
	}
	for _, v := range s.NodeSelector {
		if v == nodeLabelValue {
			return true
		}
	}
	return false
}

// UsesAggregation returns true iff an aggregation with the given key contributes to this slice.
func (s *VpaSlice) UsesAggregation(aggregationKey AggregateStateKey) bool {
	_, exists := s.aggregateContainerStates[aggregationKey]
	return exists
}

// DeleteAggregation removes the aggregation with the given key from this slice.
func (s *VpaSlice) DeleteAggregation(aggregationKey AggregateStateKey) {
	delete(s.aggregateContainerStates, aggregationKey)
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

// AggregateStateByContainerName returns a map from container name to the aggregated state
// of all containers with that name, belonging to pods matched by this VPA slice.
func (s *VpaSlice) AggregateStateByContainerName() ContainerNameToAggregateStateMap {
	containerNameToAggregateStateMap := AggregateStateByContainerName(s.aggregateContainerStates)
	s.MergeCheckpointedState(containerNameToAggregateStateMap)
	return containerNameToAggregateStateMap
}

// MergeCheckpointedState adds checkpointed VPASlice aggregations to the given aggregateStateMap.
func (s *VpaSlice) MergeCheckpointedState(aggregateContainerStateMap ContainerNameToAggregateStateMap) {
	for containerName, aggregation := range s.ContainersInitialAggregateState {
		aggregateContainerState, found := aggregateContainerStateMap[containerName]
		if !found {
			aggregateContainerState = NewAggregateContainerState()
			aggregateContainerStateMap[containerName] = aggregateContainerState
		}
		aggregateContainerState.MergeContainerState(aggregation)
	}
}

// HasRecommendation returns if the VPASlice contains any recommendation.
func (s *VpaSlice) HasRecommendation() bool {
	return s.recommendation != nil && len(s.recommendation.ContainerRecommendations) > 0
}

// SetRecommendation sets the recommendation for this VPASlice.
func (s *VpaSlice) SetRecommendation(recommendation *vpa_types.RecommendedPodResources) {
	s.recommendation = recommendation
}

// UpdateConditions updates conditions based on the VPASlice state.
func (s *VpaSlice) UpdateConditions(hasMatchingAggregations bool) {
	reason := ""
	msg := ""
	if !hasMatchingAggregations {
		reason = "NoMatchingAggregations"
		msg = "No aggregations match this VPA slice"
	}
	if s.HasRecommendation() {
		s.conditions.Set(vpa_types.RecommendationProvided, true, "", "")
	} else {
		s.conditions.Set(vpa_types.RecommendationProvided, false, reason, msg)
	}
}

// AsSliceStatus returns this object's equivalent of VPASlice Status.
func (s *VpaSlice) AsSliceStatus() *vpaslices_types.VerticalPodAutoscalerSliceStatus {
	status := &vpaslices_types.VerticalPodAutoscalerSliceStatus{
		Conditions: s.conditions.AsList(),
	}
	if s.recommendation != nil {
		status.Recommendation = s.recommendation
	}
	return status
}
