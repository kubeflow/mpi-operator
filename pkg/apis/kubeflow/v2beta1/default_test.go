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
	"k8s.io/utils/ptr"
)

func TestSetDefaults_MPIJob(t *testing.T) {
	cases := map[string]struct {
		job  MPIJob
		want MPIJob
	}{
		"base defaults": {
			want: MPIJob{
				Spec: MPIJobSpec{
					SlotsPerWorker: ptr.To[int32](1),
					RunPolicy: RunPolicy{
						CleanPodPolicy: ptr.To(CleanPodPolicyNone),
					},
					SSHAuthMountPath:       "/root/.ssh",
					MPIImplementation:      MPIImplementationOpenMPI,
					LauncherCreationPolicy: "AtStartup",
				},
			},
		},
		"base defaults overridden (intel)": {
			job: MPIJob{
				Spec: MPIJobSpec{
					SlotsPerWorker: ptr.To[int32](10),
					RunPolicy: RunPolicy{
						CleanPodPolicy:          ptr.To(CleanPodPolicyRunning),
						TTLSecondsAfterFinished: ptr.To[int32](2),
						ActiveDeadlineSeconds:   ptr.To[int64](3),
						BackoffLimit:            ptr.To[int32](4),
					},
					SSHAuthMountPath:       "/home/mpiuser/.ssh",
					MPIImplementation:      MPIImplementationIntel,
					LauncherCreationPolicy: "AtStartup",
				},
			},
			want: MPIJob{
				Spec: MPIJobSpec{
					SlotsPerWorker: ptr.To[int32](10),
					RunPolicy: RunPolicy{
						CleanPodPolicy:          ptr.To(CleanPodPolicyRunning),
						TTLSecondsAfterFinished: ptr.To[int32](2),
						ActiveDeadlineSeconds:   ptr.To[int64](3),
						BackoffLimit:            ptr.To[int32](4),
					},
					SSHAuthMountPath:       "/home/mpiuser/.ssh",
					MPIImplementation:      MPIImplementationIntel,
					LauncherCreationPolicy: "AtStartup",
				},
			},
		},
		"base defaults overridden (mpich)": {
			job: MPIJob{
				Spec: MPIJobSpec{
					SlotsPerWorker: ptr.To[int32](10),
					RunPolicy: RunPolicy{
						CleanPodPolicy:          ptr.To(CleanPodPolicyRunning),
						TTLSecondsAfterFinished: ptr.To[int32](2),
						ActiveDeadlineSeconds:   ptr.To[int64](3),
						BackoffLimit:            ptr.To[int32](4),
					},
					SSHAuthMountPath:       "/home/mpiuser/.ssh",
					MPIImplementation:      MPIImplementationMPICH,
					LauncherCreationPolicy: "AtStartup",
				},
			},
			want: MPIJob{
				Spec: MPIJobSpec{
					SlotsPerWorker: ptr.To[int32](10),
					RunPolicy: RunPolicy{
						CleanPodPolicy:          ptr.To(CleanPodPolicyRunning),
						TTLSecondsAfterFinished: ptr.To[int32](2),
						ActiveDeadlineSeconds:   ptr.To[int64](3),
						BackoffLimit:            ptr.To[int32](4),
					},
					SSHAuthMountPath:       "/home/mpiuser/.ssh",
					MPIImplementation:      MPIImplementationMPICH,
					LauncherCreationPolicy: "AtStartup",
				},
			},
		},
		"launcher defaults": {
			job: MPIJob{
				Spec: MPIJobSpec{
					MPIReplicaSpecs: map[MPIReplicaType]*ReplicaSpec{
						MPIReplicaTypeLauncher: {},
					},
				},
			},
			want: MPIJob{
				Spec: MPIJobSpec{
					SlotsPerWorker: ptr.To[int32](1),
					RunPolicy: RunPolicy{
						CleanPodPolicy: ptr.To(CleanPodPolicyNone),
					},
					SSHAuthMountPath:       "/root/.ssh",
					MPIImplementation:      MPIImplementationOpenMPI,
					LauncherCreationPolicy: "AtStartup",
					MPIReplicaSpecs: map[MPIReplicaType]*ReplicaSpec{
						MPIReplicaTypeLauncher: {
							Replicas:      ptr.To[int32](1),
							RestartPolicy: DefaultLauncherRestartPolicy,
						},
					},
				},
			},
		},
		"worker defaults": {
			job: MPIJob{
				Spec: MPIJobSpec{
					MPIReplicaSpecs: map[MPIReplicaType]*ReplicaSpec{
						MPIReplicaTypeWorker: {},
					},
				},
			},
			want: MPIJob{
				Spec: MPIJobSpec{
					SlotsPerWorker: ptr.To[int32](1),
					RunPolicy: RunPolicy{
						CleanPodPolicy: ptr.To(CleanPodPolicyNone),
					},
					SSHAuthMountPath:       "/root/.ssh",
					MPIImplementation:      MPIImplementationOpenMPI,
					LauncherCreationPolicy: "AtStartup",
					MPIReplicaSpecs: map[MPIReplicaType]*ReplicaSpec{
						MPIReplicaTypeWorker: {
							Replicas:      ptr.To[int32](0),
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
