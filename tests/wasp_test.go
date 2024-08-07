package tests

import (
	"context"
	"fmt"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	corev1 "kubevirt.io/api/core/v1"
	eviction_controller "kubevirt.io/wasp/pkg/wasp/eviction-controller"
	"kubevirt.io/wasp/tests/framework"
	node_stat "kubevirt.io/wasp/tests/node-stat"
	"kubevirt.io/wasp/tests/pod"
	"time"
)

/*
Before you run these tests please make sure swap is on
*/
var _ = Describe("Wasp tests", func() {
	f := framework.NewFramework("wasp-test")
	var nodes *v1.NodeList
	Context("Wasp", func() {
		BeforeEach(func() {
			nodes = GetAllSchedulableNodes(f.K8sClient)
			Expect(len(nodes.Items)).To(BeNumerically(">", 1),
				"should have at least two schedulable nodes in the cluster")
		})
		It("Simple eviction test", func(ctx context.Context) {
			nodeToEvict := nodes.Items[0]
			stopChan := make(chan struct{})
			defer close(stopChan)
			go printNodeStat(f, nodeToEvict, stopChan)
			Expect(true).ToNot(BeFalse())
			highmemPod := pod.GetMemhogPod("memory-hog-pod", "memory-hog", v1.ResourceRequirements{})
			highmemPod.Spec.NodeName = nodeToEvict.Name
			innocentPod := pod.InnocentPod()
			innocentPod.Spec.NodeName = nodeToEvict.Name
			innocentPod, err := f.K8sClient.CoreV1().Pods(f.Namespace.Name).Create(context.Background(), innocentPod, metav1.CreateOptions{})
			Expect(err).ShouldNot(HaveOccurred())
			highmemPod, err = f.K8sClient.CoreV1().Pods(f.Namespace.Name).Create(context.Background(), highmemPod, metav1.CreateOptions{})
			Expect(err).ShouldNot(HaveOccurred())

			Eventually(func() []v1.Taint {
				node, err := f.K8sClient.CoreV1().Nodes().Get(context.Background(), nodeToEvict.Name, metav1.GetOptions{})
				Expect(err).ToNot(HaveOccurred())
				return node.Spec.Taints
			}).WithTimeout(5 * time.Minute).WithPolling(300 * time.Millisecond).Should(ContainElement(v1.Taint{
				Key:    eviction_controller.WaspTaint,
				Effect: v1.TaintEffectNoSchedule,
			}))

			Eventually(func() bool {
				_, err := f.K8sClient.CoreV1().Pods(highmemPod.Namespace).Get(context.Background(), highmemPod.Name, metav1.GetOptions{})
				return errors.IsNotFound(err)
			}).WithTimeout(1 * time.Minute).WithPolling(1 * time.Second).Should(BeTrue())

			Eventually(func() []v1.Taint {
				node, err := f.K8sClient.CoreV1().Nodes().Get(context.Background(), nodeToEvict.Name, metav1.GetOptions{})
				Expect(err).ToNot(HaveOccurred())
				return node.Spec.Taints
			}).WithTimeout(5 * time.Minute).WithPolling(1 * time.Second).ShouldNot(ContainElement(v1.Taint{
				Key:    eviction_controller.WaspTaint,
				Effect: v1.TaintEffectNoSchedule,
			}))
		})
	})
})

// GetAllSchedulableNodes returns list of Nodes which are "KubeVirt" schedulable.
func GetAllSchedulableNodes(K8sClient *kubernetes.Clientset) *v1.NodeList {
	nodeList, err := K8sClient.CoreV1().Nodes().List(context.Background(), metav1.ListOptions{
		LabelSelector: corev1.NodeSchedulable + "=" + "true",
	})
	Expect(err).ToNot(HaveOccurred(), "Should list compute nodeList")
	return nodeList
}

func printNodeStat(f *framework.Framework, nodeToEval v1.Node, stopChan chan struct{}) {
	firstTime := true
	ticker := time.NewTicker(3 * time.Second)
	defer ticker.Stop()

	var lastPsin, lastPsout int

	for {
		select {
		case <-ticker.C:
			psin, errPsin := node_stat.GetSwapInPages(f, nodeToEval)
			psout, errPsout := node_stat.GetSwapOutPages(f, nodeToEval)
			availMem, errAvailMem := node_stat.GetAvailableMemSizeInKib(f, nodeToEval)

			if errPsin != nil || errPsout != nil || errAvailMem != nil {
				fmt.Printf("Error retrieving metrics: %v, %v, %v\n", errPsin, errPsout, errAvailMem)
			} else {
				psinIncrement := psin - lastPsin
				psoutIncrement := psout - lastPsout
				avgPsinPerSec := psinIncrement / 3
				avgPsoutPerSec := psoutIncrement / 3
				if !firstTime {
					fmt.Printf("Available Memory: %d KiB, Avrage SwapIn Increment Per Second: %d, Avrage SwapOut Increment Per Second: %d\n", availMem, avgPsinPerSec, avgPsoutPerSec)
				}
				firstTime = false

				lastPsin = psin
				lastPsout = psout
			}
		case <-stopChan:
			return
		}
	}
}
