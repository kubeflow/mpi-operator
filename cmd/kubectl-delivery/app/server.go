// Copyright 2020 The Kubeflow Authors
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
	"bufio"
	"context"
	"io"
	"os"
	"strings"

	"github.com/golang/glog"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	kubeinformers "k8s.io/client-go/informers"
	kubeclientset "k8s.io/client-go/kubernetes"
	restclientset "k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/sample-controller/pkg/signals"

	"github.com/kubeflow/mpi-operator/cmd/kubectl-delivery/app/options"
	"github.com/kubeflow/mpi-operator/pkg/controllers/kubectl_delivery"
	"github.com/kubeflow/mpi-operator/pkg/version"
)

const (
	cmdVersion                   = "v1"
	RecommendedKubeConfigPathEnv = "KUBECONFIG"
	nsEnvironmentName            = "NAMESPACE"
	filename                     = "/etc/mpi/hostfile"
)

func Run(opt *options.ServerOption) error {
	// Check if the -version flag was passed and, if so, print the version and exit.
	if opt.PrintVersion {
		version.PrintVersionAndExit(cmdVersion)
	}

	namespace := os.Getenv(nsEnvironmentName)
	if len(namespace) == 0 {
		glog.Infof("%s not set, use default namespace", nsEnvironmentName)
		namespace = metav1.NamespaceDefault
	}
	if opt.Namespace != "" {
		namespace = opt.Namespace
	}

	if namespace == corev1.NamespaceAll {
		glog.Info("Using cluster scoped operator")
	} else {
		glog.Infof("Scoping operator to namespace %s", namespace)
	}

	// To help debugging, immediately log version.
	glog.Infof("%+v", version.Info(cmdVersion))

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
		glog.Fatalf("Error building kubeConfig: %v", err)
	}
	kubeClient, err := kubeclientset.NewForConfig(restclientset.AddUserAgent(cfg, "mpi-operator-kubectl-delivery"))
	if err != nil {
		glog.Fatalf("Error building kubeClient: %v", err)
	}

	kubeInformerFactory := kubeinformers.NewSharedInformerFactoryWithOptions(kubeClient, 0, kubeinformers.WithNamespace(namespace))

	fp, err := os.Open(filename)
	if err != nil {
		glog.Fatalf("Error open file[%s]: %v", filename, err)
	}
	defer fp.Close()
	bufReader := bufio.NewReader(fp)
	pods := []string{}

	for {
		line, _, err := bufReader.ReadLine()
		if err != nil {
			if err == io.EOF {
				break
			}
			continue
		}
		lines := strings.SplitN(string(line), " ", 2)
		pods = append(pods, lines[0])
	}
	controller := kubectl_delivery.NewKubectlDeliveryController(
		namespace,
		kubeClient,
		kubeInformerFactory.Core().V1().Pods(),
		pods)

	ctx, cancel := context.WithCancel(context.TODO())
	defer cancel()

	go kubeInformerFactory.Start(ctx.Done())

	if err = controller.Run(opt.Threadiness, stopCh); err != nil {
		glog.Fatalf("Error running controller: %s", err.Error())
	}

	return nil
}
