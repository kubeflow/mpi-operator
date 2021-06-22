module github.com/kubeflow/mpi-operator

go 1.13

require (
	github.com/emicklei/go-restful v2.9.6+incompatible // indirect
	github.com/go-openapi/spec v0.19.2
	github.com/kubeflow/common v0.3.3
	github.com/kubernetes-sigs/kube-batch v0.5.0
	github.com/prometheus/client_golang v1.5.1
	github.com/stretchr/testify v1.4.0
	golang.org/x/net v0.0.0-20200226121028-0de0cce0169b // indirect
	k8s.io/api v0.16.15
	k8s.io/apimachinery v0.16.15
	k8s.io/apiserver v0.16.15
	k8s.io/client-go v10.0.0+incompatible
	k8s.io/klog v1.0.0
	k8s.io/kube-openapi v0.0.0-20200410163147-594e756bea31
	k8s.io/sample-controller v0.16.15
	volcano.sh/apis v1.2.0-k8s1.16.15
)

replace (
	k8s.io/api => k8s.io/api v0.15.10
	k8s.io/apimachinery => k8s.io/apimachinery v0.15.10
	k8s.io/apiserver => k8s.io/apiserver v0.15.10
	k8s.io/client-go => k8s.io/client-go v0.15.10
	k8s.io/code-generator => k8s.io/code-generator v0.15.10
	k8s.io/klog => k8s.io/klog v1.0.0
)
