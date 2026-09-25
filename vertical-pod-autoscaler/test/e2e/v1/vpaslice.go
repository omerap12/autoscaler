/*
Copyright The Kubernetes Authors.

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

package autoscaling

import (
	"context"
	"fmt"

	ginkgo "github.com/onsi/ginkgo/v2"
	"github.com/onsi/gomega"
	apiv1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/util/wait"
	vpa_types "k8s.io/autoscaler/vertical-pod-autoscaler/pkg/apis/autoscaling.k8s.io/v1"
	"k8s.io/autoscaler/vertical-pod-autoscaler/pkg/features"
	"k8s.io/autoscaler/vertical-pod-autoscaler/pkg/utils/test"
	"k8s.io/autoscaler/vertical-pod-autoscaler/test/e2e/utils"
	"k8s.io/kubernetes/test/e2e/framework"
	podsecurity "k8s.io/pod-security-admission/api"
)

const (
	sliceByNodeLabel = "kubernetes.io/hostname"
)

var _ = VPASliceE2eDescribe("VPASlice", func() {
	f := framework.NewDefaultFramework("vertical-pod-autoscaling")
	f.NamespacePodSecurityLevel = podsecurity.LevelBaseline
	var expectedWorkerNodes int

	ginkgo.BeforeEach(func() {
		nodes, err := f.ClientSet.CoreV1().Nodes().List(context.Background(), metav1.ListOptions{
			LabelSelector: sliceByNodeLabel,
		})
		gomega.Expect(err).NotTo(gomega.HaveOccurred())
		gomega.Expect(nodes).NotTo(gomega.BeNil())
		// -1 for the control plane
		expectedWorkerNodes = len(nodes.Items) - 1
	})

	f.It("creates VPASlice objects with per-node recommendations for a DaemonSet",
		framework.WithFeatureGate(features.VPASlices), func() {
			ginkgo.By("Setting up a hamster DaemonSet")
			SetupHamsterDaemonSet(f, "100m", "100Mi")

			ginkgo.By("Setting up a VPA with sliceByNodeLabel targeting the DaemonSet")
			containerName := utils.GetHamsterContainerNameByIndex(0)
			vpaCRD := test.VerticalPodAutoscaler().
				WithName("hamster-vpa").
				WithNamespace(f.Namespace.Name).
				WithTargetRef(HamsterDaemonSetTargetRef).
				WithContainer(containerName).
				WithUpdateMode(vpa_types.UpdateModeOff).
				WithSliceByNodeLabel(sliceByNodeLabel).
				Get()
			utils.InstallVPA(f, vpaCRD)

			ginkgo.By(fmt.Sprintf("Waiting for %d VPASlice objects to be created", expectedWorkerNodes))
			vpaClientSet := utils.GetVpaClientSet(f)
			slices, err := utils.WaitForVPASlicesPresent(vpaClientSet, vpaCRD, expectedWorkerNodes)
			gomega.Expect(err).NotTo(gomega.HaveOccurred(), "VPASlice objects were not created")
			gomega.Expect(slices).To(gomega.HaveLen(expectedWorkerNodes))

			ginkgo.By("Verifying each VPASlice has a nodeSelector with the correct label key")
			for _, slice := range slices {
				gomega.Expect(slice.Spec.VPAName).To(gomega.Equal(vpaCRD.Name))
				gomega.Expect(slice.Spec.NodeSelector).To(gomega.HaveKey(sliceByNodeLabel))
			}

			ginkgo.By("Waiting for VPASlice recommendations to be computed")
			slices, err = utils.WaitForVPASlicesWithRecommendations(vpaClientSet, vpaCRD, expectedWorkerNodes)
			gomega.Expect(err).NotTo(gomega.HaveOccurred(), "VPASlice recommendations were not computed")
			for _, slice := range slices {
				framework.Logf("VPASlice %s has recommendation: %v", slice.Name, slice.Status.Recommendation)
				gomega.Expect(slice.Status.Recommendation.ContainerRecommendations).NotTo(gomega.BeEmpty())
				gomega.Expect(slice.Status.Recommendation.ContainerRecommendations[0].ContainerName).To(gomega.Equal(containerName))
				gomega.Expect(slice.Status.Recommendation.ContainerRecommendations[0].Target).NotTo(gomega.BeEmpty())
			}

			ginkgo.By("Verifying the parent VPA recommendation is empty")
			updatedVPA, err := vpaClientSet.AutoscalingV1().VerticalPodAutoscalers(f.Namespace.Name).Get(
				context.TODO(), vpaCRD.Name, metav1.GetOptions{})
			gomega.Expect(err).NotTo(gomega.HaveOccurred())
			gomega.Expect(updatedVPA.Status.Recommendation == nil ||
				len(updatedVPA.Status.Recommendation.ContainerRecommendations) == 0).To(gomega.BeTrue(),
				"parent VPA should have empty recommendation when sliceByNodeLabel is set")
		})

	f.It("garbage collects VPASlice objects when parent VPA is deleted",
		framework.WithFeatureGate(features.VPASlices), func() {
			ginkgo.By("Setting up a hamster DaemonSet")
			SetupHamsterDaemonSet(f, "100m", "100Mi")

			ginkgo.By("Setting up a VPA with sliceByNodeLabel")
			containerName := utils.GetHamsterContainerNameByIndex(0)
			vpaCRD := test.VerticalPodAutoscaler().
				WithName("hamster-vpa").
				WithNamespace(f.Namespace.Name).
				WithTargetRef(HamsterDaemonSetTargetRef).
				WithContainer(containerName).
				WithUpdateMode(vpa_types.UpdateModeOff).
				WithSliceByNodeLabel(sliceByNodeLabel).
				Get()
			utils.InstallVPA(f, vpaCRD)

			ginkgo.By("Waiting for VPASlice objects to be created")
			vpaClientSet := utils.GetVpaClientSet(f)
			_, err := utils.WaitForVPASlicesPresent(vpaClientSet, vpaCRD, expectedWorkerNodes)
			gomega.Expect(err).NotTo(gomega.HaveOccurred())

			ginkgo.By("Deleting the parent VPA")
			err = vpaClientSet.AutoscalingV1().VerticalPodAutoscalers(f.Namespace.Name).Delete(
				context.TODO(), vpaCRD.Name, metav1.DeleteOptions{})
			gomega.Expect(err).NotTo(gomega.HaveOccurred())

			ginkgo.By("Waiting for VPASlice objects to be garbage collected")
			err = utils.WaitForVPASlicesGone(vpaClientSet, f.Namespace.Name, vpaCRD.Name)
			gomega.Expect(err).NotTo(gomega.HaveOccurred(), "VPASlice objects should be garbage collected when parent VPA is deleted")
		})

	f.It("creates VPASliceCheckpoint objects for VPASlice objects",
		framework.WithFeatureGate(features.VPASlices), func() {
			ginkgo.By("Setting up a hamster DaemonSet")
			SetupHamsterDaemonSet(f, "100m", "100Mi")

			ginkgo.By("Setting up a VPA with sliceByNodeLabel")
			containerName := utils.GetHamsterContainerNameByIndex(0)
			vpaCRD := test.VerticalPodAutoscaler().
				WithName("hamster-vpa").
				WithNamespace(f.Namespace.Name).
				WithTargetRef(HamsterDaemonSetTargetRef).
				WithContainer(containerName).
				WithUpdateMode(vpa_types.UpdateModeOff).
				WithSliceByNodeLabel(sliceByNodeLabel).
				Get()
			utils.InstallVPA(f, vpaCRD)

			ginkgo.By("Waiting for VPASlice recommendations")
			vpaClientSet := utils.GetVpaClientSet(f)
			slices, err := utils.WaitForVPASlicesWithRecommendations(vpaClientSet, vpaCRD, expectedWorkerNodes)
			gomega.Expect(err).NotTo(gomega.HaveOccurred())

			ginkgo.By("Waiting for VPASliceCheckpoint objects to be created")
			err = wait.PollUntilContextTimeout(context.Background(), utils.PollInterval, utils.PollTimeout, true, func(ctx context.Context) (bool, error) {
				checkpointList, err := vpaClientSet.AutoscalingV1alpha1().VerticalPodAutoscalerSliceCheckpoints(f.Namespace.Name).List(ctx, metav1.ListOptions{})
				if err != nil {
					return false, err
				}
				matchedCount := 0
				for _, cp := range checkpointList.Items {
					for _, slice := range slices {
						if cp.Spec.VPASliceName == slice.Name {
							matchedCount++
							break
						}
					}
				}
				return matchedCount >= expectedWorkerNodes, nil
			})
			gomega.Expect(err).NotTo(gomega.HaveOccurred(), "VPASliceCheckpoint objects should be created for each VPASlice")
		})

	f.It("updater evicts DaemonSet pods with outdated resources when mode is Recreate",
		framework.WithFeatureGate(features.VPASlices), func() {
			ginkgo.By("Setting up a hamster DaemonSet with low resources")
			SetupHamsterDaemonSet(f, "10m", "10Mi")

			ginkgo.By("Setting up a VPA with sliceByNodeLabel and Recreate update mode")
			containerName := utils.GetHamsterContainerNameByIndex(0)
			vpaCRD := test.VerticalPodAutoscaler().
				WithName("hamster-vpa").
				WithNamespace(f.Namespace.Name).
				WithTargetRef(HamsterDaemonSetTargetRef).
				WithContainer(containerName).
				WithUpdateMode(vpa_types.UpdateModeRecreate).
				WithSliceByNodeLabel(sliceByNodeLabel).
				Get()
			utils.InstallVPA(f, vpaCRD)

			ginkgo.By("Recording the initial pod set")
			podList, err := GetHamsterPods(f)
			gomega.Expect(err).NotTo(gomega.HaveOccurred())
			initialPodSet := MakePodSet(podList)

			ginkgo.By("Waiting for VPASlice recommendations")
			vpaClientSet := utils.GetVpaClientSet(f)
			_, err = utils.WaitForVPASlicesWithRecommendations(vpaClientSet, vpaCRD, expectedWorkerNodes)
			gomega.Expect(err).NotTo(gomega.HaveOccurred())

			ginkgo.By("Waiting for at least one pod to be evicted by the updater")
			err = waitForDaemonSetPodsEvicted(f, initialPodSet)
			gomega.Expect(err).NotTo(gomega.HaveOccurred(), "at least one DaemonSet pod should be evicted by the updater")

		})

	f.It("does not apply VPASlice recommendation when update mode is Off",
		framework.WithFeatureGate(features.VPASlices), func() {
			ginkgo.By("Setting up a hamster DaemonSet")
			SetupHamsterDaemonSet(f, "100m", "100Mi")

			ginkgo.By("Setting up a VPA with sliceByNodeLabel and Off update mode")
			containerName := utils.GetHamsterContainerNameByIndex(0)
			vpaCRD := test.VerticalPodAutoscaler().
				WithName("hamster-vpa").
				WithNamespace(f.Namespace.Name).
				WithTargetRef(HamsterDaemonSetTargetRef).
				WithContainer(containerName).
				WithUpdateMode(vpa_types.UpdateModeOff).
				WithSliceByNodeLabel(sliceByNodeLabel).
				Get()
			utils.InstallVPA(f, vpaCRD)

			ginkgo.By("Waiting for VPASlice recommendations")
			vpaClientSet := utils.GetVpaClientSet(f)
			_, err := utils.WaitForVPASlicesWithRecommendations(vpaClientSet, vpaCRD, expectedWorkerNodes)
			gomega.Expect(err).NotTo(gomega.HaveOccurred())

			ginkgo.By("Recording the initial pod set")
			podList, err := GetHamsterPods(f)
			gomega.Expect(err).NotTo(gomega.HaveOccurred())
			initialPodSet := MakePodSet(podList)

			ginkgo.By(fmt.Sprintf("Waiting for %s to verify no pods are evicted", VpaEvictionTimeout.String()))
			CheckNoPodsEvicted(f, initialPodSet)

			ginkgo.By("Verifying pods still have original resource requests")
			currentPodList, err := GetHamsterPods(f)
			gomega.Expect(err).NotTo(gomega.HaveOccurred())
			for _, pod := range currentPodList.Items {
				gomega.Expect(pod.Spec.Containers[0].Resources.Requests[apiv1.ResourceCPU]).To(
					gomega.Equal(ParseQuantityOrDie("100m")), "CPU request should not change with updateMode Off")
				gomega.Expect(pod.Spec.Containers[0].Resources.Requests[apiv1.ResourceMemory]).To(
					gomega.Equal(ParseQuantityOrDie("100Mi")), "memory request should not change with updateMode Off")
			}
		})
})

func waitForDaemonSetPodsEvicted(f *framework.Framework, initialPodSet PodSet) error {
	return wait.PollUntilContextTimeout(context.Background(), utils.PollInterval, utils.PollTimeout, true, func(ctx context.Context) (bool, error) {
		currentPodList, err := f.ClientSet.CoreV1().Pods(f.Namespace.Name).List(ctx, metav1.ListOptions{
			LabelSelector: labels.SelectorFromSet(labels.Set(utils.HamsterLabels)).String(),
		})
		if err != nil {
			return false, err
		}
		currentPodSet := MakePodSet(currentPodList)
		evicted := GetEvictedPodsCount(currentPodSet, initialPodSet)
		framework.Logf("%d of %d initial DaemonSet pods have been evicted", evicted, len(initialPodSet))
		return evicted > 0, nil
	})
}
