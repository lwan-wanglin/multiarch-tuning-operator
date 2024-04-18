package operator_test

import (
	"context"
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	ctrl "sigs.k8s.io/controller-runtime"
	runtimeclient "sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/openshift/multiarch-tuning-operator/controllers/operator"
	"github.com/openshift/multiarch-tuning-operator/pkg/e2e"
	"github.com/openshift/multiarch-tuning-operator/pkg/utils"
)

var (
	client    runtimeclient.Client
	clientset *kubernetes.Clientset
	ctx       context.Context
	suiteLog  = ctrl.Log.WithName("setup")
)

func init() {
	e2e.CommonInit()
}

func TestE2E(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Multiarch Tuning Operator Suite (Operator E2E)", Label("e2e", "operator"))
}

var _ = BeforeSuite(func() {
	client, clientset, ctx, suiteLog = e2e.CommonBeforeSuite()
})

func deploymentsAreRunning(g Gomega) {
	d, err := clientset.AppsV1().Deployments(utils.Namespace()).Get(ctx, operator.PodPlacementControllerName,
		metav1.GetOptions{})
	g.Expect(err).NotTo(HaveOccurred())
	g.Expect(d.Status.AvailableReplicas).To(Equal(*d.Spec.Replicas),
		"at least one pod placement controller replicas is not available yet")
	d, err = clientset.AppsV1().Deployments(utils.Namespace()).Get(ctx, operator.PodPlacementWebhookName,
		metav1.GetOptions{})
	g.Expect(err).NotTo(HaveOccurred())
	g.Expect(d.Status.AvailableReplicas).To(Equal(*d.Spec.Replicas),
		"at least one pod placement webhook replicas is not available yet")
}

func deploymentsAreDeleted(g Gomega) {
	_, err := clientset.AppsV1().Deployments(utils.Namespace()).Get(ctx, operator.PodPlacementControllerName,
		metav1.GetOptions{})
	g.Expect(err).To(HaveOccurred())
	g.Expect(apierrors.IsNotFound(err)).To(BeTrue())
	_, err = clientset.AppsV1().Deployments(utils.Namespace()).Get(ctx, operator.PodPlacementWebhookName,
		metav1.GetOptions{})
	g.Expect(err).To(HaveOccurred())
	g.Expect(apierrors.IsNotFound(err)).To(BeTrue())
}
