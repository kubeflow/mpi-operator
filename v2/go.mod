module github.com/kubeflow/mpi-operator/v2

go 1.15

require (
	github.com/emicklei/go-restful v2.9.6+incompatible // indirect
	github.com/go-openapi/spec v0.19.3
	github.com/imdario/mergo v0.3.7 // indirect
	github.com/kubeflow/common v0.3.4
	github.com/prometheus/client_golang v1.7.1
	k8s.io/api v0.19.9
	k8s.io/apimachinery v0.19.9
	k8s.io/apiserver v0.19.9
	k8s.io/client-go v0.19.9
	k8s.io/klog v1.0.0
	k8s.io/kube-openapi v0.0.0-20200805222855-6aeccd4b50c6
	k8s.io/sample-controller v0.19.9
	volcano.sh/apis v1.2.0-k8s1.19.6
)

replace k8s.io/code-generator => k8s.io/code-generator v0.19.9
