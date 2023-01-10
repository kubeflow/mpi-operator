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

package options

import (
	"flag"
	"os"

	"github.com/kubeflow/mpi-operator/v2/pkg/apis/kubeflow/v2beta1"
)

// ServerOption is the main context object for the controller manager.
type ServerOption struct {
	Kubeconfig         string
	MasterURL          string
	Threadiness        int
	MonitoringPort     int
	PrintVersion       bool
	GangSchedulingName string
	Namespace          string
	LockNamespace      string
	QPS                int
	Burst              int
}

// NewServerOption creates a new CMServer with a default config.
func NewServerOption() *ServerOption {
	s := ServerOption{}
	return &s
}

// AddFlags adds flags for a specific CMServer to the specified FlagSet.
func (s *ServerOption) AddFlags(fs *flag.FlagSet) {
	fs.StringVar(&s.MasterURL, "master", "",
		`The url of the Kubernetes API server,
		 will overrides any value in kubeconfig, only required if out-of-cluster.`)

	fs.StringVar(&s.Kubeconfig, "kubeConfig", "",
		"Path to a kubeConfig. Only required if out-of-cluster.")

	fs.StringVar(&s.Namespace, "namespace", os.Getenv(v2beta1.EnvKubeflowNamespace),
		`The namespace to monitor mpijobs. If unset, it monitors all namespaces cluster-wide. 
                If set, it only monitors mpijobs in the given namespace.`)

	fs.IntVar(&s.Threadiness, "threadiness", 2,
		`How many threads to process the main logic`)

	fs.BoolVar(&s.PrintVersion, "version", false, "Show version and quit")

	fs.IntVar(&s.MonitoringPort, "monitoring-port", 0,
		`Endpoint port for displaying monitoring metrics. It can be set to "0" to disable the metrics serving.`)

	fs.StringVar(&s.GangSchedulingName, "gang-scheduling", "", "Set gang scheduler name if enable gang scheduling.")

	fs.StringVar(&s.LockNamespace, "lock-namespace", "mpi-operator", "Set locked namespace name while enabling leader election.")

	fs.IntVar(&s.QPS, "kube-api-qps", 5, "QPS indicates the maximum QPS to the master from this client.")
	fs.IntVar(&s.Burst, "kube-api-burst", 10, "Maximum burst for throttle.")
}
