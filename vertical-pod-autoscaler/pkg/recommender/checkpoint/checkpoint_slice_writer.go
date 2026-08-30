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

package checkpoint

import (
	"context"
	"encoding/json"
	"fmt"
	"slices"
	"strings"
	"sync"
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/klog/v2"

	vpaslices_types "k8s.io/autoscaler/vertical-pod-autoscaler/pkg/apis/autoscaling.k8s.io/v1alpha1"
	vpa_slice_api "k8s.io/autoscaler/vertical-pod-autoscaler/pkg/client/clientset/versioned/typed/autoscaling.k8s.io/v1alpha1"
	"k8s.io/autoscaler/vertical-pod-autoscaler/pkg/recommender/model"
)

// CheckpointSliceWriter persistently stores aggregated historical usage of containers
// controlled by VPA slice objects.
type CheckpointSliceWriter interface {
	StoreCheckpointSlices(ctx context.Context, concurrentWorkers int)
}

type checkpointSliceWriter struct {
	vpaSliceCheckpointClient vpa_slice_api.VerticalPodAutoscalerSliceCheckpointsGetter
	cluster                  model.ClusterState
}

// NewCheckpointSliceWriter returns a new instance of CheckpointSliceWriter.
func NewCheckpointSliceWriter(cluster model.ClusterState, vpaSliceCheckpointClient vpa_slice_api.VerticalPodAutoscalerSliceCheckpointsGetter) CheckpointSliceWriter {
	return &checkpointSliceWriter{
		vpaSliceCheckpointClient: vpaSliceCheckpointClient,
		cluster:                  cluster,
	}
}

type slicePatchRecord struct {
	Op    string `json:"op,inline"`
	Path  string `json:"path,inline"`
	Value any    `json:"value"`
}

func getSlicesToCheckpoint(clusterSlices map[model.VpaSliceID]*model.VpaSlice) []*model.VpaSlice {
	result := make([]*model.VpaSlice, 0, len(clusterSlices))
	for _, slice := range clusterSlices {
		result = append(result, slice)
	}
	slices.SortFunc(result, func(a, b *model.VpaSlice) int {
		return a.CheckpointWritten.Compare(b.CheckpointWritten)
	})
	return result
}

func processCheckpointUpdateForSlice(slice *model.VpaSlice, writer *checkpointSliceWriter) {
	now := time.Now()
	aggregateContainerStateMap := buildAggregateContainerStateMapForSlice(slice, writer.cluster, now)

	var containerCheckpoints []vpaslices_types.ContainerHistogramCheckpoint
	for containerName, aggregatedState := range aggregateContainerStateMap {
		checkpointStatus, err := aggregatedState.SaveToCheckpoint()
		if err != nil {
			klog.ErrorS(err, "Cannot serialize checkpoint for slice",
				"vpaSlice", klog.KRef(slice.ID.Namespace, slice.ID.SliceName),
				"container", containerName)
			continue
		}
		containerCheckpoints = append(containerCheckpoints, vpaslices_types.ContainerHistogramCheckpoint{
			ContainerName:     containerName,
			CPUHistogram:      checkpointStatus.CPUHistogram,
			MemoryHistogram:   checkpointStatus.MemoryHistogram,
			FirstSampleStart:  checkpointStatus.FirstSampleStart,
			LastSampleStart:   checkpointStatus.LastSampleStart,
			TotalSamplesCount: checkpointStatus.TotalSamplesCount,
		})
	}

	if len(containerCheckpoints) == 0 {
		return
	}

	sliceCheckpoint := &vpaslices_types.VerticalPodAutoscalerSliceCheckpoint{
		ObjectMeta: metav1.ObjectMeta{
			Name:      slice.ID.SliceName,
			Namespace: slice.ID.Namespace,
		},
		Spec: vpaslices_types.VerticalPodAutoscalerSliceCheckpointSpec{
			VPAObjectName: slice.VPAName,
			VPASliceName:  slice.ID.SliceName,
		},
		Status: vpaslices_types.VerticalPodAutoscalerSliceCheckpointStatus{
			LastUpdateTime:       metav1.NewTime(now),
			Version:              model.SupportedCheckpointVersion,
			ContainerCheckpoints: containerCheckpoints,
		},
	}

	err := createOrUpdateSliceCheckpoint(
		writer.vpaSliceCheckpointClient.VerticalPodAutoscalerSliceCheckpoints(slice.ID.Namespace),
		sliceCheckpoint)
	if err != nil {
		klog.ErrorS(err, "Cannot save checkpoint for slice",
			"vpaSlice", klog.KRef(slice.ID.Namespace, slice.ID.SliceName))
	} else {
		klog.V(3).InfoS("Saved checkpoint for slice",
			"vpaSlice", klog.KRef(slice.ID.Namespace, slice.ID.SliceName))
		slice.CheckpointWritten = now
	}
}

func (writer *checkpointSliceWriter) StoreCheckpointSlices(ctx context.Context, concurrentWorkers int) {
	vpaSlices := getSlicesToCheckpoint(writer.cluster.VPASlices())

	sliceCheckpointUpdates := make(chan *model.VpaSlice, len(vpaSlices))

	var wg sync.WaitGroup
	for range concurrentWorkers {
		wg.Go(func() {
			for slice := range sliceCheckpointUpdates {
				processCheckpointUpdateForSlice(slice, writer)
				select {
				case <-ctx.Done():
					return
				default:
				}
			}
		})
	}

	for _, slice := range vpaSlices {
		sliceCheckpointUpdates <- slice
	}

	close(sliceCheckpointUpdates)
	wg.Wait()

	if ctx.Err() != nil {
		klog.V(0).InfoS("Failed to store all slice checkpoints within the configured timeout", "err", ctx.Err())
	}
}

func buildAggregateContainerStateMapForSlice(slice *model.VpaSlice, cluster model.ClusterState, now time.Time) map[string]*model.AggregateContainerState {
	aggregateContainerStateMap := slice.AggregateStateByContainerName()
	for _, pod := range cluster.Pods() {
		for containerName, container := range pod.Containers {
			aggregateKey := cluster.MakeAggregateStateKey(pod, containerName)
			if slice.UsesAggregation(aggregateKey) {
				if aggregateContainerState, exists := aggregateContainerStateMap[containerName]; exists {
					subtractCurrentContainerMemoryPeak(aggregateContainerState, container, now)
				}
			}
		}
	}
	return aggregateContainerStateMap
}

func createOrUpdateSliceCheckpoint(
	client vpa_slice_api.VerticalPodAutoscalerSliceCheckpointInterface,
	checkpoint *vpaslices_types.VerticalPodAutoscalerSliceCheckpoint,
) error {
	patches := []slicePatchRecord{{
		Op:    "replace",
		Path:  "/status",
		Value: checkpoint.Status,
	}}
	bytes, err := json.Marshal(patches)
	if err != nil {
		return fmt.Errorf("cannot marshal slice checkpoint status patch: %v", err)
	}
	_, err = client.Patch(context.TODO(), checkpoint.Name, types.JSONPatchType, bytes, metav1.PatchOptions{})
	if err != nil && strings.Contains(err.Error(), fmt.Sprintf(`"%s" not found`, checkpoint.Name)) {
		_, err = client.Create(context.TODO(), checkpoint, metav1.CreateOptions{})
	}
	if err != nil {
		return fmt.Errorf("cannot save slice checkpoint %s/%s: %v", checkpoint.Namespace, checkpoint.Name, err)
	}
	return nil
}
