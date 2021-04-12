package handlers

import (
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("HandlePing", func() {
	Describe("getPongMessage", func() {
		It("should return the string 'pong", func() {
			//By("starting with no registered routes")
			result := getPongMessage()
			Expect(result).To(Equal("pong"))
		})
	})
})
