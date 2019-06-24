// Copyright 2018 The Kubeflow Authors.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package main

import (
	"flag"

	"github.com/golang/glog"
	controllersv1alpha1 "github.com/kubeflow/mpi-operator/pkg/controllers/v1alpha1"
	kubeinformers "k8s.io/client-go/informers"
	"k8s.io/client-go/kubernetes"
	clientgokubescheme "k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/sample-controller/pkg/signals"

	clientset "github.com/kubeflow/mpi-operator/pkg/client/clientset/versioned"
	kubeflowScheme "github.com/kubeflow/mpi-operator/pkg/client/clientset/versioned/scheme"
	informers "github.com/kubeflow/mpi-operator/pkg/client/informers/externalversions"
	policyinformers "k8s.io/client-go/informers/policy/v1beta1"
)

var (
	masterURL              string
	kubeConfig             string
	gpusPerNode            int
	processingUnitsPerNode int
	processingResourceType string
	kubectlDeliveryImage   string
	namespace              string
	enableGangScheduling   bool
)

func main() {
	flag.Parse()

	// set up signals so we handle the first shutdown signal gracefully
	stopCh := signals.SetupSignalHandler()

	cfg, err := clientcmd.BuildConfigFromFlags(masterURL, kubeConfig)
	if err != nil {
		glog.Fatalf("Error building kubeConfig: %s", err.Error())
	}

	kubeClient, err := kubernetes.NewForConfig(cfg)
	if err != nil {
		glog.Fatalf("Error building kubernetes clientset: %s", err.Error())
	}

	kubeflowClient, err := clientset.NewForConfig(cfg)
	if err != nil {
		glog.Fatalf("Error building kubeflow clientset: %s", err.Error())
	}

	// Add mpi-job-controller types to the default Kubernetes Scheme so Events
	// can be logged for mpi-job-controller types.
	err = kubeflowScheme.AddToScheme(clientgokubescheme.Scheme)
	if err != nil {
		glog.Fatalf("CoreV1 Add Scheme failed: %v", err)
	}

	var kubeInformerFactory kubeinformers.SharedInformerFactory
	var kubeflowInformerFactory informers.SharedInformerFactory
	if namespace == "" {
		kubeInformerFactory = kubeinformers.NewSharedInformerFactory(kubeClient, 0)
		kubeflowInformerFactory = informers.NewSharedInformerFactory(kubeflowClient, 0)
	} else {
		kubeInformerFactory = kubeinformers.NewSharedInformerFactoryWithOptions(kubeClient, 0, kubeinformers.WithNamespace(namespace), nil)
		kubeflowInformerFactory = informers.NewSharedInformerFactoryWithOptions(kubeflowClient, 0, informers.WithNamespace(namespace), nil)
	}

	var pdbInformer policyinformers.PodDisruptionBudgetInformer
	if enableGangScheduling {
		pdbInformer = kubeInformerFactory.Policy().V1beta1().PodDisruptionBudgets()
	}
	controller := controllersv1alpha1.NewMPIJobController(
		kubeClient,
		kubeflowClient,
		kubeInformerFactory.Core().V1().ConfigMaps(),
		kubeInformerFactory.Core().V1().ServiceAccounts(),
		kubeInformerFactory.Rbac().V1().Roles(),
		kubeInformerFactory.Rbac().V1().RoleBindings(),
		kubeInformerFactory.Apps().V1().StatefulSets(),
		kubeInformerFactory.Batch().V1().Jobs(),
		pdbInformer,
		kubeflowInformerFactory.Kubeflow().V1alpha1().MPIJobs(),
		gpusPerNode,
		processingUnitsPerNode,
		processingResourceType,
		kubectlDeliveryImage,
		enableGangScheduling)

	go kubeInformerFactory.Start(stopCh)
	go kubeflowInformerFactory.Start(stopCh)

	if err = controller.Run(2, stopCh); err != nil {
		glog.Fatalf("Error running controller: %s", err.Error())
	}
}

func init() {
	flag.StringVar(&kubeConfig, "kubeConfig", "", "Path to a kubeConfig. Only required if out-of-cluster.")
	flag.StringVar(&masterURL, "master", "", "The address of the Kubernetes API server. Overrides any value in kubeConfig. Only required if out-of-cluster.")
	flag.IntVar(
		&gpusPerNode,
		"gpus-per-node",
		1,
		"(Deprecated. This will be overwritten by MPIJobSpec) The maximum number of GPUs available per node. Note that this will be ignored if the GPU resources are explicitly specified in the MPIJob pod spec.")
	flag.StringVar(&kubectlDeliveryImage, "kubectl-delivery-image", "", "The container image used to deliver the kubectl binary.")
	flag.StringVar(&namespace, "namespace", "", "The namespace used to obtain the listers.")
	flag.IntVar(
		&processingUnitsPerNode,
		"processing-units-per-node",
		1,
		"(Deprecated. This will be overwritten by MPIJobSpec) The maximum number of processing units available per node. Note that this will be ignored if the processing resources are explicitly specified in the MPIJob pod spec.")
	flag.StringVar(&processingResourceType, "processing-resource-type", "nvidia.com/gpu", "(Deprecated. This will be overwritten by MPIJobSpec) The compute resource name, e.g. 'nvidia.com/gpu' or 'cpu'.")
	flag.BoolVar(&enableGangScheduling, "enable-gang-scheduling", false, "Whether to enable gang scheduling by kube-batch.")
}
