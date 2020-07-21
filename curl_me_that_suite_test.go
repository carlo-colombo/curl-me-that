package main_test

import (
	"os"
	"os/exec"
	"testing"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gexec"
)

var (
	curlMeThatPath    string
	session           *gexec.Session
	CurlMeThatCommand *exec.Cmd
)

var _ = SynchronizedBeforeSuite(func() []byte {
	binPath, err := gexec.Build("github.com/carlo-colombo/curl-me-that")
	Expect(err).NotTo(HaveOccurred())

	return []byte(binPath)
}, func(data []byte) {
	curlMeThatPath = string(data)
	CurlMeThatCommand = exec.Command(
		curlMeThatPath,
		"--kubeconfig",
		os.Getenv("KUBECONFIG"))
})

var _ = SynchronizedAfterSuite(func() {
	gexec.CleanupBuildArtifacts()
}, func() {})

func TestCurlMeThat(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "CurlMeThat Suite")
}
