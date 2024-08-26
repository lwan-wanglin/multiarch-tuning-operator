package operator_test

import (
	"os"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	runtimeclient "sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/openshift/multiarch-tuning-operator/apis/multiarch/v1alpha1"
	"github.com/openshift/multiarch-tuning-operator/apis/multiarch/v1beta1"

	"github.com/openshift/multiarch-tuning-operator/pkg/e2e"
	. "github.com/openshift/multiarch-tuning-operator/pkg/testing/builder"
	"github.com/openshift/multiarch-tuning-operator/pkg/testing/framework"
	"github.com/openshift/multiarch-tuning-operator/pkg/utils"
)

const (
	helloOpenshiftPublicMultiarchImage = "quay.io/openshifttest/hello-openshift:1.2.0"
)

var _ = Describe("The Multiarch Tuning Operator", func() {
	var (
		podLabel            = map[string]string{"app": "test"}
		schedulingGateLabel = map[string]string{utils.SchedulingGateLabel: utils.SchedulingGateLabelValueRemoved}
	)
	AfterEach(func() {
		if CurrentSpecReport().Failed() {
			By("The test case failed, get the PPC and PPWH logs for debug")
			// ignore err
			_ = framework.GetPodsLogToFile(ctx, clientset, client, utils.Namespace(), "controller", utils.PodPlacementControllerName, os.Getenv("ARTIFACT_DIR"))
			_ = framework.GetPodsLogToFile(ctx, clientset, client, utils.Namespace(), "controller", utils.PodPlacementWebhookName, os.Getenv("ARTIFACT_DIR"))
		}
		err := client.Delete(ctx, &v1beta1.ClusterPodPlacementConfig{
			ObjectMeta: metav1.ObjectMeta{
				Name: "cluster",
			},
		})
		Expect(err).NotTo(HaveOccurred())
		Eventually(framework.ValidateDeletion(client, ctx)).Should(Succeed())
	})
	Context("When the operator is running and a pod placement config is created", func() {
		It("should deploy the operands with v1beta1 API", func() {
			err := client.Create(ctx, &v1beta1.ClusterPodPlacementConfig{
				ObjectMeta: metav1.ObjectMeta{
					Name: "cluster",
				},
			})
			Expect(err).NotTo(HaveOccurred())
			Eventually(framework.ValidateCreation(client, ctx)).Should(Succeed())
			By("convert the v1beta1 CR to v1alpha1 should succeed")
			c := &v1alpha1.ClusterPodPlacementConfig{}
			err = client.Get(ctx, runtimeclient.ObjectKey{Name: "cluster"}, c)
			Expect(err).NotTo(HaveOccurred())
		})
		It("should deploy the operands with v1alpha1 API", func() {
			err := client.Create(ctx, &v1alpha1.ClusterPodPlacementConfig{
				ObjectMeta: metav1.ObjectMeta{
					Name: "cluster",
				},
			})
			Expect(err).NotTo(HaveOccurred())
			Eventually(framework.ValidateCreation(client, ctx)).Should(Succeed())
			By("convert the v1alpha1 CR to v1beta1 should succeed")
			c := &v1beta1.ClusterPodPlacementConfig{}
			err = client.Get(ctx, runtimeclient.ObjectKey{Name: "cluster"}, c)
			Expect(err).NotTo(HaveOccurred())
		})
	})
	Context("The webhook should get requests only for pods matching the namespaceSelector in the ClusterPodPlacementConfig CR", func() {
		BeforeEach(func() {
			err := client.Create(ctx, &v1beta1.ClusterPodPlacementConfig{
				ObjectMeta: metav1.ObjectMeta{
					Name: "cluster",
				},
				Spec: v1beta1.ClusterPodPlacementConfigSpec{
					NamespaceSelector: &metav1.LabelSelector{
						MatchExpressions: []metav1.LabelSelectorRequirement{
							{
								Key:      "multiarch.openshift.io/exclude-pod-placement",
								Operator: "DoesNotExist",
							},
						}}}})
			Expect(err).NotTo(HaveOccurred())
			Eventually(framework.ValidateCreation(client, ctx)).Should(Succeed())
		})
		It("should exclude namespaces that have the opt-out label", func() {
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
			err = client.Create(ctx, d)
			Expect(err).NotTo(HaveOccurred())
			//should exclude the namespace
			verifyPodLabels(ns, "app", "test", e2e.Absent, schedulingGateLabel)
			verifyPodNodeAffinity(ns, "app", "test")
		})
		It("should handle namespaces that do not have the opt-out label", func() {
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
			err = client.Create(ctx, d)
			Expect(err).NotTo(HaveOccurred())
			archLabelNSR := NewNodeSelectorRequirement().
				WithKeyAndValues(utils.ArchLabel, corev1.NodeSelectorOpIn, utils.ArchitectureAmd64,
					utils.ArchitectureArm64, utils.ArchitectureS390x, utils.ArchitecturePpc64le).
				Build()
			expectedNSTs := NewNodeSelectorTerm().WithMatchExpressions(&archLabelNSR).Build()
			//should handle the namespace
			verifyPodLabels(ns, "app", "test", e2e.Present, schedulingGateLabel)
			verifyPodNodeAffinity(ns, "app", "test", expectedNSTs)
		})
	})
	Context("The operator should respect to an opt-in namespaceSelector in ClusterPodPlacementConfig CR", func() {
		BeforeEach(func() {
			err := client.Create(ctx, &v1beta1.ClusterPodPlacementConfig{
				ObjectMeta: metav1.ObjectMeta{
					Name: "cluster",
				},
				Spec: v1beta1.ClusterPodPlacementConfigSpec{
					NamespaceSelector: &metav1.LabelSelector{
						MatchExpressions: []metav1.LabelSelectorRequirement{
							{
								Key:      "multiarch.openshift.io/include-pod-placement",
								Operator: "Exists",
							},
						}}}})
			Expect(err).NotTo(HaveOccurred())
			Eventually(framework.ValidateCreation(client, ctx)).Should(Succeed())
		})
		It("should exclude namespaces that do not match the opt-in configuration", func() {
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
			err = client.Create(ctx, d)
			Expect(err).NotTo(HaveOccurred())
			//should exclude the namespace
			verifyPodLabels(ns, "app", "test", e2e.Absent, schedulingGateLabel)
			verifyPodNodeAffinity(ns, "app", "test")
		})
		It("should handle namespaces that match the opt-in configuration", func() {
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
			err = client.Create(ctx, d)
			Expect(err).NotTo(HaveOccurred())
			archLabelNSR := NewNodeSelectorRequirement().
				WithKeyAndValues(utils.ArchLabel, corev1.NodeSelectorOpIn, utils.ArchitectureAmd64,
					utils.ArchitectureArm64, utils.ArchitectureS390x, utils.ArchitecturePpc64le).
				Build()
			expectedNSTs := NewNodeSelectorTerm().WithMatchExpressions(&archLabelNSR).Build()
			//should handle the namespace
			verifyPodLabels(ns, "app", "test", e2e.Present, schedulingGateLabel)
			verifyPodNodeAffinity(ns, "app", "test", expectedNSTs)
		})
	})
	Context("The webhook should not gate pods with node selectors that pin them to the control plane", func() {
		BeforeEach(func() {
			err := client.Create(ctx, &v1beta1.ClusterPodPlacementConfig{
				ObjectMeta: metav1.ObjectMeta{
					Name: "cluster",
				},
				Spec: v1beta1.ClusterPodPlacementConfigSpec{
					NamespaceSelector: &metav1.LabelSelector{
						MatchExpressions: []metav1.LabelSelectorRequirement{
							{
								Key:      "multiarch.openshift.io/exclude-pod-placement",
								Operator: "DoesNotExist",
							},
						}}}})
			Expect(err).NotTo(HaveOccurred())
			Eventually(framework.ValidateCreation(client, ctx)).Should(Succeed())
		})
		DescribeTable("should not gate pods to schedule in control plane nodes", func(selector string) {
			var err error
			ns := framework.NewEphemeralNamespace()
			err = client.Create(ctx, ns)
			Expect(err).NotTo(HaveOccurred())
			//nolint:errcheck
			defer client.Delete(ctx, ns)
			var nodeSelectors = map[string]string{selector: ""}
			ps := NewPodSpec().
				WithContainersImages(helloOpenshiftPublicMultiarchImage).
				WithNodeSelectors(nodeSelectors).
				Build()
			d := NewDeployment().
				WithSelectorAndPodLabels(podLabel).
				WithPodSpec(ps).
				WithReplicas(utils.NewPtr(int32(1))).
				WithName("test-deployment").
				WithNamespace(ns.Name).
				Build()
			err = client.Create(ctx, d)
			Expect(err).NotTo(HaveOccurred())
			//should exclude the namespace
			verifyPodLabels(ns, "app", "test", e2e.Absent, schedulingGateLabel)
			verifyPodNodeAffinity(ns, "app", "test")
		},
			Entry(utils.ControlPlaneNodeSelectorLabel, utils.ControlPlaneNodeSelectorLabel),
			Entry(utils.MasterNodeSelectorLabel, utils.MasterNodeSelectorLabel),
		)
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

func verifyPodLabels(ns *corev1.Namespace, labelKey string, labelInValue string, ifPresent bool, entries map[string]string) {
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
		for k, v := range entries {
			if ifPresent {
				g.Expect(pods.Items).Should(HaveEach(WithTransform(func(p corev1.Pod) map[string]string {
					return p.Labels
				}, And(Not(BeEmpty()), HaveKeyWithValue(k, v)))))
			} else {
				g.Expect(pods.Items).Should(HaveEach(WithTransform(func(p corev1.Pod) map[string]string {
					return p.Labels
				}, Not(HaveKey(k)))))
			}
		}
	}, e2e.WaitShort).Should(Succeed())
}
