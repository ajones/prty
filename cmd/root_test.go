package cmd

import (
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("cmd", func() {
	Describe("placeholder", func() {
		It("should run this test as a placeholder", func() {
			Expect("foo").ToNot(Equal("bar"))
		})
	})
})
