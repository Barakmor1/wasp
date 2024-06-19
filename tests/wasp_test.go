package tests

import (
	"context"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"kubevirt.io/wasp/tests/framework"
)

var _ = Describe("Wasp tests", func() {
	f := framework.NewFramework("wasp-test")
	Context("Wasp", func() {
		It("first fake test", func(ctx context.Context) {
			_, err := f.K8sClient.CoreV1().Namespaces().Get(ctx, f.WaspNamespace, v1.GetOptions{})
			Expect(err).ShouldNot(HaveOccurred())
			Expect(true).ToNot(BeFalse())
		})
	})
})
