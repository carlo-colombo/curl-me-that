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
	"k8s.io/client-go/tools/reference"
	"k8s.io/klog"
)

type HttpGetFn func(string) (*http.Response, error)

func logErrorf(recorder record.EventRecorder, cm *v1.ConfigMap, messagefmt string, args ...interface{}) {
	klog.Errorf(messagefmt, args...)

	ref, err := reference.GetReference(scheme.Scheme, cm)

	if err != nil {
		// as we are getting the configMap from the informers
		// it should never happen
		panic(err)
	}

	recorder.Eventf(ref,
		v1.EventTypeWarning,
		"Failed",
		messagefmt, args...)
}

func NewResourceEventHandlerFunc(clientset clientset.Interface, getFn HttpGetFn) cache.ResourceEventHandlerFuncs {

	return cache.ResourceEventHandlerFuncs{
		AddFunc: func(obj interface{}) {
			cm := obj.(*v1.ConfigMap)

			if value, ok := cm.GetAnnotations()["x-k8s.io/curl-me-that"]; ok {
				klog.Info("annotation detected: ", value)
				recorder := eventRecorder(clientset, cm.Namespace)
				components := strings.SplitN(value, "=", 2)

				if len(components) != 2 {
					logErrorf(recorder, cm, "cannot parse annotation value, miss '=' : %s", value)
					return
				}

				if components[1] == "" {
					logErrorf(recorder, cm, "empty url: %s", value)
					return
				}

				parsedURL, err := url.Parse(components[1])

				if err != nil {
					logErrorf(recorder, cm, "cannot parse url %s: %s", components[1], err)
					return
				}

				if parsedURL.Scheme == "" {
					parsedURL.Scheme = "http"
				}

				resp, err := getFn(parsedURL.String())

				if err != nil {
					logErrorf(recorder, cm, "failed to connect to %s: %s", components[1], err)
					return
				}
				if resp.StatusCode >= 300 {
					logErrorf(recorder, cm, "non valid status code connecting to %s: %d", parsedURL.String(), resp.StatusCode)
					return
				}

				buf := new(strings.Builder)
				_, err = io.Copy(buf, resp.Body)

				if err != nil {
					logErrorf(recorder, cm, "response body cannot be read: %s", err)
					return
				}

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
			}
		},
	}
}

func eventRecorder(kubeClient clientset.Interface, namespace string) record.EventRecorder {
	eventBroadcaster := record.NewBroadcaster()
	eventBroadcaster.StartLogging(klog.Infof)
	eventBroadcaster.StartRecordingToSink(
		&typedcorev1.EventSinkImpl{
			Interface: kubeClient.CoreV1().Events(namespace)})
	recorder := eventBroadcaster.NewRecorder(
		scheme.Scheme,
		v1.EventSource{Component: "curl-me-that"})
	return recorder
}
