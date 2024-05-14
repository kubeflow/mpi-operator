package options

import (
	mpijobclientset "github.com/kubeflow/mpi-operator/pkg/client/clientset/versioned"
	informers "github.com/kubeflow/mpi-operator/pkg/client/informers/externalversions"
	kubeinformers "k8s.io/client-go/informers"
	kubeclientset "k8s.io/client-go/kubernetes"
	schedclientset "sigs.k8s.io/scheduler-plugins/pkg/generated/clientset/versioned"
	schedinformers "sigs.k8s.io/scheduler-plugins/pkg/generated/informers/externalversions"
	volcanoclient "volcano.sh/apis/pkg/client/clientset/versioned"
	volcanoinformers "volcano.sh/apis/pkg/client/informers/externalversions"
)

type NamespaceParserFunc func(namespace string, kubeClient kubeclientset.Interface) ([]string, error)

type NamespaceOptions struct {
	Namespaces NamespaceParserFunc
}

func DefaultNamespaceParser(namespace string, kubeClient kubeclientset.Interface) ([]string, error) {
	return []string{namespace}, nil
}

type KubeInformerFunc func(namespaces []string, kubeClient kubeclientset.Interface) kubeinformers.SharedInformerFactory
type MpiJobInformerFunc func(namespaces []string, mpiJobClient mpijobclientset.Interface) informers.SharedInformerFactory
type VolcanoInformerFunc func(namespaces []string, volcanoClient volcanoclient.Interface) volcanoinformers.SharedInformerFactory
type SchedulerPluginsInformerFunc func(namespaces []string, schedClient schedclientset.Interface) schedinformers.SharedInformerFactory

type InformerOptions struct {
	KubeInformer             KubeInformerFunc
	MpiJobInformer           MpiJobInformerFunc
	VolcanoInformer          VolcanoInformerFunc
	SchedulerPluginsInformer SchedulerPluginsInformerFunc
}
type AdditionalOptions struct {
	NamespaceOptions
	InformerOptions
}
