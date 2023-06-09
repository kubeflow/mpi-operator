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

package validation

import (
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	common "github.com/kubeflow/common/pkg/apis/common/v1"
	kubeflow "github.com/kubeflow/mpi-operator/pkg/apis/kubeflow/v2beta1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/validation/field"
)

func GenerateValidJob(mpiImplementation kubeflow.MPIImplementation, hasWorkers bool) kubeflow.MPIJob {

	replicaSpecs := map[kubeflow.MPIReplicaType]*common.ReplicaSpec{
		kubeflow.MPIReplicaTypeLauncher: {
			Replicas:      newInt32(1),
			RestartPolicy: common.RestartPolicyNever,
			Template: corev1.PodTemplateSpec{
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{{}},
				},
			},
		},
	}

	if hasWorkers {
		replicaSpecs[kubeflow.MPIReplicaTypeWorker] = &common.ReplicaSpec{
			Replicas:      newInt32(3),
			RestartPolicy: common.RestartPolicyNever,
			Template: corev1.PodTemplateSpec{
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{{}},
				},
			},
		}
	}

	job := kubeflow.MPIJob{
		ObjectMeta: metav1.ObjectMeta{
			Name: "foo",
		},
		Spec: kubeflow.MPIJobSpec{
			SlotsPerWorker: newInt32(2),
			RunPolicy: kubeflow.RunPolicy{
				CleanPodPolicy: kubeflow.NewCleanPodPolicy(kubeflow.CleanPodPolicyRunning),
			},
			SSHAuthMountPath:  "/home/mpiuser/.ssh",
			MPIImplementation: mpiImplementation,
			MPIReplicaSpecs: replicaSpecs,
		},
	}

	return job
}

func TestValidateMPIJob(t *testing.T) {
	cases := map[string]struct {
		job      kubeflow.MPIJob
		wantErrs field.ErrorList
	}{
		"valid (intel)": {
			job: GenerateValidJob(kubeflow.MPIImplementationIntel, false),
		},
		"valid with worker (intel)": {
			job: GenerateValidJob(kubeflow.MPIImplementationIntel, true),
		},
		"valid (mpich)": {
			job: GenerateValidJob(kubeflow.MPIImplementationMPICH, false),
		},
		"valid with worker (mpich)": {
			job: GenerateValidJob(kubeflow.MPIImplementationMPICH, true),
		},
		"empty job": {
			wantErrs: field.ErrorList{
				&field.Error{
					Type:  field.ErrorTypeInvalid,
					Field: "metadata.name",
				},
				&field.Error{
					Type:  field.ErrorTypeRequired,
					Field: "spec.mpiReplicaSpecs",
				},
				&field.Error{
					Type:  field.ErrorTypeRequired,
					Field: "spec.slotsPerWorker",
				},
				&field.Error{
					Type:  field.ErrorTypeRequired,
					Field: "spec.runPolicy.cleanPodPolicy",
				},
				&field.Error{
					Type:  field.ErrorTypeRequired,
					Field: "spec.sshAuthMountPath",
				},
				&field.Error{
					Type:  field.ErrorTypeNotSupported,
					Field: "spec.mpiImplementation",
				},
			},
		},
		"invalid fields": {
			job: kubeflow.MPIJob{
				ObjectMeta: metav1.ObjectMeta{
					Name: "this-name-is-waaaaaaaay-too-long-for-a-worker-hostname",
				},
				Spec: kubeflow.MPIJobSpec{
					SlotsPerWorker: newInt32(2),
					RunPolicy: kubeflow.RunPolicy{
						CleanPodPolicy:          kubeflow.NewCleanPodPolicy("unknown"),
						TTLSecondsAfterFinished: newInt32(-1),
						ActiveDeadlineSeconds:   newInt64(-1),
						BackoffLimit:            newInt32(-1),
					},
					SSHAuthMountPath:  "/root/.ssh",
					MPIImplementation: kubeflow.MPIImplementation("Unknown"),
					MPIReplicaSpecs: map[kubeflow.MPIReplicaType]*common.ReplicaSpec{
						kubeflow.MPIReplicaTypeLauncher: {
							Replicas:      newInt32(1),
							RestartPolicy: common.RestartPolicyNever,
							Template: corev1.PodTemplateSpec{
								Spec: corev1.PodSpec{
									Containers: []corev1.Container{{}},
								},
							},
						},
						kubeflow.MPIReplicaTypeWorker: {
							Replicas:      newInt32(1000),
							RestartPolicy: common.RestartPolicyNever,
							Template: corev1.PodTemplateSpec{
								Spec: corev1.PodSpec{
									Containers: []corev1.Container{{}},
								},
							},
						},
					},
				},
			},
			wantErrs: field.ErrorList{
				{
					Type:  field.ErrorTypeInvalid,
					Field: "metadata.name",
				},
				{
					Type:  field.ErrorTypeNotSupported,
					Field: "spec.runPolicy.cleanPodPolicy",
				},
				{
					Type:  field.ErrorTypeInvalid,
					Field: "spec.runPolicy.ttlSecondsAfterFinished",
				},
				{
					Type:  field.ErrorTypeInvalid,
					Field: "spec.runPolicy.activeDeadlineSeconds",
				},
				{
					Type:  field.ErrorTypeInvalid,
					Field: "spec.runPolicy.backoffLimit",
				},
				{
					Type:  field.ErrorTypeNotSupported,
					Field: "spec.mpiImplementation",
				},
			},
		},
		"empty replica specs": {
			job: kubeflow.MPIJob{
				ObjectMeta: metav1.ObjectMeta{
					Name: "foo",
				},
				Spec: kubeflow.MPIJobSpec{
					SlotsPerWorker: newInt32(2),
					RunPolicy: kubeflow.RunPolicy{
						CleanPodPolicy: kubeflow.NewCleanPodPolicy(kubeflow.CleanPodPolicyRunning),
					},
					SSHAuthMountPath:  "/root/.ssh",
					MPIImplementation: kubeflow.MPIImplementationOpenMPI,
					MPIReplicaSpecs:   map[kubeflow.MPIReplicaType]*common.ReplicaSpec{},
				},
			},
			wantErrs: field.ErrorList{
				&field.Error{
					Type:  field.ErrorTypeRequired,
					Field: "spec.mpiReplicaSpecs[Launcher]",
				},
			},
		},
		"missing replica spec fields": {
			job: kubeflow.MPIJob{
				ObjectMeta: metav1.ObjectMeta{
					Name: "foo",
				},
				Spec: kubeflow.MPIJobSpec{
					SlotsPerWorker: newInt32(2),
					RunPolicy: kubeflow.RunPolicy{
						CleanPodPolicy: kubeflow.NewCleanPodPolicy(kubeflow.CleanPodPolicyRunning),
					},
					SSHAuthMountPath:  "/root/.ssh",
					MPIImplementation: kubeflow.MPIImplementationOpenMPI,
					MPIReplicaSpecs: map[kubeflow.MPIReplicaType]*common.ReplicaSpec{
						kubeflow.MPIReplicaTypeLauncher: {},
						kubeflow.MPIReplicaTypeWorker:   {},
					},
				},
			},
			wantErrs: field.ErrorList{
				{
					Type:  field.ErrorTypeRequired,
					Field: "spec.mpiReplicaSpecs[Launcher].replicas",
				},
				{
					Type:  field.ErrorTypeNotSupported,
					Field: "spec.mpiReplicaSpecs[Launcher].restartPolicy",
				},
				{
					Type:  field.ErrorTypeRequired,
					Field: "spec.mpiReplicaSpecs[Launcher].template.spec.containers",
				},
				{
					Type:  field.ErrorTypeRequired,
					Field: "spec.mpiReplicaSpecs[Worker].replicas",
				},
				{
					Type:  field.ErrorTypeNotSupported,
					Field: "spec.mpiReplicaSpecs[Worker].restartPolicy",
				},
				{
					Type:  field.ErrorTypeRequired,
					Field: "spec.mpiReplicaSpecs[Worker].template.spec.containers",
				},
			},
		},
		"invalid replica fields": {
			job: kubeflow.MPIJob{
				ObjectMeta: metav1.ObjectMeta{
					Name: "foo",
				},
				Spec: kubeflow.MPIJobSpec{
					SlotsPerWorker: newInt32(2),
					RunPolicy: kubeflow.RunPolicy{
						CleanPodPolicy: kubeflow.NewCleanPodPolicy(kubeflow.CleanPodPolicyRunning),
					},
					SSHAuthMountPath:  "/root/.ssh",
					MPIImplementation: kubeflow.MPIImplementationOpenMPI,
					MPIReplicaSpecs: map[kubeflow.MPIReplicaType]*common.ReplicaSpec{
						kubeflow.MPIReplicaTypeLauncher: {
							Replicas:      newInt32(2),
							RestartPolicy: common.RestartPolicyAlways,
							Template: corev1.PodTemplateSpec{
								Spec: corev1.PodSpec{
									Containers: []corev1.Container{{}},
								},
							},
						},
						kubeflow.MPIReplicaTypeWorker: {
							Replicas:      newInt32(0),
							RestartPolicy: "Invalid",
							Template: corev1.PodTemplateSpec{
								Spec: corev1.PodSpec{
									Containers: []corev1.Container{{}},
								},
							},
						},
					},
				},
			},
			wantErrs: field.ErrorList{
				{
					Type:  field.ErrorTypeNotSupported,
					Field: "spec.mpiReplicaSpecs[Launcher].restartPolicy",
				},
				{
					Type:  field.ErrorTypeInvalid,
					Field: "spec.mpiReplicaSpecs[Launcher].replicas",
				},
				{
					Type:  field.ErrorTypeNotSupported,
					Field: "spec.mpiReplicaSpecs[Worker].restartPolicy",
				},
				{
					Type:  field.ErrorTypeInvalid,
					Field: "spec.mpiReplicaSpecs[Worker].replicas",
				},
			},
		},
		"invalid mpiJob name": {
			job: kubeflow.MPIJob{
				ObjectMeta: metav1.ObjectMeta{
					Name: "1-foo",
				},
				Spec: kubeflow.MPIJobSpec{
					SlotsPerWorker: newInt32(2),
					RunPolicy: kubeflow.RunPolicy{
						CleanPodPolicy: kubeflow.NewCleanPodPolicy(kubeflow.CleanPodPolicyRunning),
					},
					SSHAuthMountPath:  "/home/mpiuser/.ssh",
					MPIImplementation: kubeflow.MPIImplementationIntel,
					MPIReplicaSpecs: map[kubeflow.MPIReplicaType]*common.ReplicaSpec{
						kubeflow.MPIReplicaTypeLauncher: {
							Replicas:      newInt32(1),
							RestartPolicy: common.RestartPolicyNever,
							Template: corev1.PodTemplateSpec{
								Spec: corev1.PodSpec{
									Containers: []corev1.Container{{}},
								},
							},
						},
					},
				},
			},
			wantErrs: field.ErrorList{{
				Type:  field.ErrorTypeInvalid,
				Field: "metadata.name",
			}},
		},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			got := ValidateMPIJob(&tc.job)
			if diff := cmp.Diff(tc.wantErrs, got, cmpopts.IgnoreFields(field.Error{}, "Detail", "BadValue")); diff != "" {
				t.Errorf("Unexpected errors (-want,+got):\n%s", diff)
			}
		})
	}
}

func newInt32(v int32) *int32 {
	return &v
}

func newInt64(v int64) *int64 {
	return &v
}
