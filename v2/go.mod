module github.com/kubeflow/mpi-operator/v2

go 1.15

require (
	github.com/emicklei/go-restful v2.9.6+incompatible // indirect
	github.com/go-openapi/spec v0.19.3
	github.com/google/go-cmp v0.5.6
	github.com/kubeflow/common v0.3.4
	github.com/prometheus/client_golang v1.7.1
	golang.org/x/crypto v0.0.0-20210616213533-5ff15b29337e
	k8s.io/api v0.19.9
	k8s.io/apimachinery v0.19.9
	k8s.io/apiserver v0.19.9
	k8s.io/client-go v0.19.9
	k8s.io/klog v1.0.0
	k8s.io/kube-openapi v0.0.0-20200805222855-6aeccd4b50c6
	k8s.io/sample-controller v0.19.9
	sigs.k8s.io/controller-runtime v0.7.2
	volcano.sh/apis v1.2.0-k8s1.19.6
)

replace k8s.io/code-generator => k8s.io/code-generator v0.19.9
