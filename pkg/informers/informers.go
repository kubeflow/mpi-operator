package informers

import (
	mpijobclientset "github.com/kubeflow/mpi-operator/pkg/client/clientset/versioned"
	informers "github.com/kubeflow/mpi-operator/pkg/client/informers/externalversions"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	kubeinformers "k8s.io/client-go/informers"
	kubeclientset "k8s.io/client-go/kubernetes"
	schedclientset "sigs.k8s.io/scheduler-plugins/pkg/generated/clientset/versioned"
	schedinformers "sigs.k8s.io/scheduler-plugins/pkg/generated/informers/externalversions"
	volcanoclient "volcano.sh/apis/pkg/client/clientset/versioned"
	volcanoinformers "volcano.sh/apis/pkg/client/informers/externalversions"
)

func DefaultKubeInformer(namespaces []string, kubeClient kubeclientset.Interface) kubeinformers.SharedInformerFactory {
	var kubeInformerFactoryOpts []kubeinformers.SharedInformerOption
	if namespaces[0] != metav1.NamespaceAll {
		kubeInformerFactoryOpts = append(kubeInformerFactoryOpts, kubeinformers.WithNamespace(namespaces[0]))
	}

	return kubeinformers.NewSharedInformerFactoryWithOptions(kubeClient, 0, kubeInformerFactoryOpts...)
}

func DefaultMpiJobInformer(namespaces []string, mpiJobClient mpijobclientset.Interface) informers.SharedInformerFactory {
	var kubeflowInformerFactoryOpts []informers.SharedInformerOption
	if namespaces[0] != metav1.NamespaceAll {
		kubeflowInformerFactoryOpts = append(kubeflowInformerFactoryOpts, informers.WithNamespace(namespaces[0]))
	}

	return informers.NewSharedInformerFactoryWithOptions(mpiJobClient, 0, kubeflowInformerFactoryOpts...)
}

func DefaultVolcanoInformer(namespaces []string, volcanoClient volcanoclient.Interface) volcanoinformers.SharedInformerFactory {
	var informerFactoryOpts []volcanoinformers.SharedInformerOption
	if namespaces[0] != metav1.NamespaceAll {
		informerFactoryOpts = append(informerFactoryOpts, volcanoinformers.WithNamespace(namespaces[0]))
	}
	return volcanoinformers.NewSharedInformerFactoryWithOptions(volcanoClient, 0, informerFactoryOpts...)
}

func DefaultSchedulerPluginsInformer(namespaces []string, schedClient schedclientset.Interface) schedinformers.SharedInformerFactory {
	var informerFactoryOpts []schedinformers.SharedInformerOption
	if namespaces[0] != metav1.NamespaceAll {
		informerFactoryOpts = append(informerFactoryOpts, schedinformers.WithNamespace(namespaces[0]))
	}
	return schedinformers.NewSharedInformerFactoryWithOptions(schedClient, 0, informerFactoryOpts...)
}
