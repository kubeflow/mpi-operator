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
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"net/http"
	"strconv"

	"github.com/golang/glog"

	"github.com/kubeflow/mpi-operator/cmd/mpi-operator.v1alpha2/app"
	"github.com/kubeflow/mpi-operator/cmd/mpi-operator.v1alpha2/app/options"
)

func startMonitoring(monitoringPort int) {
	if monitoringPort != 0 {
		go func() {
			glog.Infof("Setting up client for monitoring on port: %s", strconv.Itoa(monitoringPort))
			http.Handle("/metrics", promhttp.Handler())
			err := http.ListenAndServe(fmt.Sprintf(":%s", strconv.Itoa(monitoringPort)), nil)
			if err != nil {
				glog.Error("Monitoring endpoint setup failure.", err)
			}
		}()
	}
}

func main() {
	s := options.NewServerOption()
	s.AddFlags(flag.CommandLine)

	flag.Parse()

	startMonitoring(s.MonitoringPort)

	if err := app.Run(s); err != nil {
		glog.Fatalf("%v\n", err)
	}
}
