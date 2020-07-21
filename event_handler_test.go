package main_test

import (
	"bytes"
	"errors"
	"io/ioutil"
	"net/http"

	"golang.org/x/net/context"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/ginkgo/extensions/table"
	. "github.com/onsi/gomega"

	v1 "k8s.io/api/core/v1"
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

				newCM, err := fcs.
					CoreV1().
					ConfigMaps("testns").
					Get(
						context.TODO(),
						"test-config-map",
						metav1.GetOptions{})

				Expect(err).ToNot(HaveOccurred())
				Expect(newCM).ToNot(BeNil())

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

			It("only updates `x-k8s.io/curl-me-that` annotated config maps, ignoring others", func() {
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

				newCM, err := fcs.
					CoreV1().
					ConfigMaps("testns").
					Get(
						context.TODO(),
						"test-config-map",
						metav1.GetOptions{})

				Expect(err).ToNot(HaveOccurred())
				Expect(newCM).ToNot(BeNil())
				Expect(newCM.Data).ToNot(HaveKey("joke"))
			})

			Context("when the config map already contains data", func() {
				It("only replaces the key in the value of the annotation", func() {

					cm := v1.ConfigMap{
						ObjectMeta: metav1.ObjectMeta{
							Name:      "test-config-map",
							Namespace: "testns",
							Annotations: map[string]string{
								"x-k8s.io/curl-me-that": "mykey=https://foobar.com",
							},
						},
						Data: map[string]string{
							"mykey":       "to be replaced",
							"another-key": "will remain",
						},
					}

					mockClient := mockHTTPClient{responseBody: "a remote answer"}
					fcs := fake.NewSimpleClientset(&cm)
					rehf := cmt.NewResourceEventHandlerFunc(fcs, mockClient.get)

					rehf.AddFunc(&cm)

					newCM, err := fcs.
						CoreV1().
						ConfigMaps("testns").
						Get(
							context.TODO(),
							"test-config-map",
							metav1.GetOptions{})

					Expect(err).ToNot(HaveOccurred())

					Expect(newCM.Data).To(
						SatisfyAll(
							HaveKeyWithValue("mykey", "a remote answer"),
							HaveKeyWithValue("another-key", "will remain"),
						))

				})
			})

			XDescribeTable("events are added when the url does not answer with success",
				func(fn cmt.HttpGetFn) {
					cm := v1.ConfigMap{
						ObjectMeta: metav1.ObjectMeta{
							Name:      "test-config-map",
							Namespace: "testns",
							Annotations: map[string]string{
								"x-k8s.io/curl-me-that": "mykey=https://foobar.com",
							},
						},
					}

					fcs := fake.NewSimpleClientset(&cm)
					rehf := cmt.NewResourceEventHandlerFunc(fcs, fn)

					rehf.AddFunc(&cm)

					list, err := fcs.
						CoreV1().
						Events("testns").
						List(context.TODO(), metav1.ListOptions{
							FieldSelector: "involvedObject.name=test-config-map",
						})
					Expect(err).ToNot(HaveOccurred())

					Expect(list.Items).ToNot(BeEmpty())
					Expect(list.Items[0].Message).To(Equal("some error"))
					Expect(list.Items[0].Reason).To(Equal("adasd"))
				},
				Entry("http client return an error", func(string) (*http.Response, error) {
					return nil, errors.New("you got an error")
				}),
			)
		})
	})
})
