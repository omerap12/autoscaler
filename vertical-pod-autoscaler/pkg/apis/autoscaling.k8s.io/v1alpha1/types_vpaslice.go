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

package v1alpha1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	vpa "k8s.io/autoscaler/vertical-pod-autoscaler/pkg/apis/autoscaling.k8s.io/v1"
)

// ---- Labels used to associate VPASlice objects with their parent VPA ----

const (
	// VPASliceOwnerLabel is set on each VPASlice to reference the parent VPA by name.
	// Analogous to kubernetes.io/service-name on EndpointSlice.
	VPASliceOwnerLabel = "autoscaling.k8s.io/vpa-name"

	// VPASliceNodeLabelKey is set on each VPASlice to record which node label
	// key was used for slicing (e.g. "node.kubernetes.io/instance-type").
	VPASliceNodeLabelKey = "autoscaling.k8s.io/slice-label-key"
)

// +genclient
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
// +kubebuilder:storageversion
// +kubebuilder:resource:shortName=vpaslice
// +kubebuilder:subresource:status
// +kubebuilder:printcolumn:name="VPA",type="string",JSONPath=".spec.vpaName"
// +kubebuilder:printcolumn:name="NodeSelector",type="string",JSONPath=".spec.nodeSelector"
// +kubebuilder:printcolumn:name="CPU",type="string",JSONPath=".status.recommendation.containerRecommendations[0].target.cpu"
// +kubebuilder:printcolumn:name="Mem",type="string",JSONPath=".status.recommendation.containerRecommendations[0].target.memory"
// +kubebuilder:printcolumn:name="Provided",type="string",JSONPath=".status.conditions[?(@.type=='RecommendationProvided')].status"
// +kubebuilder:printcolumn:name="Age",type="date",JSONPath=".metadata.creationTimestamp"
// +kubebuilder:metadata:annotations="api-approved.kubernetes.io=https://github.com/kubernetes/kubernetes/pull/63797"

// VerticalPodAutoscalerSlice holds resource recommendations for a subset of
// pods managed by a VerticalPodAutoscaler, scoped to pods running on nodes
// that match a specific label selector.
//
// VPASlice objects are created and managed by the VPA recommender when the
// parent VPA has spec.sliceByNodeLabel set. Each slice contains independent
// recommendations computed from the resource usage of pods on matching nodes.
//
// This is analogous to how EndpointSlice breaks a Service's endpoints into
// smaller, per-subset objects — except here the slicing dimension is node
// characteristics rather than size.
type VerticalPodAutoscalerSlice struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	// Specification identifying which pods this slice covers.
	Spec VerticalPodAutoscalerSliceSpec `json:"spec"`

	// Computed recommendations for pods on matching nodes.
	// +optional
	Status VerticalPodAutoscalerSliceStatus `json:"status,omitempty"`
}

// VerticalPodAutoscalerSliceSpec identifies the parent VPA and the node
// subset this slice covers.
type VerticalPodAutoscalerSliceSpec struct {
	// VPAName is the name of the parent VerticalPodAutoscaler object.
	// The VPASlice must be in the same namespace as the parent VPA.
	// The recommender sets an ownerReference on the VPASlice pointing to
	// the parent VPA for garbage collection.
	VPAName string `json:"vpaName"`

	// NodeSelector identifies the set of nodes this slice covers.
	// Pods running on nodes whose labels match ALL entries in this map
	// will use this slice's recommendations.
	//
	// Typically contains a single key-value pair derived from the parent
	// VPA's spec.sliceByNodeLabel field (e.g. {"node.kubernetes.io/instance-type": "m5.xlarge"}).
	NodeSelector map[string]string `json:"nodeSelector"`
}

// VerticalPodAutoscalerSliceStatus contains the recommendations computed for
// pods on nodes matching this slice's selector.
type VerticalPodAutoscalerSliceStatus struct {
	// Recommendation is the most recently computed resource recommendation
	// for containers of pods running on nodes matching this slice's selector.
	// Reuses the same RecommendedPodResources type as VerticalPodAutoscalerStatus.
	// +optional
	Recommendation *vpa.RecommendedPodResources `json:"recommendation,omitempty"`

	// Conditions is the set of conditions for this slice, indicating whether
	// a recommendation could be computed.
	// +optional
	// +patchMergeKey=type
	// +patchStrategy=merge
	Conditions []vpa.VerticalPodAutoscalerCondition `json:"conditions,omitempty" patchStrategy:"merge" patchMergeKey:"type"`

	// ObservedGeneration is the most recent generation observed by the
	// recommender for this slice.
	// +optional
	// +kubebuilder:validation:Minimum=0
	ObservedGeneration *int64 `json:"observedGeneration,omitempty"`
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// VerticalPodAutoscalerSliceList is a list of VerticalPodAutoscalerSlice objects.
type VerticalPodAutoscalerSliceList struct {
	metav1.TypeMeta `json:",inline"`
	// metadata is the standard list metadata.
	// +optional
	metav1.ListMeta `json:"metadata"`

	// items is the list of VPASlice objects.
	Items []VerticalPodAutoscalerSlice `json:"items"`
}

// +genclient
// +genclient:noStatus
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
// +kubebuilder:storageversion
// +kubebuilder:resource:shortName=vpaslicecheckpoint
// +kubebuilder:metadata:annotations="api-approved.kubernetes.io=https://github.com/kubernetes/kubernetes/pull/63797"

// VerticalPodAutoscalerSliceCheckpoint is the checkpoint of the internal state
// of VPA for a single VPASlice. It stores per-container histogram data scoped
// to the pods on nodes matching the slice's nodeSelector, enabling recovery
// of per-node-group recommender state after a restart.
//
// Unlike VerticalPodAutoscalerCheckpoint (which stores one container per object),
// a slice checkpoint packs all containers into a single object, yielding one
// checkpoint per VPASlice regardless of container count.
type VerticalPodAutoscalerSliceCheckpoint struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	// Specification of the slice checkpoint.
	// +optional
	Spec VerticalPodAutoscalerSliceCheckpointSpec `json:"spec,omitempty"`

	// Data of the slice checkpoint.
	// +optional
	Status VerticalPodAutoscalerSliceCheckpointStatus `json:"status,omitempty"`
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// VerticalPodAutoscalerSliceCheckpointList is a list of VerticalPodAutoscalerSliceCheckpoint objects.
type VerticalPodAutoscalerSliceCheckpointList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata"`
	Items           []VerticalPodAutoscalerSliceCheckpoint `json:"items"`
}

// VerticalPodAutoscalerSliceCheckpointSpec identifies the parent VPA and slice.
type VerticalPodAutoscalerSliceCheckpointSpec struct {
	// VPAObjectName is the name of the parent VerticalPodAutoscaler.
	VPAObjectName string `json:"vpaObjectName,omitempty"`

	// VPASliceName is the name of the VerticalPodAutoscalerSlice this
	// checkpoint belongs to.
	VPASliceName string `json:"vpaSliceName,omitempty"`
}

// VerticalPodAutoscalerSliceCheckpointStatus contains per-container histogram
// data for a single VPASlice.
type VerticalPodAutoscalerSliceCheckpointStatus struct {
	// The time when the status was last refreshed.
	// +nullable
	LastUpdateTime metav1.Time `json:"lastUpdateTime,omitempty"`

	// Version of the format of the stored data.
	Version string `json:"version,omitempty"`

	// ContainerCheckpoints holds histogram data for each container in the slice.
	// +optional
	ContainerCheckpoints []ContainerHistogramCheckpoint `json:"containerCheckpoints,omitempty"`
}

// ContainerHistogramCheckpoint holds histogram data for a single container.
type ContainerHistogramCheckpoint struct {
	// Name of the container.
	ContainerName string `json:"containerName"`

	// Checkpoint of histogram for consumption of CPU.
	CPUHistogram vpa.HistogramCheckpoint `json:"cpuHistogram,omitempty"`

	// Checkpoint of histogram for consumption of memory.
	MemoryHistogram vpa.HistogramCheckpoint `json:"memoryHistogram,omitempty"`

	// Timestamp of the first sample from the histograms.
	// +nullable
	FirstSampleStart metav1.Time `json:"firstSampleStart,omitempty"`

	// Timestamp of the last sample from the histograms.
	// +nullable
	LastSampleStart metav1.Time `json:"lastSampleStart,omitempty"`

	// Total number of samples in the histograms.
	TotalSamplesCount int `json:"totalSamplesCount,omitempty"`
}
