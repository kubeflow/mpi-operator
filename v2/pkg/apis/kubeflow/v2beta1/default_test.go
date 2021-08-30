// Copyright 2021 The Kubeflow Authors.
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
	"testing"

	"github.com/google/go-cmp/cmp"
	common "github.com/kubeflow/common/pkg/apis/common/v1"
)

func TestSetDefaults_MPIJob(t *testing.T) {
	cases := map[string]struct {
		job  MPIJob
		want MPIJob
	}{
		"base defaults": {
			want: MPIJob{
				Spec: MPIJobSpec{
					SlotsPerWorker: newInt32(1),
					RunPolicy: common.RunPolicy{
						CleanPodPolicy: newCleanPodPolicy(common.CleanPodPolicyNone),
					},
					SSHAuthMountPath:  "/root/.ssh",
					MPIImplementation: MPIImplementationOpenMPI,
				},
			},
		},
		"base defaults overridden": {
			job: MPIJob{
				Spec: MPIJobSpec{
					SlotsPerWorker: newInt32(10),
					RunPolicy: common.RunPolicy{
						CleanPodPolicy:          newCleanPodPolicy(common.CleanPodPolicyRunning),
						TTLSecondsAfterFinished: newInt32(2),
						ActiveDeadlineSeconds:   newInt64(3),
						BackoffLimit:            newInt32(4),
					},
					SSHAuthMountPath:  "/home/mpiuser/.ssh",
					MPIImplementation: MPIImplementationIntel,
				},
			},
			want: MPIJob{
				Spec: MPIJobSpec{
					SlotsPerWorker: newInt32(10),
					RunPolicy: common.RunPolicy{
						CleanPodPolicy:          newCleanPodPolicy(common.CleanPodPolicyRunning),
						TTLSecondsAfterFinished: newInt32(2),
						ActiveDeadlineSeconds:   newInt64(3),
						BackoffLimit:            newInt32(4),
					},
					SSHAuthMountPath:  "/home/mpiuser/.ssh",
					MPIImplementation: MPIImplementationIntel,
				},
			},
		},
		"launcher defaults": {
			job: MPIJob{
				Spec: MPIJobSpec{
					MPIReplicaSpecs: map[MPIReplicaType]*common.ReplicaSpec{
						MPIReplicaTypeLauncher: {},
					},
				},
			},
			want: MPIJob{
				Spec: MPIJobSpec{
					SlotsPerWorker: newInt32(1),
					RunPolicy: common.RunPolicy{
						CleanPodPolicy: newCleanPodPolicy(common.CleanPodPolicyNone),
					},
					SSHAuthMountPath:  "/root/.ssh",
					MPIImplementation: MPIImplementationOpenMPI,
					MPIReplicaSpecs: map[MPIReplicaType]*common.ReplicaSpec{
						MPIReplicaTypeLauncher: {
							Replicas:      newInt32(1),
							RestartPolicy: DefaultLauncherRestartPolicy,
						},
					},
				},
			},
		},
		"worker defaults": {
			job: MPIJob{
				Spec: MPIJobSpec{
					MPIReplicaSpecs: map[MPIReplicaType]*common.ReplicaSpec{
						MPIReplicaTypeWorker: {},
					},
				},
			},
			want: MPIJob{
				Spec: MPIJobSpec{
					SlotsPerWorker: newInt32(1),
					RunPolicy: common.RunPolicy{
						CleanPodPolicy: newCleanPodPolicy(common.CleanPodPolicyNone),
					},
					SSHAuthMountPath:  "/root/.ssh",
					MPIImplementation: MPIImplementationOpenMPI,
					MPIReplicaSpecs: map[MPIReplicaType]*common.ReplicaSpec{
						MPIReplicaTypeWorker: {
							Replicas:      newInt32(0),
							RestartPolicy: DefaultRestartPolicy,
						},
					},
				},
			},
		},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			got := tc.job.DeepCopy()
			SetDefaults_MPIJob(got)
			if diff := cmp.Diff(tc.want, *got); diff != "" {
				t.Errorf("Unexpected changes (-want,+got):\n%s", diff)
			}
		})
	}
}

func newInt64(v int64) *int64 {
	return &v
}
