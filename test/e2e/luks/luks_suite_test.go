package luks_test

import (
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

func TestLuks(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "LUKS Suite")
}
