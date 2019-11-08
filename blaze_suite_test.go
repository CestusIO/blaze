package blaze_test

import (
	"testing"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

func TestBlaze(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Blaze Suite")
}
