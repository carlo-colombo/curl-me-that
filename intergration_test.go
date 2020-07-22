package main_test

import (
	"fmt"
	"math/rand"
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

var namespace string

func createConfigMap(annotations map[string]string) (*v1.ConfigMap, string) {
	name := fmt.Sprintf("test-config-map-%d", rand.Int())

	cm, err := kubeClient.
		CoreV1().
		ConfigMaps(namespace).
		Create(context.TODO(), &v1.ConfigMap{
			ObjectMeta: metav1.ObjectMeta{
				Name:        name,
				Namespace:   namespace,
				Annotations: annotations,
			},
		}, metav1.CreateOptions{})

	Expect(err).NotTo(HaveOccurred())
	return cm, name
}

var _ = Describe("Intergration", func() {
	BeforeEach(func() {
		var err error
		session, err = gexec.Start(CurlMeThatCommand, GinkgoWriter, GinkgoWriter)

		Expect(err).NotTo(HaveOccurred())
		cfg, err := clientcmd.BuildConfigFromFlags("", "/home/ilich/.kube/config")
		Expect(err).NotTo(HaveOccurred())

		kubeClient, err = kubernetes.NewForConfig(cfg)
		Expect(err).NotTo(HaveOccurred())

		namespace = fmt.Sprintf("test-ns-%d", rand.Int())

		kubeClient.
			CoreV1().Namespaces().Create(context.TODO(), &v1.Namespace{
			ObjectMeta: metav1.ObjectMeta{
				Name: namespace,
			},
		}, metav1.CreateOptions{})

		server = ghttp.NewServer()
	})

	Context("when the url is reachable", func() {
		It("does a GET request and put the returned value in the data", func() {

			server.AppendHandlers(
				ghttp.CombineHandlers(
					ghttp.VerifyRequest("GET", "/"),
					ghttp.RespondWith(http.StatusOK, "my-value"),
				),
			)

			parsedUrl, err := url.Parse(server.URL())
			Expect(err).NotTo(HaveOccurred())

			parsedUrl.Scheme = ""

			_, name := createConfigMap(map[string]string{
				"x-k8s.io/curl-me-that": "mykey=" + parsedUrl.String(),
			})

			Eventually(func() map[string]string {
				ucm, err := kubeClient.
					CoreV1().
					ConfigMaps(namespace).
					Get(
						context.TODO(),
						name,
						metav1.GetOptions{})
				Expect(err).NotTo(HaveOccurred())
				return ucm.Data
			}).Should(HaveKeyWithValue("mykey", "my-value"))

		})
	})

	AfterEach(func() {
		err := kubeClient.
			CoreV1().Namespaces().Delete(context.TODO(), namespace,
			metav1.DeleteOptions{})
		Expect(err).NotTo(HaveOccurred())

		server.Close()
		session.Kill()
		<-session.Exited
	})
})
