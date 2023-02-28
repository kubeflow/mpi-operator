// Copyright 2023 The Kubeflow Authors.
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

package controller

import (
	"testing"

	"github.com/google/go-cmp/cmp"
	common "github.com/kubeflow/common/pkg/apis/common/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/pointer"
	volcanov1beta1 "volcano.sh/apis/pkg/apis/scheduling/v1beta1"

	kubeflow "github.com/kubeflow/mpi-operator/pkg/apis/kubeflow/v2beta1"
)

func TestNewPodGroup(t *testing.T) {
	testCases := map[string]struct {
		mpiJob       *kubeflow.MPIJob
		wantPodGroup *volcanov1beta1.PodGroup
	}{
		"all schedulingPolicy fields are set": {
			mpiJob: &kubeflow.MPIJob{
				ObjectMeta: metav1.ObjectMeta{
					Name: "test",
					Annotations: map[string]string{
						volcanov1beta1.QueueNameAnnotationKey: "project-x",
					},
				},
				Spec: kubeflow.MPIJobSpec{
					RunPolicy: kubeflow.RunPolicy{
						SchedulingPolicy: &kubeflow.SchedulingPolicy{
							MinAvailable:  pointer.Int32(2),
							Queue:         "project-y",
							PriorityClass: "high",
							MinResources: &corev1.ResourceList{
								corev1.ResourceCPU:    resource.MustParse("100"),
								corev1.ResourceMemory: resource.MustParse("512Gi"),
								"example.com/gpu":     resource.MustParse("40"),
							},
						},
					},
					MPIReplicaSpecs: map[kubeflow.MPIReplicaType]*common.ReplicaSpec{
						kubeflow.MPIReplicaTypeLauncher: {
							Replicas: pointer.Int32(1),
							Template: corev1.PodTemplateSpec{
								Spec: corev1.PodSpec{
									Containers: []corev1.Container{{
										Resources: corev1.ResourceRequirements{
											Requests: corev1.ResourceList{
												corev1.ResourceCPU:    resource.MustParse("1"),
												corev1.ResourceMemory: resource.MustParse("2Gi"),
											},
										},
									}},
								},
							},
						},
						kubeflow.MPIReplicaTypeWorker: {
							Replicas: pointer.Int32(1000),
							Template: corev1.PodTemplateSpec{
								Spec: corev1.PodSpec{
									Containers: []corev1.Container{{
										Resources: corev1.ResourceRequirements{
											Requests: corev1.ResourceList{
												corev1.ResourceCPU:    resource.MustParse("10"),
												corev1.ResourceMemory: resource.MustParse("20Gi"),
											},
										},
									}},
								},
							},
						},
					},
				},
			},
			wantPodGroup: &volcanov1beta1.PodGroup{
				ObjectMeta: metav1.ObjectMeta{
					Name: "test",
				},
				Spec: volcanov1beta1.PodGroupSpec{
					MinMember:         2,
					Queue:             "project-y",
					PriorityClassName: "high",
					MinResources: &corev1.ResourceList{
						corev1.ResourceCPU:    resource.MustParse("100"),
						corev1.ResourceMemory: resource.MustParse("512Gi"),
						"example.com/gpu":     resource.MustParse("40"),
					},
				},
			},
		},
		"schedulingPolicy is empty": {
			mpiJob: &kubeflow.MPIJob{
				ObjectMeta: metav1.ObjectMeta{
					Name: "test",
					Annotations: map[string]string{
						volcanov1beta1.QueueNameAnnotationKey: "project-x",
					},
				},
				Spec: kubeflow.MPIJobSpec{
					MPIReplicaSpecs: map[kubeflow.MPIReplicaType]*common.ReplicaSpec{
						kubeflow.MPIReplicaTypeLauncher: {
							Replicas: pointer.Int32(1),
							Template: corev1.PodTemplateSpec{
								Spec: corev1.PodSpec{
									PriorityClassName: "high",
									Containers: []corev1.Container{{
										Resources: corev1.ResourceRequirements{
											Requests: corev1.ResourceList{
												corev1.ResourceCPU:    resource.MustParse("1"),
												corev1.ResourceMemory: resource.MustParse("2Gi"),
											},
										},
									}},
								},
							},
						},
						kubeflow.MPIReplicaTypeWorker: {
							Replicas: pointer.Int32(2),
							Template: corev1.PodTemplateSpec{
								Spec: corev1.PodSpec{
									Containers: []corev1.Container{{
										Resources: corev1.ResourceRequirements{
											Requests: corev1.ResourceList{
												corev1.ResourceCPU:    resource.MustParse("10"),
												corev1.ResourceMemory: resource.MustParse("20Gi"),
											},
										},
									}},
								},
							},
						},
					},
				},
			},
			wantPodGroup: &volcanov1beta1.PodGroup{
				ObjectMeta: metav1.ObjectMeta{
					Name: "test",
				},
				Spec: volcanov1beta1.PodGroupSpec{
					MinMember:         3,
					Queue:             "project-x",
					PriorityClassName: "high",
				},
			},
		},
	}

	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			pg := newPodGroup(tc.mpiJob)
			if !metav1.IsControlledBy(pg, tc.mpiJob) {
				t.Errorf("Created podGroup is not controlled by MPIJob")
			}
			if diff := cmp.Diff(tc.wantPodGroup, pg, ignoreReferences); len(diff) != 0 {
				t.Errorf("Unexpected podGroup (-want,+got):\n%s", diff)
			}
		})
	}
}

func TestCalcPriorityClassName(t *testing.T) {
	testCases := map[string]struct {
		replicas   map[kubeflow.MPIReplicaType]*common.ReplicaSpec
		sp         *kubeflow.SchedulingPolicy
		wantPCName string
	}{
		"use schedulingPolicy": {
			replicas: map[kubeflow.MPIReplicaType]*common.ReplicaSpec{},
			sp: &kubeflow.SchedulingPolicy{
				PriorityClass: "high",
			},
			wantPCName: "high",
		},
		"use launcher": {
			replicas: map[kubeflow.MPIReplicaType]*common.ReplicaSpec{
				kubeflow.MPIReplicaTypeLauncher: {
					Template: corev1.PodTemplateSpec{
						Spec: corev1.PodSpec{
							PriorityClassName: "high",
						},
					},
				},
				kubeflow.MPIReplicaTypeWorker: {
					Template: corev1.PodTemplateSpec{
						Spec: corev1.PodSpec{
							PriorityClassName: "low",
						},
					},
				},
			},
			wantPCName: "high",
		},
		"use worker": {
			replicas: map[kubeflow.MPIReplicaType]*common.ReplicaSpec{
				kubeflow.MPIReplicaTypeLauncher: {
					Template: corev1.PodTemplateSpec{
						Spec: corev1.PodSpec{},
					},
				},
				kubeflow.MPIReplicaTypeWorker: {
					Template: corev1.PodTemplateSpec{
						Spec: corev1.PodSpec{
							PriorityClassName: "low",
						},
					},
				},
			},
			wantPCName: "low",
		},
		"nothing": {
			replicas: map[kubeflow.MPIReplicaType]*common.ReplicaSpec{
				kubeflow.MPIReplicaTypeLauncher: {
					Template: corev1.PodTemplateSpec{
						Spec: corev1.PodSpec{},
					},
				},
				kubeflow.MPIReplicaTypeWorker: {
					Template: corev1.PodTemplateSpec{
						Spec: corev1.PodSpec{},
					},
				},
			},
			wantPCName: "",
		},
	}

	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			pg := calcPriorityClassName(tc.replicas, tc.sp)
			if diff := cmp.Diff(tc.wantPCName, pg); len(diff) != 0 {
				t.Errorf("Unexpected priorityClass name (-want,+got):\n%s", diff)
			}
		})
	}
}
