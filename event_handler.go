package main

import (
	"io"
	"net/http"
	"net/url"
	"strings"

	"golang.org/x/net/context"
	v1 "k8s.io/api/core/v1"
	v1meta "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/deprecated/scheme"
	clientset "k8s.io/client-go/kubernetes"
	typedcorev1 "k8s.io/client-go/kubernetes/typed/core/v1"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/tools/record"
	"k8s.io/klog"
)

type HttpGetFn func(string) (*http.Response, error)

func NewResourceEventHandlerFunc(clientset clientset.Interface, getFn HttpGetFn) cache.ResourceEventHandlerFuncs {

	return cache.ResourceEventHandlerFuncs{
		AddFunc: func(obj interface{}) {
			cm := obj.(*v1.ConfigMap)

			if value, ok := cm.GetAnnotations()["x-k8s.io/curl-me-that"]; ok {
				klog.Info("annotation detected: ", value)
				// recorder, _ := eventRecorder(clientset, cm.Namespace)
				components := strings.SplitN(value, "=", 2)

				parsedURL, _ := url.Parse(components[1])

				if parsedURL.Scheme == "" {
					parsedURL.Scheme = "http"
				}

				resp, err := getFn(parsedURL.String())

				if err != nil {
					klog.Errorf("boo errror %s", err)
					// ref, _ := reference.GetReference(scheme.Scheme, cm)
					// recorder.Event(ref, v1.EventTypeWarning, "asdasd", "asdasdasd")
					return
				}

				buf := new(strings.Builder)
				_, _ = io.Copy(buf, resp.Body)

				if cm.Data == nil {
					cm.Data = map[string]string{}
				}

				cm.Data[components[0]] = buf.String()

				clientset.
					CoreV1().
					ConfigMaps(cm.Namespace).
					Update(
						context.TODO(),
						cm,
						v1meta.UpdateOptions{})

				// ref, _ := reference.GetReference(scheme.Scheme, cm)
				// recorder.Event(ref, v1.EventTypeNormal, "cm updated", "cm update")

				// clientset.
				// 	CoreV1().
				// 	Events(cm.Namespace).
				// 	Create(context.TODO(), &v1.Event{
				// 		InvolvedObject: ref,
				// 		ObjectMeta: v1meta.ObjectMeta{
				// 			Name: "event name",
				// 		},
				// 	}, v1meta.CreateOptions{})
			}
		},
	}
}

func eventRecorder(kubeClient clientset.Interface, namespace string) (record.EventRecorder, error) {
	eventBroadcaster := record.NewBroadcaster()
	eventBroadcaster.StartLogging(klog.Infof)
	eventBroadcaster.StartRecordingToSink(
		&typedcorev1.EventSinkImpl{
			Interface: kubeClient.CoreV1().Events(namespace)})
	recorder := eventBroadcaster.NewRecorder(
		scheme.Scheme,
		v1.EventSource{Component: "curl-me-that"})
	return recorder, nil
}
