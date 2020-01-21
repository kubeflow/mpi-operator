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
	"context"
	"fmt"
	"os"
	"time"

	"github.com/golang/glog"
	kubeflowScheme "github.com/kubeflow/mpi-operator/pkg/client/clientset/versioned/scheme"
	kubebatchclient "github.com/kubernetes-sigs/kube-batch/pkg/client/clientset/versioned"
	kubebatchinformers "github.com/kubernetes-sigs/kube-batch/pkg/client/informers/externalversions"
	podgroupsinformer "github.com/kubernetes-sigs/kube-batch/pkg/client/informers/externalversions/scheduling/v1alpha1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/uuid"
	kubeinformers "k8s.io/client-go/informers"
	kubeclientset "k8s.io/client-go/kubernetes"
	clientgokubescheme "k8s.io/client-go/kubernetes/scheme"
	v1core "k8s.io/client-go/kubernetes/typed/core/v1"
	restclientset "k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
	election "k8s.io/client-go/tools/leaderelection"
	"k8s.io/client-go/tools/leaderelection/resourcelock"
	"k8s.io/client-go/tools/record"
	"k8s.io/sample-controller/pkg/signals"

	"github.com/kubeflow/mpi-operator/cmd/mpi-operator.v1alpha2/app/options"
	"github.com/kubeflow/mpi-operator/pkg/apis/kubeflow/v1alpha2"
	mpijobclientset "github.com/kubeflow/mpi-operator/pkg/client/clientset/versioned"
	informers "github.com/kubeflow/mpi-operator/pkg/client/informers/externalversions"
	controllersv1alpha2 "github.com/kubeflow/mpi-operator/pkg/controllers/v1alpha2"
	"github.com/kubeflow/mpi-operator/pkg/version"
)

const (
	apiVersion                   = "v1alpha2"
	RecommendedKubeConfigPathEnv = "KUBECONFIG"
)

var (
	// leader election config
	leaseDuration = 15 * time.Second
	renewDuration = 5 * time.Second
	retryPeriod   = 3 * time.Second
)

func Run(opt *options.ServerOption) error {
	// Check if the -version flag was passed and, if so, print the version and exit.
	if opt.PrintVersion {
		version.PrintVersionAndExit(apiVersion)
	}

	namespace := os.Getenv(v1alpha2.EnvKubeflowNamespace)
	if len(namespace) == 0 {
		glog.Infof("%s not set, use default namespace", v1alpha2.EnvKubeflowNamespace)
		namespace = metav1.NamespaceDefault
	}

	if opt.Namespace == corev1.NamespaceAll {
		glog.Info("Using cluster scoped operator")
	} else {
		glog.Infof("Scoping operator to namespace %s", opt.Namespace)
	}

	// To help debugging, immediately log version.
	glog.Infof("%+v", version.Info(apiVersion))

	// To help debugging, immediately log opts.
	glog.Infof("Server options: %+v", opt)

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

	// Create clients.
	kubeClient, leaderElectionClientSet, mpiJobClientSet, kubeBatchClientSet, err := createClientSets(cfg)
	if err != nil {
		return err
	}
	if !checkCRDExists(mpiJobClientSet, opt.Namespace) {
		glog.Info("CRD doesn't exist. Exiting")
		os.Exit(1)
	}

	// Add mpi-job-controller types to the default Kubernetes Scheme so Events
	// can be logged for mpi-job-controller types.
	err = kubeflowScheme.AddToScheme(clientgokubescheme.Scheme)
	if err != nil {
		return fmt.Errorf("CoreV1 Add Scheme failed: %v", err)
	}

	// Set leader election start function.
	run := func(ctx context.Context) {
		var kubeInformerFactory kubeinformers.SharedInformerFactory
		var kubeflowInformerFactory informers.SharedInformerFactory
		var kubebatchInformerFactory kubebatchinformers.SharedInformerFactory
		if opt.Namespace == "" {
			kubeInformerFactory = kubeinformers.NewSharedInformerFactory(kubeClient, 0)
			kubeflowInformerFactory = informers.NewSharedInformerFactory(mpiJobClientSet, 0)
			kubebatchInformerFactory = kubebatchinformers.NewSharedInformerFactory(kubeBatchClientSet, 0)
		} else {
			kubeInformerFactory = kubeinformers.NewSharedInformerFactoryWithOptions(kubeClient, 0, kubeinformers.WithNamespace(opt.Namespace))
			kubeflowInformerFactory = informers.NewSharedInformerFactoryWithOptions(mpiJobClientSet, 0, informers.WithNamespace(opt.Namespace))
			kubebatchInformerFactory = kubebatchinformers.NewSharedInformerFactoryWithOptions(kubeBatchClientSet, 0, kubebatchinformers.WithNamespace(opt.Namespace))
		}

		var podgroupsInformer podgroupsinformer.PodGroupInformer
		if opt.EnableGangScheduling {
			podgroupsInformer = kubebatchInformerFactory.Scheduling().V1alpha1().PodGroups()
		}
		controller := controllersv1alpha2.NewMPIJobController(
			kubeClient,
			mpiJobClientSet,
			kubeBatchClientSet,
			kubeInformerFactory.Core().V1().ConfigMaps(),
			kubeInformerFactory.Core().V1().ServiceAccounts(),
			kubeInformerFactory.Rbac().V1().Roles(),
			kubeInformerFactory.Rbac().V1().RoleBindings(),
			kubeInformerFactory.Apps().V1().StatefulSets(),
			kubeInformerFactory.Batch().V1().Jobs(),
			podgroupsInformer,
			kubeflowInformerFactory.Kubeflow().V1alpha2().MPIJobs(),
			opt.KubectlDeliveryImage,
			opt.EnableGangScheduling)

		go kubeInformerFactory.Start(ctx.Done())
		go kubeflowInformerFactory.Start(ctx.Done())
		if opt.EnableGangScheduling {
			go kubebatchInformerFactory.Start(ctx.Done())
		}

		if err = controller.Run(opt.Threadiness, stopCh); err != nil {
			glog.Fatalf("Error running controller: %s", err.Error())
		}
	}

	id, err := os.Hostname()
	if err != nil {
		return fmt.Errorf("failed to get hostname: %v", err)
	}
	// add a uniquifier so that two processes on the same host don't accidentally both become active
	id = id + "_" + string(uuid.NewUUID())

	// Prepare event clients.
	eventBroadcaster := record.NewBroadcaster()
	eventBroadcaster.StartLogging(glog.Infof)
	eventBroadcaster.StartRecordingToSink(&v1core.EventSinkImpl{Interface: kubeClient.CoreV1().Events("")})
	recorder := eventBroadcaster.NewRecorder(clientgokubescheme.Scheme, corev1.EventSource{Component: "mpi-operator"})

	rl := &resourcelock.EndpointsLock{
		EndpointsMeta: metav1.ObjectMeta{
			Namespace: namespace,
			Name:      "mpi-operator",
		},
		Client: leaderElectionClientSet.CoreV1(),
		LockConfig: resourcelock.ResourceLockConfig{
			Identity:      id,
			EventRecorder: recorder,
		},
	}

	ctx, cancel := context.WithCancel(context.TODO())
	defer cancel()

	go func() {
		select {
		case <-stopCh:
			cancel()
		case <-ctx.Done():
		}
	}()

	// Start leader election.
	election.RunOrDie(ctx, election.LeaderElectionConfig{
		Lock:          rl,
		LeaseDuration: leaseDuration,
		RenewDeadline: renewDuration,
		RetryPeriod:   retryPeriod,
		Callbacks: election.LeaderCallbacks{
			OnStartedLeading: func(ctx context.Context) {
				glog.Infof("Leading started")
				run(ctx)
			},
			OnStoppedLeading: func() {
				glog.Fatalf("Leader election stopped")
			},
			OnNewLeader: func(identity string) {
				if identity == id {
					return
				}
				glog.Infof("New leader has been elected: %s", identity)
			},
		},
		Name: "mpi-operator",
	})

	return fmt.Errorf("finished without leader elect")
}

func createClientSets(config *restclientset.Config) (kubeclientset.Interface, kubeclientset.Interface, mpijobclientset.Interface, kubebatchclient.Interface, error) {

	kubeClientSet, err := kubeclientset.NewForConfig(restclientset.AddUserAgent(config, "mpi-operator"))
	if err != nil {
		return nil, nil, nil, nil, err
	}

	leaderElectionClientSet, err := kubeclientset.NewForConfig(restclientset.AddUserAgent(config, "leader-election"))
	if err != nil {
		return nil, nil, nil, nil, err
	}

	mpiJobClientSet, err := mpijobclientset.NewForConfig(config)
	if err != nil {
		return nil, nil, nil, nil, err
	}

	kubeBatchClientSet, err := kubebatchclient.NewForConfig(restclientset.AddUserAgent(config, "kube-batch"))
	if err != nil {
		return nil, nil, nil, nil, err
	}

	return kubeClientSet, leaderElectionClientSet, mpiJobClientSet, kubeBatchClientSet, nil
}

func checkCRDExists(clientset mpijobclientset.Interface, namespace string) bool {
	_, err := clientset.KubeflowV1alpha2().MPIJobs(namespace).List(metav1.ListOptions{})

	if err != nil {
		glog.Error(err)
		if _, ok := err.(*errors.StatusError); ok {
			if errors.IsNotFound(err) {
				return false
			}
		}
	}
	return true
}
