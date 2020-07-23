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

Once running against a cluster

