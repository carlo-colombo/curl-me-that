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
