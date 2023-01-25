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
	"fmt"
	"net/http"

	"k8s.io/klog"

	"github.com/kubeflow/mpi-operator/cmd/mpi-operator/app"
	"github.com/kubeflow/mpi-operator/cmd/mpi-operator/app/options"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

func startMonitoring(monitoringPort int) {
	if monitoringPort != 0 {
		go func() {
			klog.Infof("Setting up client for monitoring on port: %d", monitoringPort)
			http.Handle("/metrics", promhttp.Handler())
			err := http.ListenAndServe(fmt.Sprintf(":%d", monitoringPort), nil)
			if err != nil {
				klog.Error("Monitoring endpoint setup failure.", err)
			}
		}()
	}
}

func main() {
	klog.InitFlags(nil)
	s := options.NewServerOption()
	s.AddFlags(flag.CommandLine)

	flag.Parse()

	startMonitoring(s.MonitoringPort)

	if err := app.Run(s); err != nil {
		klog.Fatalf("%v\n", err)
	}
}
