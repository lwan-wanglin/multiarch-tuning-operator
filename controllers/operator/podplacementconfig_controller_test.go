/*
Copyright 2023 Red Hat, Inc.

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

package operator

import (
	"fmt"

	"k8s.io/apimachinery/pkg/api/errors"
	crclient "sigs.k8s.io/controller-runtime/pkg/client"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/openshift/multiarch-tuning-operator/apis/multiarch/common"
	"github.com/openshift/multiarch-tuning-operator/apis/multiarch/v1beta1"
	"github.com/openshift/multiarch-tuning-operator/pkg/testing/builder"
	"github.com/openshift/multiarch-tuning-operator/pkg/utils"
)

var _ = Describe("Controllers/PodPlacementConfig/PodPlacementConfigReconciler", Serial, Ordered, func() {
	When("Creating a namespaced pod placement config", func() {
		It("should deny creation if clusterpodplacementconfig doesn't exist", func() {
			cppc := &v1beta1.ClusterPodPlacementConfig{}
			err := k8sClient.Get(ctx, crclient.ObjectKey{
				Name: common.SingletonResourceObjectName,
			}, cppc)
			Expect(errors.IsNotFound(err)).To(BeTrue(), "The ClusterPodPlacementConfig should not exist")
			By("Create a PodPlacementConfig")
			ppc := builder.NewPodPlacementConfig().
				WithName("ppc1").
				WithNamespace("test-namespace").
				WithPlugins().
				WithNodeAffinityScoring(true).
				WithNodeAffinityScoringTerm(utils.ArchitectureAmd64, 0).
				Build()
			err = k8sClient.Create(ctx, ppc)
			By(fmt.Sprintf("The error is: %+v", err))
			By("Verify the PodPlacementConfig is not created")
			Expect(err).To(HaveOccurred(), "The create PodPlacementConfig should not be accepted")
			By("Verify the error is 'invalid'")
			Expect(errors.IsInvalid(err)).To(BeTrue(), "The invalid PodPlacementConfig should not be accepted")
		})
		It("should deny creation if there is pod placement config with the same priority", func() {
			cppc := builder.NewClusterPodPlacementConfig().
				WithName(common.SingletonResourceObjectName).
				WithPlugins().
				WithNodeAffinityScoring(true).
				WithNodeAffinityScoringTerm(utils.ArchitectureAmd64, 0).
				Build()
			err := k8sClient.Create(ctx, cppc)
			Expect(err).NotTo(HaveOccurred(), "The create ClusterPodPlacementConfig should succeed")
			By("Create a PodPlacementConfig")
			ppc := builder.NewPodPlacementConfig().
				WithName("ppc1").
				WithNamespace("test-namespace").
				WithPriority(common.Priority(50)).
				Build()
			err = k8sClient.Create(ctx, ppc)
			Expect(err).NotTo(HaveOccurred(), "The create PodPlacementConfig should succeed")
			By("Create a PodPlacementConfig with the same priority")
			ppc2 := builder.NewPodPlacementConfig().
				WithName("ppc2").
				WithNamespace("test-namespace").
				WithPriority(common.Priority(50)).
				Build()
			err = k8sClient.Create(ctx, ppc2)
			Expect(err).To(HaveOccurred(), "The create PodPlacementConfig should not be accepted")
			By(fmt.Sprintf("The error is: %+v", err))
			By("Verify the PodPlacementConfig is not created")
			Expect(err).To(HaveOccurred(), "The create PodPlacementConfig should not be accepted")
			By("Verify the error is 'invalid'")
			Expect(errors.IsInvalid(err)).To(BeTrue(), "The invalid PodPlacementConfig should not be accepted")
		})
		Context("with invalid values in the plugins.nodeAffinityScoring stanza", func() {
			DescribeTable("The request should fail with", func(object *v1beta1.ClusterPodPlacementConfig) {
				By("Ensure no ClusterPodPlacementConfig exists")
				cppc := &v1beta1.ClusterPodPlacementConfig{}
				err := k8sClient.Get(ctx, crclient.ObjectKey{
					Name: common.SingletonResourceObjectName,
				}, cppc)
				Expect(errors.IsNotFound(err)).To(BeTrue(), "The ClusterPodPlacementConfig should not exist")
				// Expect(errors.IsNotFound(err)).To(BeTrue(), "The ClusterPodPlacementConfig should not exist")
				By("Create the ClusterPodPlacementConfig")
				err = k8sClient.Create(ctx, object)
				By(fmt.Sprintf("The error is: %+v", err))
				By("Verify the ClusterPodPlacementConfig is not created")
				Expect(err).To(HaveOccurred(), "The create ClusterPodPlacementConfig should not be accepted")
				By("Verify the error is 'invalid'")
				Expect(errors.IsInvalid(err)).To(BeTrue(), "The invalid ClusterPodPlacementConfig should not be accepted")
			},
				Entry("Negative weight", builder.NewClusterPodPlacementConfig().
					WithName(common.SingletonResourceObjectName).
					WithPlugins().
					WithNodeAffinityScoring(true).
					WithNodeAffinityScoringTerm(utils.ArchitectureAmd64, -100).
					Build()),
				Entry("Zero weight", builder.NewClusterPodPlacementConfig().
					WithName(common.SingletonResourceObjectName).
					WithPlugins().
					WithNodeAffinityScoring(true).
					WithNodeAffinityScoringTerm(utils.ArchitectureAmd64, 0).
					Build()),
				Entry("Excessive weight", builder.NewClusterPodPlacementConfig().
					WithName(common.SingletonResourceObjectName).
					WithPlugins().
					WithNodeAffinityScoring(true).
					WithNodeAffinityScoringTerm(utils.ArchitectureAmd64, 200).
					Build()),
				Entry("Wrong architecture", builder.NewClusterPodPlacementConfig().
					WithName(common.SingletonResourceObjectName).
					WithPlugins().
					WithNodeAffinityScoring(true).
					WithNodeAffinityScoringTerm("Wrong", 200).
					Build()),
				Entry("No terms", builder.NewClusterPodPlacementConfig().
					WithName(common.SingletonResourceObjectName).
					WithPlugins().
					WithNodeAffinityScoring(true).
					Build()),
				Entry("Missing architecture in a term", builder.NewClusterPodPlacementConfig().
					WithName(common.SingletonResourceObjectName).
					WithPlugins().
					WithNodeAffinityScoring(true).
					WithNodeAffinityScoringTerm("", 5).
					Build()),
			)
			AfterEach(func() {
				By("Ensure the ClusterPodPlacementConfig is deleted")
				err := k8sClient.Delete(ctx, builder.NewClusterPodPlacementConfig().WithName(common.SingletonResourceObjectName).Build())
				Expect(crclient.IgnoreNotFound(err)).NotTo(HaveOccurred(), "failed to delete ClusterPodPlacementConfig", err)
			})
		})
	})
})
