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
		spec.RestartPolicy = DefaultLauncherRestartPolicy
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

func setDefaultsRunPolicy(policy *common.RunPolicy) {
	if policy.CleanPodPolicy == nil {
		policy.CleanPodPolicy = newCleanPodPolicy(common.CleanPodPolicyNone)
	}
	// The remaining fields are passed as-is to the k8s Job API, which does its
	// own defaulting.
}

func SetDefaults_MPIJob(mpiJob *MPIJob) {
	setDefaultsRunPolicy(&mpiJob.Spec.RunPolicy)
	if mpiJob.Spec.SlotsPerWorker == nil {
		mpiJob.Spec.SlotsPerWorker = newInt32(1)
	}
	if mpiJob.Spec.SSHAuthMountPath == "" {
		mpiJob.Spec.SSHAuthMountPath = "/root/.ssh"
	}
	if mpiJob.Spec.MPIImplementation == "" {
		mpiJob.Spec.MPIImplementation = MPIImplementationOpenMPI
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
