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

package v2beta1

import common "github.com/kubeflow/common/pkg/apis/common/v1"

const (
	// EnvKubeflowNamespace is ENV for kubeflow namespace specified by user.
	EnvKubeflowNamespace = "KUBEFLOW_NAMESPACE"
	// DefaultRestartPolicy is default RestartPolicy for ReplicaSpec.
	DefaultRestartPolicy = common.RestartPolicyNever
	// DefaultLauncherRestartPolicy is default RestartPolicy for Launcher Job.
	DefaultLauncherRestartPolicy = common.RestartPolicyOnFailure
	// OperatorName is the name of the operator used as value to the label common.OperatorLabelName
	OperatorName = "mpi-operator"
)
