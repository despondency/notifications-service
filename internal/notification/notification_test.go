package notification

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("Notification test", func() {

	var (
		destination         string
		err                 error
		actualDestination   Destination
		expectedDestination Destination
	)

	JustBeforeEach(func() {
		actualDestination, err = toServerNotificationDestination(destination)
	})

	Context("SMS Destination", func() {
		BeforeEach(func() {
			destination = "SMS"
			expectedDestination = 0
		})

		It("should have no err and be converted", func() {
			Expect(actualDestination).To(Equal(expectedDestination))
			Expect(err).To(BeNil())
		})
	})

	Context("Email Destination", func() {
		BeforeEach(func() {
			destination = "EMAIL"
			expectedDestination = 1
		})

		It("should have no err and be converted", func() {
			Expect(actualDestination).To(Equal(expectedDestination))
			Expect(err).To(BeNil())
		})
	})

	Context("Slack Destination", func() {
		BeforeEach(func() {
			destination = "SLACK"
			expectedDestination = 2
		})

		It("should have no err and be converted", func() {
			Expect(actualDestination).To(Equal(expectedDestination))
			Expect(err).To(BeNil())
		})
	})

	Context("Unknown Destination", func() {
		BeforeEach(func() {
			destination = "Unknown Destination"
			expectedDestination = -1
		})

		It("should have no err and be converted", func() {
			Expect(actualDestination).To(Equal(expectedDestination))
			Expect(err).To(Equal(ErrNoSuchDestination))
		})
	})

})
