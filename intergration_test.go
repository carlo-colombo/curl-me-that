package main_test

import (
	"net/http"
	"net/url"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gexec"
	"github.com/onsi/gomega/ghttp"
	"golang.org/x/net/context"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/clientcmd"
)

var kubeClient *kubernetes.Clientset

var server *ghttp.Server

func createConfigMap(ns string, name string, annotations map[string]string) *v1.ConfigMap {
	cm, err := kubeClient.
		CoreV1().
		ConfigMaps("testns").
		Create(context.TODO(), &v1.ConfigMap{
			ObjectMeta: metav1.ObjectMeta{
				Name:        name,
				Namespace:   ns,
				Annotations: annotations,
			},
		}, metav1.CreateOptions{})

	Expect(err).NotTo(HaveOccurred())
	return cm
}

var _ = FDescribe("Intergration", func() {
	BeforeEach(func() {
		var err error
		session, err = gexec.Start(CurlMeThatCommand, GinkgoWriter, GinkgoWriter)

		Expect(err).NotTo(HaveOccurred())
		cfg, err := clientcmd.BuildConfigFromFlags("", "/home/ilich/.kube/config")
		Expect(err).NotTo(HaveOccurred())

		kubeClient, err = kubernetes.NewForConfig(cfg)
		Expect(err).NotTo(HaveOccurred())

		kubeClient.
			CoreV1().Namespaces().Create(context.TODO(), &v1.Namespace{
			ObjectMeta: metav1.ObjectMeta{
				Name: "testns",
			},
		}, metav1.CreateOptions{})

		server = ghttp.NewServer()
	})

	It("starts", func() {

		server.AppendHandlers(
			ghttp.CombineHandlers(
				ghttp.VerifyRequest("GET", "/"),
				ghttp.RespondWith(http.StatusOK, "barzot"),
			),
		)

		parsedUrl, err := url.Parse(server.URL())
		Expect(err).NotTo(HaveOccurred())

		parsedUrl.Scheme = ""

		cm := createConfigMap("testns", "test-config-map2", map[string]string{
			"x-k8s.io/curl-me-that": "mykey=" + parsedUrl.String(),
		})

		Eventually(func() map[string]string {
			ucm, err := kubeClient.
				CoreV1().
				ConfigMaps("testns").
				Get(
					context.TODO(),
					cm.Name,
					metav1.GetOptions{})
			Expect(err).NotTo(HaveOccurred())
			return ucm.Data
		}).Should(HaveKeyWithValue("mykey", "barzot"))

	})

	AfterEach(func() {
		err := kubeClient.
			CoreV1().Namespaces().Delete(context.TODO(), "testns",
			metav1.DeleteOptions{})
		Expect(err).NotTo(HaveOccurred())

		server.Close()
		session.Kill()
		<-session.Exited
	})
})
