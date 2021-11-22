module github.com/kubeflow/mpi-operator

go 1.13

require (
	github.com/emicklei/go-restful v2.9.6+incompatible // indirect
	github.com/go-logr/logr v0.3.0 // indirect
	github.com/go-openapi/spec v0.20.3
	github.com/google/go-cmp v0.5.6 // indirect
	github.com/googleapis/gnostic v0.5.1 // indirect
	github.com/hashicorp/golang-lru v0.5.4 // indirect
	github.com/imdario/mergo v0.3.10 // indirect
	github.com/kubeflow/common v0.4.0
	github.com/onsi/ginkgo v1.14.1 // indirect
	github.com/onsi/gomega v1.10.2 // indirect
	github.com/prometheus/client_golang v1.10.0
	golang.org/x/crypto v0.0.0-20210616213533-5ff15b29337e // indirect
	golang.org/x/time v0.0.0-20200630173020-3af7569d3a1e // indirect
	google.golang.org/appengine v1.6.6 // indirect
	k8s.io/api v0.19.9
	k8s.io/apimachinery v0.19.9
	k8s.io/apiserver v0.19.9
	k8s.io/client-go v0.19.9
	k8s.io/klog v1.0.0
	k8s.io/kube-openapi v0.0.0-20200805222855-6aeccd4b50c6
	k8s.io/sample-controller v0.19.9
	k8s.io/utils v0.0.0-20200912215256-4140de9c8800 // indirect
	volcano.sh/apis v1.2.0-k8s1.19.6
)

replace k8s.io/code-generator => k8s.io/code-generator v0.19.9
