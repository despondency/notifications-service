package notification_test

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"testing"
)

func TestNotifications(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Notifications Suite")
}
