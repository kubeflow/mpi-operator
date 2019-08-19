// Copyright 2019 The Kubeflow Authors.
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

package v1alpha2

import (
	common "github.com/kubeflow/common/job_controller/api/v1"
	"k8s.io/apimachinery/pkg/runtime"
)

// Int32 is a helper routine that allocates a new int32 value
// to store v and returns a pointer to it.
func Int32(v int32) *int32 {
	return &v
}

func addDefaultingFuncs(scheme *runtime.Scheme) error {
	return RegisterDefaults(scheme)
}

// setDefaults_TypeLauncher sets the default value to launcher.
func setDefaults_TypeLauncher(spec *common.ReplicaSpec) {
	// Only a `RestartPolicy` equal to `Never` or `OnFailure` is allowed for `Job`.
	if spec.RestartPolicy != common.RestartPolicyNever {
		spec.RestartPolicy = common.RestartPolicyOnFailure
	}
}

// setDefaults_TypeWorker sets the default value to worker.
func setDefaults_TypeWorker(spec *common.ReplicaSpec) {
}

func SetDefaults_MPIJob(mpiJob *MPIJob) {
	// set default BackoffLimit
	if mpiJob.Spec.BackoffLimit == nil {
		mpiJob.Spec.BackoffLimit = new(int32)
		*mpiJob.Spec.BackoffLimit = 6
	}

	// Set default cleanpod policy to None.
	if mpiJob.Spec.CleanPodPolicy == nil {
		none := common.CleanPodPolicyNone
		mpiJob.Spec.CleanPodPolicy = &none
	}

	// set default to Launcher
	setDefaults_TypeLauncher(mpiJob.Spec.MPIReplicaSpecs[MPIReplicaTypeLauncher])

	// set default to Worker
	setDefaults_TypeWorker(mpiJob.Spec.MPIReplicaSpecs[MPIReplicaTypeWorker])

}
