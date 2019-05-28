// Copyright 2019 The Kubeflow Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package app

import (
	"os"

	"github.com/golang/glog"
	controllersv1alpha2 "github.com/kubeflow/mpi-operator/pkg/controllers/v1alpha2"
	corev1 "k8s.io/api/core/v1"
	kubeinformers "k8s.io/client-go/informers"
	policyinformers "k8s.io/client-go/informers/policy/v1beta1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/sample-controller/pkg/signals"

	"github.com/kubeflow/mpi-operator/cmd/mpi-operator.v1alpha2/app/options"
	clientset "github.com/kubeflow/mpi-operator/pkg/client/clientset/versioned"
	informers "github.com/kubeflow/mpi-operator/pkg/client/informers/externalversions"
	"github.com/kubeflow/mpi-operator/pkg/version"
)

const (
	apiVersion                   = "v1alpha2"
	RecommendedKubeConfigPathEnv = "KUBECONFIG"
)

func Run(opt *options.ServerOption) error {
	// Check if the -version flag was passed and, if so, print the version and exit.
	if opt.PrintVersion {
		version.PrintVersionAndExit(apiVersion)
	}

	if opt.Namespace == corev1.NamespaceAll {
		glog.Info("Using cluster scoped operator")
	} else {
		glog.Infof("Scoping operator to namespace %s", opt.Namespace)
	}

	// To help debugging, immediately log version.
	glog.Infof("%+v", version.Info(apiVersion))

	// set up signals so we handle the first shutdown signal gracefully
	stopCh := signals.SetupSignalHandler()

	// Note: ENV KUBECONFIG will overwrite user defined Kubeconfig option.
	if len(os.Getenv(RecommendedKubeConfigPathEnv)) > 0 {
		// use the current context in kubeconfig
		// This is very useful for running locally.
		opt.Kubeconfig = os.Getenv(RecommendedKubeConfigPathEnv)
	}

	cfg, err := clientcmd.BuildConfigFromFlags(opt.MasterURL, opt.Kubeconfig)
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

	var kubeInformerFactory kubeinformers.SharedInformerFactory
	var kubeflowInformerFactory informers.SharedInformerFactory
	if opt.Namespace == "" {
		kubeInformerFactory = kubeinformers.NewSharedInformerFactory(kubeClient, 0)
		kubeflowInformerFactory = informers.NewSharedInformerFactory(kubeflowClient, 0)
	} else {
		kubeInformerFactory = kubeinformers.NewSharedInformerFactoryWithOptions(kubeClient, 0, kubeinformers.WithNamespace(opt.Namespace), nil)
		kubeflowInformerFactory = informers.NewSharedInformerFactoryWithOptions(kubeflowClient, 0, informers.WithNamespace(opt.Namespace), nil)
	}

	var pdbInformer policyinformers.PodDisruptionBudgetInformer
	if opt.EnableGangScheduling {
		pdbInformer = kubeInformerFactory.Policy().V1beta1().PodDisruptionBudgets()
	}
	controller := controllersv1alpha2.NewMPIJobController(
		kubeClient,
		kubeflowClient,
		kubeInformerFactory.Core().V1().ConfigMaps(),
		kubeInformerFactory.Core().V1().ServiceAccounts(),
		kubeInformerFactory.Rbac().V1().Roles(),
		kubeInformerFactory.Rbac().V1().RoleBindings(),
		kubeInformerFactory.Apps().V1().StatefulSets(),
		kubeInformerFactory.Batch().V1().Jobs(),
		pdbInformer,
		kubeflowInformerFactory.Kubeflow().V1alpha2().MPIJobs(),
		opt.KubectlDeliveryImage,
		opt.EnableGangScheduling)

	go kubeInformerFactory.Start(stopCh)
	go kubeflowInformerFactory.Start(stopCh)

	if err = controller.Run(opt.Threadiness, stopCh); err != nil {
		glog.Fatalf("Error running controller: %s", err.Error())
	}
	return nil
}
