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
	"net/http"
	"os"
	"time"

	kubeflowScheme "github.com/kubeflow/mpi-operator/pkg/client/clientset/versioned/scheme"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/uuid"
	"k8s.io/apiserver/pkg/server/healthz"
	kubeinformers "k8s.io/client-go/informers"
	kubeclientset "k8s.io/client-go/kubernetes"
	clientgokubescheme "k8s.io/client-go/kubernetes/scheme"
	v1core "k8s.io/client-go/kubernetes/typed/core/v1"
	restclientset "k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
	election "k8s.io/client-go/tools/leaderelection"
	"k8s.io/client-go/tools/leaderelection/resourcelock"
	"k8s.io/client-go/tools/record"
	"k8s.io/klog"
	"k8s.io/sample-controller/pkg/signals"
	volcanoclient "volcano.sh/apis/pkg/client/clientset/versioned"
	volcanoinformers "volcano.sh/apis/pkg/client/informers/externalversions"
	podgroupsinformer "volcano.sh/apis/pkg/client/informers/externalversions/scheduling/v1beta1"

	"github.com/kubeflow/mpi-operator/cmd/mpi-operator.v1/app/options"
	mpijobclientset "github.com/kubeflow/mpi-operator/pkg/client/clientset/versioned"
	informers "github.com/kubeflow/mpi-operator/pkg/client/informers/externalversions"
	controllersv1 "github.com/kubeflow/mpi-operator/pkg/controllers/v1"
	version "github.com/kubeflow/mpi-operator/pkg/version"
)

const (
	apiVersion                   = "v1"
	RecommendedKubeConfigPathEnv = "KUBECONFIG"
	controllerName               = "mpi-operator"
)

var (
	// leader election config
	leaseDuration = 15 * time.Second
	renewDuration = 5 * time.Second
	retryPeriod   = 3 * time.Second
	// leader election health check
	healthCheckPort = 8080
	// This is the timeout that determines the time beyond the lease expiry to be
	// allowed for timeout. Checks within the timeout period after the lease
	// expires will still return healthy.
	leaderHealthzAdaptorTimeout = time.Second * 20
)

var (
	isLeader = promauto.NewGauge(prometheus.GaugeOpts{
		Name: "mpi_operator_is_leader",
		Help: "Is this client the leader of this mpi-operator client set?",
	})
)

func Run(opt *options.ServerOption) error {
	// Check if the -version flag was passed and, if so, print the version and exit.
	if opt.PrintVersion {
		version.PrintVersionAndExit(apiVersion)
	}

	namespace := opt.Namespace
	if namespace == corev1.NamespaceAll {
		klog.Info("Using cluster scoped operator")
	} else {
		klog.Infof("Scoping operator to namespace %s", namespace)
	}

	// To help debugging, immediately log version.
	klog.Infof("%+v", version.Info(apiVersion))

	// To help debugging, immediately log opts.
	klog.Infof("Server options: %+v", opt)

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
		klog.Fatalf("Error building kubeConfig: %s", err.Error())
	}

	cfg.QPS = float32(opt.QPS)
	cfg.Burst = opt.Burst

	// Create clients.
	kubeClient, leaderElectionClientSet, mpiJobClientSet, volcanoClientSet, err := createClientSets(cfg)
	if err != nil {
		return err
	}
	if !checkCRDExists(mpiJobClientSet, namespace) {
		klog.Info("CRD doesn't exist. Exiting")
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
		var volcanoInformerFactory volcanoinformers.SharedInformerFactory
		if namespace == metav1.NamespaceAll {
			kubeInformerFactory = kubeinformers.NewSharedInformerFactory(kubeClient, 0)
			kubeflowInformerFactory = informers.NewSharedInformerFactory(mpiJobClientSet, 0)
			volcanoInformerFactory = volcanoinformers.NewSharedInformerFactory(volcanoClientSet, 0)
		} else {
			kubeInformerFactory = kubeinformers.NewSharedInformerFactoryWithOptions(kubeClient, 0, kubeinformers.WithNamespace(namespace))
			kubeflowInformerFactory = informers.NewSharedInformerFactoryWithOptions(mpiJobClientSet, 0, informers.WithNamespace(namespace))
			volcanoInformerFactory = volcanoinformers.NewSharedInformerFactoryWithOptions(volcanoClientSet, 0, volcanoinformers.WithNamespace(namespace))
		}

		var podgroupsInformer podgroupsinformer.PodGroupInformer
		if opt.GangSchedulingName != "" {
			podgroupsInformer = volcanoInformerFactory.Scheduling().V1beta1().PodGroups()
		}
		controller := controllersv1.NewMPIJobController(
			kubeClient,
			mpiJobClientSet,
			volcanoClientSet,
			kubeInformerFactory.Core().V1().ConfigMaps(),
			kubeInformerFactory.Core().V1().ServiceAccounts(),
			kubeInformerFactory.Rbac().V1().Roles(),
			kubeInformerFactory.Rbac().V1().RoleBindings(),
			kubeInformerFactory.Core().V1().Pods(),
			podgroupsInformer,
			kubeflowInformerFactory.Kubeflow().V1().MPIJobs(),
			opt.KubectlDeliveryImage,
			opt.GangSchedulingName)

		go kubeInformerFactory.Start(ctx.Done())
		go kubeflowInformerFactory.Start(ctx.Done())
		if opt.GangSchedulingName != "" {
			go volcanoInformerFactory.Start(ctx.Done())
		}

		// Set leader election start function.
		isLeader.Set(1)
		if err = controller.Run(opt.Threadiness, stopCh); err != nil {
			klog.Fatalf("Error running controller: %s", err.Error())
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
	eventBroadcaster.StartLogging(klog.Infof)
	eventBroadcaster.StartRecordingToSink(&v1core.EventSinkImpl{Interface: kubeClient.CoreV1().Events("")})
	recorder := eventBroadcaster.NewRecorder(clientgokubescheme.Scheme, corev1.EventSource{Component: controllerName})

	var electionChecker *election.HealthzAdaptor = election.NewLeaderHealthzAdaptor(leaderHealthzAdaptorTimeout)

	mux := http.NewServeMux()
	healthz.InstallPathHandler(mux, "/healthz", electionChecker)

	server := &http.Server{
		Addr:    fmt.Sprintf(":%d", healthCheckPort),
		Handler: mux,
	}

	go func() {
		klog.Infof("Start listening to %d for health check", healthCheckPort)

		if err := server.ListenAndServe(); err != nil {
			klog.Fatalf("Error starting server for health check: %v", err)
		}
	}()

	rl := &resourcelock.EndpointsLock{
		EndpointsMeta: metav1.ObjectMeta{
			Namespace: opt.LockNamespace,
			Name:      controllerName,
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
				klog.Infof("Leading started")
				run(ctx)
			},
			OnStoppedLeading: func() {
				isLeader.Set(0)
				klog.Fatalf("Leader election stopped")
			},
			OnNewLeader: func(identity string) {
				if identity == id {
					return
				}
				klog.Infof("New leader has been elected: %s", identity)
			},
		},
		Name:     "mpi-operator",
		WatchDog: electionChecker,
	})

	return fmt.Errorf("finished without leader elect")
}

func createClientSets(config *restclientset.Config) (kubeclientset.Interface, kubeclientset.Interface, mpijobclientset.Interface, volcanoclient.Interface, error) {

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

	volcanoClientSet, err := volcanoclient.NewForConfig(restclientset.AddUserAgent(config, "volcano"))
	if err != nil {
		return nil, nil, nil, nil, err
	}

	return kubeClientSet, leaderElectionClientSet, mpiJobClientSet, volcanoClientSet, nil
}

func checkCRDExists(clientset mpijobclientset.Interface, namespace string) bool {
	_, err := clientset.KubeflowV1().MPIJobs(namespace).List(context.TODO(), metav1.ListOptions{})

	if err != nil {
		klog.Error(err)
		if _, ok := err.(*errors.StatusError); ok {
			if errors.IsNotFound(err) {
				return false
			}
		}
	}
	return true
}
