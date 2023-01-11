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
	"github.com/kubeflow/mpi-operator/v2/pkg/apis/kubeflow/v2beta1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/validation/field"
)

func TestValidateMPIJob(t *testing.T) {
	cases := map[string]struct {
		job      v2beta1.MPIJob
		wantErrs field.ErrorList
	}{
		"valid": {
			job: v2beta1.MPIJob{
				ObjectMeta: metav1.ObjectMeta{
					Name: "foo",
				},
				Spec: v2beta1.MPIJobSpec{
					SlotsPerWorker: newInt32(2),
					RunPolicy: common.RunPolicy{
						CleanPodPolicy: newCleanPodPolicy(common.CleanPodPolicyRunning),
					},
					SSHAuthMountPath:  "/home/mpiuser/.ssh",
					MPIImplementation: v2beta1.MPIImplementationIntel,
					MPIReplicaSpecs: map[v2beta1.MPIReplicaType]*common.ReplicaSpec{
						v2beta1.MPIReplicaTypeLauncher: {
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
		},
		"valid with worker": {
			job: v2beta1.MPIJob{
				ObjectMeta: metav1.ObjectMeta{
					Name: "foo",
				},
				Spec: v2beta1.MPIJobSpec{
					SlotsPerWorker: newInt32(2),
					RunPolicy: common.RunPolicy{
						CleanPodPolicy: newCleanPodPolicy(common.CleanPodPolicyRunning),
					},
					SSHAuthMountPath:  "/home/mpiuser/.ssh",
					MPIImplementation: v2beta1.MPIImplementationIntel,
					MPIReplicaSpecs: map[v2beta1.MPIReplicaType]*common.ReplicaSpec{
						v2beta1.MPIReplicaTypeLauncher: {
							Replicas:      newInt32(1),
							RestartPolicy: common.RestartPolicyOnFailure,
							Template: corev1.PodTemplateSpec{
								Spec: corev1.PodSpec{
									Containers: []corev1.Container{{}},
								},
							},
						},
						v2beta1.MPIReplicaTypeWorker: {
							Replicas:      newInt32(3),
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
			job: v2beta1.MPIJob{
				ObjectMeta: metav1.ObjectMeta{
					Name: "this-name-is-waaaaaaaay-too-long-for-a-worker-hostname",
				},
				Spec: v2beta1.MPIJobSpec{
					SlotsPerWorker: newInt32(2),
					RunPolicy: common.RunPolicy{
						CleanPodPolicy:          newCleanPodPolicy("unknown"),
						TTLSecondsAfterFinished: newInt32(-1),
						ActiveDeadlineSeconds:   newInt64(-1),
						BackoffLimit:            newInt32(-1),
					},
					SSHAuthMountPath:  "/root/.ssh",
					MPIImplementation: v2beta1.MPIImplementation("Unknown"),
					MPIReplicaSpecs: map[v2beta1.MPIReplicaType]*common.ReplicaSpec{
						v2beta1.MPIReplicaTypeLauncher: {
							Replicas:      newInt32(1),
							RestartPolicy: common.RestartPolicyNever,
							Template: corev1.PodTemplateSpec{
								Spec: corev1.PodSpec{
									Containers: []corev1.Container{{}},
								},
							},
						},
						v2beta1.MPIReplicaTypeWorker: {
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
			job: v2beta1.MPIJob{
				ObjectMeta: metav1.ObjectMeta{
					Name: "foo",
				},
				Spec: v2beta1.MPIJobSpec{
					SlotsPerWorker: newInt32(2),
					RunPolicy: common.RunPolicy{
						CleanPodPolicy: newCleanPodPolicy(common.CleanPodPolicyRunning),
					},
					SSHAuthMountPath:  "/root/.ssh",
					MPIImplementation: v2beta1.MPIImplementationOpenMPI,
					MPIReplicaSpecs:   map[v2beta1.MPIReplicaType]*common.ReplicaSpec{},
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
			job: v2beta1.MPIJob{
				ObjectMeta: metav1.ObjectMeta{
					Name: "foo",
				},
				Spec: v2beta1.MPIJobSpec{
					SlotsPerWorker: newInt32(2),
					RunPolicy: common.RunPolicy{
						CleanPodPolicy: newCleanPodPolicy(common.CleanPodPolicyRunning),
					},
					SSHAuthMountPath:  "/root/.ssh",
					MPIImplementation: v2beta1.MPIImplementationOpenMPI,
					MPIReplicaSpecs: map[v2beta1.MPIReplicaType]*common.ReplicaSpec{
						v2beta1.MPIReplicaTypeLauncher: {},
						v2beta1.MPIReplicaTypeWorker:   {},
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
			job: v2beta1.MPIJob{
				ObjectMeta: metav1.ObjectMeta{
					Name: "foo",
				},
				Spec: v2beta1.MPIJobSpec{
					SlotsPerWorker: newInt32(2),
					RunPolicy: common.RunPolicy{
						CleanPodPolicy: newCleanPodPolicy(common.CleanPodPolicyRunning),
					},
					SSHAuthMountPath:  "/root/.ssh",
					MPIImplementation: v2beta1.MPIImplementationOpenMPI,
					MPIReplicaSpecs: map[v2beta1.MPIReplicaType]*common.ReplicaSpec{
						v2beta1.MPIReplicaTypeLauncher: {
							Replicas:      newInt32(2),
							RestartPolicy: common.RestartPolicyAlways,
							Template: corev1.PodTemplateSpec{
								Spec: corev1.PodSpec{
									Containers: []corev1.Container{{}},
								},
							},
						},
						v2beta1.MPIReplicaTypeWorker: {
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

func newCleanPodPolicy(v common.CleanPodPolicy) *common.CleanPodPolicy {
	return &v
}
