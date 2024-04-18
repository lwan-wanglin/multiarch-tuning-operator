package operator_test

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	runtimeclient "sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/openshift/multiarch-tuning-operator/apis/multiarch/v1alpha1"
	"github.com/openshift/multiarch-tuning-operator/pkg/e2e"
	. "github.com/openshift/multiarch-tuning-operator/pkg/testing/builder"
	"github.com/openshift/multiarch-tuning-operator/pkg/testing/framework"
	"github.com/openshift/multiarch-tuning-operator/pkg/utils"
)

const (
	helloOpenshiftPublicMultiarchImage = "quay.io/openshifttest/hello-openshift:1.2.0"
)

var _ = Describe("The Multiarch Tuning Operator", func() {
	var podLabel = map[string]string{"app": "test"}
	AfterEach(func() {
		err := client.Delete(ctx, &v1alpha1.PodPlacementConfig{
			ObjectMeta: metav1.ObjectMeta{
				Name: "cluster",
			},
		})
		Expect(err).NotTo(HaveOccurred())
		Eventually(deploymentsAreDeleted).Should(Succeed())
	})
	Context("When the operator is running and a pod placement config is created", func() {
		It("should deploy the operands", func() {
			err := client.Create(ctx, &v1alpha1.PodPlacementConfig{
				ObjectMeta: metav1.ObjectMeta{
					Name: "cluster",
				},
			})
			Expect(err).NotTo(HaveOccurred())
			Eventually(deploymentsAreRunning).Should(Succeed())
		})
	})
	Context("The operator should respect to an opt-out namespaceSelector in PodPlacementConfig CR", func() {
		BeforeEach(func() {
			err := client.Create(ctx, &v1alpha1.PodPlacementConfig{
				ObjectMeta: metav1.ObjectMeta{
					Name: "cluster",
				},
				Spec: v1alpha1.PodPlacementConfigSpec{
					NamespaceSelector: &metav1.LabelSelector{
						MatchExpressions: []metav1.LabelSelectorRequirement{
							{
								Key:      "multiarch.openshift.io/exclude-pod-placement",
								Operator: "DoesNotExist",
							},
						}}}})
			Expect(err).NotTo(HaveOccurred())
			Eventually(deploymentsAreRunning).Should(Succeed())
		})
		It("should exclude namespaces which have the opt-out label", func() {
			var err error
			ns := framework.NewEphemeralNamespace()
			ns.Labels = map[string]string{
				"multiarch.openshift.io/exclude-pod-placement": "",
			}
			err = client.Create(ctx, ns)
			Expect(err).NotTo(HaveOccurred())
			//nolint:errcheck
			defer client.Delete(ctx, ns)
			ps := NewPodSpec().
				WithContainersImages(helloOpenshiftPublicMultiarchImage).
				Build()
			d := NewDeployment().
				WithSelectorAndPodLabels(podLabel).
				WithPodSpec(ps).
				WithReplicas(utils.NewPtr(int32(1))).
				WithName("test-deployment").
				WithNamespace(ns.Name).
				Build()
			err = client.Create(ctx, &d)
			Expect(err).NotTo(HaveOccurred())
			//should exclude the namespace
			verifyPodNodeAffinity(ns, "app", "test")
		})
		It("should handle namespaces which does not have the opt-out lable", func() {
			var err error
			ns := framework.NewEphemeralNamespace()
			err = client.Create(ctx, ns)
			Expect(err).NotTo(HaveOccurred())
			//nolint:errcheck
			defer client.Delete(ctx, ns)
			ps := NewPodSpec().
				WithContainersImages(helloOpenshiftPublicMultiarchImage).
				Build()
			d := NewDeployment().
				WithSelectorAndPodLabels(podLabel).
				WithPodSpec(ps).
				WithReplicas(utils.NewPtr(int32(1))).
				WithName("test-deployment").
				WithNamespace(ns.Name).
				Build()
			err = client.Create(ctx, &d)
			Expect(err).NotTo(HaveOccurred())
			archLabelNSR := NewNodeSelectorRequirement().
				WithKeyAndValues(utils.ArchLabel, corev1.NodeSelectorOpIn, utils.ArchitectureAmd64,
					utils.ArchitectureArm64, utils.ArchitectureS390x, utils.ArchitecturePpc64le).
				Build()
			expectedNSTs := NewNodeSelectorTerm().WithMatchExpressions(&archLabelNSR).Build()
			//should handle the namespace
			verifyPodNodeAffinity(ns, "app", "test", expectedNSTs)
		})
	})
	Context("The operator should respect to an opt-in namespaceSelector in PodPlacementConfig CR", func() {
		BeforeEach(func() {
			err := client.Create(ctx, &v1alpha1.PodPlacementConfig{
				ObjectMeta: metav1.ObjectMeta{
					Name: "cluster",
				},
				Spec: v1alpha1.PodPlacementConfigSpec{
					NamespaceSelector: &metav1.LabelSelector{
						MatchExpressions: []metav1.LabelSelectorRequirement{
							{
								Key:      "multiarch.openshift.io/include-pod-placement",
								Operator: "Exists",
							},
						}}}})
			Expect(err).NotTo(HaveOccurred())
			Eventually(deploymentsAreRunning).Should(Succeed())
		})
		It("should exclude namespaces which does not have the opt-in label", func() {
			var err error
			ns := framework.NewEphemeralNamespace()
			err = client.Create(ctx, ns)
			Expect(err).NotTo(HaveOccurred())
			//nolint:errcheck
			defer client.Delete(ctx, ns)
			ps := NewPodSpec().
				WithContainersImages(helloOpenshiftPublicMultiarchImage).
				Build()
			d := NewDeployment().
				WithSelectorAndPodLabels(podLabel).
				WithPodSpec(ps).
				WithReplicas(utils.NewPtr(int32(1))).
				WithName("test-deployment").
				WithNamespace(ns.Name).
				Build()
			err = client.Create(ctx, &d)
			Expect(err).NotTo(HaveOccurred())
			//should exclude the namespace
			verifyPodNodeAffinity(ns, "app", "test")
		})
		It("should handle namespaces which match the opt-in lable", func() {
			var err error
			ns := framework.NewEphemeralNamespace()
			ns.Labels = map[string]string{
				"multiarch.openshift.io/include-pod-placement": "",
			}
			err = client.Create(ctx, ns)
			Expect(err).NotTo(HaveOccurred())
			//nolint:errcheck
			defer client.Delete(ctx, ns)
			ps := NewPodSpec().
				WithContainersImages(helloOpenshiftPublicMultiarchImage).
				Build()
			d := NewDeployment().
				WithSelectorAndPodLabels(podLabel).
				WithPodSpec(ps).
				WithReplicas(utils.NewPtr(int32(1))).
				WithName("test-deployment").
				WithNamespace(ns.Name).
				Build()
			err = client.Create(ctx, &d)
			Expect(err).NotTo(HaveOccurred())
			archLabelNSR := NewNodeSelectorRequirement().
				WithKeyAndValues(utils.ArchLabel, corev1.NodeSelectorOpIn, utils.ArchitectureAmd64,
					utils.ArchitectureArm64, utils.ArchitectureS390x, utils.ArchitecturePpc64le).
				Build()
			expectedNSTs := NewNodeSelectorTerm().WithMatchExpressions(&archLabelNSR).Build()
			//should handle the namespace
			verifyPodNodeAffinity(ns, "app", "test", expectedNSTs)
		})
	})
})

func verifyPodNodeAffinity(ns *corev1.Namespace, labelKey string, labelInValue string, nodeSelectorTerms ...corev1.NodeSelectorTerm) {
	r, err := labels.NewRequirement(labelKey, "in", []string{labelInValue})
	labelSelector := labels.NewSelector().Add(*r)
	Expect(err).NotTo(HaveOccurred())
	Eventually(func(g Gomega) {
		pods := &corev1.PodList{}
		err := client.List(ctx, pods, &runtimeclient.ListOptions{
			Namespace:     ns.Name,
			LabelSelector: labelSelector,
		})
		g.Expect(err).NotTo(HaveOccurred())
		g.Expect(pods.Items).NotTo(BeEmpty())
		if len(nodeSelectorTerms) == 0 {
			g.Expect(pods.Items).To(HaveEach(WithTransform(func(p corev1.Pod) *corev1.Affinity {
				return p.Spec.Affinity
			}, BeNil())))
		} else {
			g.Expect(pods.Items).To(HaveEach(framework.HaveEquivalentNodeAffinity(
				&corev1.NodeAffinity{
					RequiredDuringSchedulingIgnoredDuringExecution: &corev1.NodeSelector{
						NodeSelectorTerms: nodeSelectorTerms,
					},
				})))
		}
	}, e2e.WaitShort).Should(Succeed())
}
