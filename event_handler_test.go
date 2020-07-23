package main_test

import (
	"bytes"
	"errors"
	"io"
	"io/ioutil"
	"net/http"

	"golang.org/x/net/context"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/ginkgo/extensions/table"
	. "github.com/onsi/gomega"
	. "github.com/onsi/gomega/gstruct"

	v1 "k8s.io/api/core/v1"
	clientset "k8s.io/client-go/kubernetes"
	"k8s.io/client-go/kubernetes/fake"

	cmt "github.com/carlo-colombo/curl-me-that"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type mockHTTPClient struct {
	responseBody string
	url          string
}

func (m *mockHTTPClient) get(url string) (*http.Response, error) {
	m.url = url
	return &http.Response{
		Body: ioutil.NopCloser(bytes.NewBufferString(m.responseBody)),
	}, nil
}

var _ = Describe("EventHandler", func() {
	Describe("NewResourceEventHandlerFunc", func() {
		Describe("AddFunc", func() {
			It("updates the config map", func() {
				cm := v1.ConfigMap{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "test-config-map",
						Namespace: "testns",
						Annotations: map[string]string{
							"x-k8s.io/curl-me-that": "mykey=https://foobar.com",
						},
					},
				}

				mockClient := mockHTTPClient{responseBody: "a remote answer"}
				fcs := fake.NewSimpleClientset(&cm)
				rehf := cmt.NewResourceEventHandlerFunc(fcs, mockClient.get)

				rehf.AddFunc(&cm)

				newCM := getConfigMap(fcs, "test-config-map")

				By("extracting the key from the value of the annotation")
				Expect(newCM.Data).To(HaveKey("mykey"))

				By("doing a GET to the url in the value")
				Expect(newCM.Data["mykey"]).To(Equal("a remote answer"))
				Expect(mockClient.url).To(Equal("https://foobar.com"))
			})

			It("prefixes the url with http if the schema is missing when doing the request", func() {
				cm := v1.ConfigMap{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "test-config-map",
						Namespace: "testns",
						Annotations: map[string]string{
							"x-k8s.io/curl-me-that": "mykey=foobar.com",
						},
					},
				}

				mockClient := mockHTTPClient{responseBody: "a remote answer"}
				fcs := fake.NewSimpleClientset(&cm)
				rehf := cmt.NewResourceEventHandlerFunc(fcs, mockClient.get)

				rehf.AddFunc(&cm)

				Expect(mockClient.url).To(Equal("http://foobar.com"))
			})

			It("handle url with querystring", func() {
				cm := v1.ConfigMap{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "test-config-map",
						Namespace: "testns",
						Annotations: map[string]string{
							"x-k8s.io/curl-me-that": "mykey=foobar.com?bar=zot",
						},
					},
				}

				mockClient := mockHTTPClient{responseBody: "a remote answer"}
				fcs := fake.NewSimpleClientset(&cm)
				rehf := cmt.NewResourceEventHandlerFunc(fcs, mockClient.get)

				rehf.AddFunc(&cm)

				Expect(mockClient.url).To(Equal("http://foobar.com?bar=zot"))
			})

			It("ignores config map without `x-k8s.io/curl-me-that` annotation", func() {
				cm := v1.ConfigMap{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "test-config-map",
						Namespace: "testns",
						Annotations: map[string]string{
							"not-x-k8s.io/curl-me-that-not": "",
						},
					},
				}
				fcs := fake.NewSimpleClientset(&cm)
				rehf := cmt.NewResourceEventHandlerFunc(fcs, http.Get)

				rehf.AddFunc(&cm)

				newCM := getConfigMap(fcs, "test-config-map")

				Expect(newCM).ToNot(BeNil())
				Expect(newCM.Data).ToNot(HaveKey("joke"))
			})

			Context("when the config map already contains data", func() {
				It("integrates the data if the key is not already present", func() {
					cm := v1.ConfigMap{
						ObjectMeta: metav1.ObjectMeta{
							Name:      "test-config-map",
							Namespace: "testns",
							Annotations: map[string]string{
								"x-k8s.io/curl-me-that": "mykey=https://foobar.com",
							},
						},
						Data: map[string]string{
							"another-key": "will remain",
						},
					}

					mockClient := mockHTTPClient{responseBody: "a remote answer"}
					fcs := fake.NewSimpleClientset(&cm)
					rehf := cmt.NewResourceEventHandlerFunc(fcs, mockClient.get)

					rehf.AddFunc(&cm)

					newCM := getConfigMap(fcs, "test-config-map")
					Expect(newCM.Data).To(
						SatisfyAll(
							HaveKeyWithValue("mykey", "a remote answer"),
							HaveKeyWithValue("another-key", "will remain"),
						))

				})
				It("does not change the configmap if the key is already present", func() {
					cm := v1.ConfigMap{
						ObjectMeta: metav1.ObjectMeta{
							Name:      "test-config-map",
							Namespace: "testns",
							Annotations: map[string]string{
								"x-k8s.io/curl-me-that": "mykey=https://foobar.com",
							},
						},
						Data: map[string]string{
							"another-key": "will remain",
							"mykey":       "this too",
						},
					}

					mockClient := mockHTTPClient{responseBody: "a remote answer"}
					fcs := fake.NewSimpleClientset(&cm)
					rehf := cmt.NewResourceEventHandlerFunc(fcs, mockClient.get)

					rehf.AddFunc(&cm)

					newCM := getConfigMap(fcs, "test-config-map")

					Expect(newCM.Data).To(
						SatisfyAll(
							HaveKeyWithValue("another-key", "will remain"),
							HaveKeyWithValue("mykey", "this too"),
						))

				})
			})

			DescribeTable("events are added when it's not possible to add content to data",
				func(message string, annotationValue string, fn cmt.HttpGetFn) {
					cm := v1.ConfigMap{
						ObjectMeta: metav1.ObjectMeta{
							Name:      "test-config-map",
							Namespace: "testns",
							Annotations: map[string]string{
								"x-k8s.io/curl-me-that": annotationValue,
							},
						},
					}

					fcs := fake.NewSimpleClientset(&cm)
					rehf := cmt.NewResourceEventHandlerFunc(fcs, fn)

					rehf.AddFunc(&cm)
					Eventually(
						listEvents("test-config-map", "testns", fcs),
					).Should(SatisfyAll(
						HaveLen(1),
						WithTransform(func(list []v1.Event) v1.Event {
							return list[0]
						}, MatchFields(IgnoreExtras, Fields{
							"Message": ContainSubstring(message),
							"Reason":  ContainSubstring("Failed"),
							"InvolvedObject": MatchFields(IgnoreExtras, Fields{
								"Name": Equal("test-config-map"),
							}),
						}))))

				},
				Entry("http client return an error", "you got an error", "mykey=https://foobar.com", func(string) (*http.Response, error) {
					return nil, errors.New("you got an error")
				}),
				Entry("non 2xx status codes", "401", "mykey=https://foobar.com", func(string) (*http.Response, error) {
					return &http.Response{StatusCode: 401}, nil
				}),
				Entry("invalid url", "cannot parse url", "mykey=http://[foosomething-invalid", nopGet),
				Entry("empty url", "empty url", "mykey=", nopGet),
				Entry("annotation without =", "cannot parse annotation value", "mykey", nopGet),
				Entry("empty annotation", "cannot parse annotation value", "", nopGet),
				Entry("response has an invalid body", "response body cannot be read", "mykey=foobar.com", func(string) (*http.Response, error) {
					return &http.Response{
						StatusCode: 200,
						Body:       failingBody{},
					}, nil
				}),
			)
		})
	})
})

func nopGet(string) (*http.Response, error) {
	return nil, nil
}

type failingBody struct{}

func (m failingBody) WriteTo(w io.Writer) (n int64, err error) {
	return 0, errors.New("some error")
}
func (m failingBody) Read(p []byte) (n int, err error) {
	return 0, errors.New("some error")
}
func (m failingBody) Close() error {
	return errors.New("some error")
}

func listEvents(name string, namespace string, client clientset.Interface) func() []v1.Event {
	return func() []v1.Event {
		list, err := client.
			CoreV1().
			Events(namespace).
			List(context.TODO(), metav1.ListOptions{
				FieldSelector: "involvedObject.name=" + name,
			})
		Expect(err).ToNot(HaveOccurred())
		return list.Items
	}
}

func getConfigMap(fcs *fake.Clientset, name string) *v1.ConfigMap {
	cm, err := fcs.
		CoreV1().
		ConfigMaps("testns").
		Get(
			context.TODO(),
			name,
			metav1.GetOptions{})

	Expect(err).ToNot(HaveOccurred())

	return cm
}
