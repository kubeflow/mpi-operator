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

package v2beta1

import (
	common "github.com/kubeflow/common/pkg/apis/common/v1"
	"k8s.io/apimachinery/pkg/runtime"
)

func addDefaultingFuncs(scheme *runtime.Scheme) error {
	return RegisterDefaults(scheme)
}

// setDefaultsTypeLauncher sets the default value to launcher.
func setDefaultsTypeLauncher(spec *common.ReplicaSpec) {
	if spec == nil {
		return
	}
	if spec.RestartPolicy == "" {
		spec.RestartPolicy = DefaultRestartPolicy
	}
	if spec.Replicas == nil {
		spec.Replicas = newInt32(1)
	}
}

// setDefaultsTypeWorker sets the default value to worker.
func setDefaultsTypeWorker(spec *common.ReplicaSpec) {
	if spec == nil {
		return
	}
	if spec.RestartPolicy == "" {
		spec.RestartPolicy = DefaultRestartPolicy
	}
	if spec.Replicas == nil {
		spec.Replicas = newInt32(0)
	}
}

func SetDefaults_MPIJob(mpiJob *MPIJob) {
	if mpiJob.Spec.CleanPodPolicy == nil {
		mpiJob.Spec.CleanPodPolicy = newCleanPodPolicy(common.CleanPodPolicyNone)
	}
	if mpiJob.Spec.SlotsPerWorker == nil {
		mpiJob.Spec.SlotsPerWorker = newInt32(1)
	}

	// set default to Launcher
	setDefaultsTypeLauncher(mpiJob.Spec.MPIReplicaSpecs[MPIReplicaTypeLauncher])

	// set default to Worker
	setDefaultsTypeWorker(mpiJob.Spec.MPIReplicaSpecs[MPIReplicaTypeWorker])
}

func newInt32(v int32) *int32 {
	return &v
}

func newCleanPodPolicy(policy common.CleanPodPolicy) *common.CleanPodPolicy {
	return &policy
}
