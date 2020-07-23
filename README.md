# curl-me-that

A small Kubernetes controller that watch ConfigMaps and inject data from URL inthe  `x-k8s.io/curl-me-that` annotation.

### How to run `curl-me-that` in cluster

`kubectl apply -f spec.yml` , the spec includes:
* a deployment that run a pod with the image `carlocolombo/curl-me-that`
* a service account (`curl-me-that-sa`)
* a cluster role (`curl-me-that-role`) with the necessary permissions
* a cluster role binding

### How to run `curl-me-that` out of cluster

```bash
git clone git@github.com:carlo-colombo/curl-me-that.git
cd curl-me-that
go build -o curl-me-that main.go event_handler.go

./curl-me-that --kubeconfig ~/.kube/config
```

### How it works

Once it runs agains a cluster, it watches for the creation of config map annotated with`x-k8s.io/curl-me-that`, parses the value of the annotation in the format `key=url` and then put the response of a GET request to the `url` with the `key` in the config map.

```bash
cat <<EOF | kubectl apply -f -
apiVersion: v1
kind: ConfigMap
metadata:
  name: example
  annotations:
    x-k8s.io/curl-me-that: jokes=curl-a-joke.herokuapp.com
data: {}
EOF
```

Observe the joke in the `jokes` field of the config map

`kubectl describe configmaps`

```yaml
Name:         example
Namespace:    default
Labels:       <none>
Annotations:  x-k8s.io/curl-me-that: jokes=curl-a-joke.herokuapp.com

Data
====
jokes:
----
What's the difference between a guitar and a fish? You can't tuna fish!

Events:  <none>
```

Config maps are not update if the key extracted from the annotation is already present in the config map to avoid accidentally overriding data.

Urls without schema (e.g. `example.com`) are defaulted to `http` to mimic the default behaviour of `curl`. 

Events are logged in case somethings goes wrong as host unreachable or non 2xx status code from the server. Invalid annotations values are also logged in events.

```
Name:         example
Namespace:    default
Labels:       <none>
Annotations:  x-k8s.io/curl-me-that: jokes=not-a-valid-url.com

Data
====
Events:
  Type     Reason  Age   From          Message
  ----     ------  ----  ----          -------
  Warning  Failed  82s   curl-me-that  failed to connect to not-a-valid-url.com: Get "http://not-a-valid-url.com": dial tcp: lookup not-a-valid-url.com on 10.96.0.10:53: no such host

```

### Build the image

`pack build curl-me-that --builder cloudfoundry/cnb:tiny`

The image is built using [pack](https://github.com/buildpacks/pack) from the [Cloud Native Buildpack](https://buildpacks.io/) project. The base image is `cloudfoundry/tiny` a functionally equivalent image to `gcr.io/distroless/base` built on top of Ubuntu and available as builder for Cloud Native Buildpacks that I collaborated creating. 

### Tests

To run tests execute

```bash
 KUBECONFIG=/home/ilich/.kube/config go test
 ```
 
 Note that integrations tests can fails if run against a cluster with already the `curl-me-that` controller.
 
 ### TODO
 * [ ] Implement a way to recoincile to the desired state in case of failure in updating the config maps during creation. Use `UpdateFunc` or a queue.
 
 # Open questions

1. How would you deploy your controller to a Kubernetes cluster?

> As shown in the `spec.yml` present in the repository I would deploy the controller as `deployment` (with a single replica) in the cluster, mounting a service account with the minimal required permissions and use the facilities from `client-go` to use it to authenticate against the cluster. Using a deployment, instead of a bare pod, allows the pod to be restarted in case of crash or eviction from the node. The controller does not support multiple replicas at the moment, it would probably work but without any gain as all the instance will try to do the same work.

6. In the context of your controller, what is the observed state and what is the desired state?

> The desired state is the config map with the result of curling the url in the data, and the observed state is the config map passed from the informer. The controller act as user that observe a config map with the annotation, perform a GET request and then submit a new desired state of a config map with the result of the request in the data.

7. The content returned when curling URLs may be always different. How is it going to affect your controllers?

> I considered this and decided that if the key is already present in the config maps it should not be changed by the controller. So if the controller is restarted and the config maps are resubmitted to the controller it check the presence of the the key and skip to curl the url. This also means that if a config maps is created on the cluster already containing the key present in the annotation the original value is preserved and nothing is performed.
>
> A consequence of different content is that deleting and then recreating a config map is not deterministic: the content can have changed or the url can be not anymore accessibile or viceversa. This is not really controllable from the controller, the only possible solution would be to cache the first result of connecting to the url (content, response status, availability) and always use it but it would reduced the functionality of the controller.
