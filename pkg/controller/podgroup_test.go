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
	"reflect"
	"sort"
	"testing"

	"github.com/google/go-cmp/cmp"
	corev1 "k8s.io/api/core/v1"
	schedulingv1 "k8s.io/api/scheduling/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/clock"
	"k8s.io/utils/ptr"
	schedv1alpha1 "sigs.k8s.io/scheduler-plugins/apis/scheduling/v1alpha1"
	volcanov1beta1 "volcano.sh/apis/pkg/apis/scheduling/v1beta1"

	kubeflow "github.com/kubeflow/mpi-operator/pkg/apis/kubeflow/v2beta1"
)

var (
	minResources = &corev1.ResourceList{
		corev1.ResourceCPU:    resource.MustParse("100"),
		corev1.ResourceMemory: resource.MustParse("512Gi"),
		"example.com/gpu":     resource.MustParse("40"),
	}

	minResourcesNoMinMember = &corev1.ResourceList{
		corev1.ResourceCPU:    resource.MustParse("1"),
		corev1.ResourceMemory: resource.MustParse("2Gi"),
	}
)

func TestNewPodGroup(t *testing.T) {
	testCases := map[string]struct {
		mpiJob        *kubeflow.MPIJob
		wantSchedPG   *schedv1alpha1.PodGroup
		wantVolcanoPG *volcanov1beta1.PodGroup
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
							MinAvailable:           ptr.To[int32](2),
							Queue:                  "project-y",
							PriorityClass:          "high",
							MinResources:           minResources,
							ScheduleTimeoutSeconds: ptr.To[int32](100),
						},
					},
					MPIReplicaSpecs: map[kubeflow.MPIReplicaType]*kubeflow.ReplicaSpec{
						kubeflow.MPIReplicaTypeLauncher: {
							Replicas: ptr.To[int32](1),
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
							Replicas: ptr.To[int32](1000),
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
			wantVolcanoPG: &volcanov1beta1.PodGroup{
				TypeMeta: metav1.TypeMeta{
					APIVersion: volcanov1beta1.SchemeGroupVersion.String(),
					Kind:       "PodGroup",
				},
				ObjectMeta: metav1.ObjectMeta{
					Name: "test",
				},
				Spec: volcanov1beta1.PodGroupSpec{
					MinMember:         2,
					Queue:             "project-y",
					PriorityClassName: "high",
					MinResources:      minResources,
				},
			},
			wantSchedPG: &schedv1alpha1.PodGroup{
				TypeMeta: metav1.TypeMeta{
					APIVersion: schedv1alpha1.SchemeGroupVersion.String(),
					Kind:       "PodGroup",
				},
				ObjectMeta: metav1.ObjectMeta{
					Name: "test",
				},
				Spec: schedv1alpha1.PodGroupSpec{
					MinMember:              2,
					MinResources:           *minResources,
					ScheduleTimeoutSeconds: ptr.To[int32](100),
				},
			},
		},
		"schedulingPolicy is nil": {
			mpiJob: &kubeflow.MPIJob{
				ObjectMeta: metav1.ObjectMeta{
					Name: "test",
					Annotations: map[string]string{
						volcanov1beta1.QueueNameAnnotationKey: "project-x",
					},
				},
				Spec: kubeflow.MPIJobSpec{
					MPIReplicaSpecs: map[kubeflow.MPIReplicaType]*kubeflow.ReplicaSpec{
						kubeflow.MPIReplicaTypeLauncher: {
							Replicas: ptr.To[int32](1),
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
							Replicas: ptr.To[int32](2),
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
			wantVolcanoPG: &volcanov1beta1.PodGroup{
				TypeMeta: metav1.TypeMeta{
					APIVersion: volcanov1beta1.SchemeGroupVersion.String(),
					Kind:       "PodGroup",
				},
				ObjectMeta: metav1.ObjectMeta{
					Name: "test",
				},
				Spec: volcanov1beta1.PodGroupSpec{
					MinMember:         3,
					Queue:             "project-x",
					PriorityClassName: "high",
					MinResources: &corev1.ResourceList{
						corev1.ResourceCPU:    resource.MustParse("21"),
						corev1.ResourceMemory: resource.MustParse("42Gi"),
					},
				},
			},
			wantSchedPG: &schedv1alpha1.PodGroup{
				TypeMeta: metav1.TypeMeta{
					APIVersion: schedv1alpha1.SchemeGroupVersion.String(),
					Kind:       "PodGroup",
				},
				ObjectMeta: metav1.ObjectMeta{
					Name: "test",
				},
				Spec: schedv1alpha1.PodGroupSpec{
					MinMember:              3,
					ScheduleTimeoutSeconds: ptr.To[int32](0),
					MinResources: corev1.ResourceList{
						corev1.ResourceCPU:    resource.MustParse("21"),
						corev1.ResourceMemory: resource.MustParse("42Gi"),
					},
				},
			},
		},
		"no worker no MinResources": {
			mpiJob: &kubeflow.MPIJob{
				ObjectMeta: metav1.ObjectMeta{
					Name: "test",
					Annotations: map[string]string{
						volcanov1beta1.QueueNameAnnotationKey: "project-x",
					},
				},
				Spec: kubeflow.MPIJobSpec{
					RunLauncherAsWorker: ptr.To[bool](true),
					RunPolicy: kubeflow.RunPolicy{
						SchedulingPolicy: &kubeflow.SchedulingPolicy{
							MinAvailable:           ptr.To[int32](1),
							Queue:                  "project-y",
							PriorityClass:          "high",
							ScheduleTimeoutSeconds: ptr.To[int32](100),
						},
					},
					MPIReplicaSpecs: map[kubeflow.MPIReplicaType]*kubeflow.ReplicaSpec{
						kubeflow.MPIReplicaTypeLauncher: {
							Replicas: ptr.To[int32](1),
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
					},
				},
			},
			wantVolcanoPG: &volcanov1beta1.PodGroup{
				TypeMeta: metav1.TypeMeta{
					APIVersion: volcanov1beta1.SchemeGroupVersion.String(),
					Kind:       "PodGroup",
				},
				ObjectMeta: metav1.ObjectMeta{
					Name: "test",
				},
				Spec: volcanov1beta1.PodGroupSpec{
					MinMember:         1,
					Queue:             "project-y",
					PriorityClassName: "high",
					MinResources:      minResourcesNoMinMember,
				},
			},
			wantSchedPG: &schedv1alpha1.PodGroup{
				TypeMeta: metav1.TypeMeta{
					APIVersion: schedv1alpha1.SchemeGroupVersion.String(),
					Kind:       "PodGroup",
				},
				ObjectMeta: metav1.ObjectMeta{
					Name: "test",
				},
				Spec: schedv1alpha1.PodGroupSpec{
					MinMember:              1,
					MinResources:           *minResourcesNoMinMember,
					ScheduleTimeoutSeconds: ptr.To[int32](100),
				},
			},
		},
	}
	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			volcanoFixture := newFixture(t, "default-scheduler")
			jobController, _, _ := volcanoFixture.newController(clock.RealClock{})
			volcanoPGCtrl := &VolcanoCtrl{
				Client:              volcanoFixture.volcanoClient,
				PriorityClassLister: jobController.priorityClassLister,
			}
			volcanoPG := volcanoPGCtrl.newPodGroup(tc.mpiJob)
			if diff := cmp.Diff(tc.wantVolcanoPG, volcanoPG, ignoreReferences); len(diff) != 0 {
				t.Errorf("Unexpected volcano PodGroup (-want,+got):\n%s", diff)
			}
			schedFixture := newFixture(t, "default-scheduler")
			schedController, _, _ := schedFixture.newController(clock.RealClock{})
			schedPGCtrl := &SchedulerPluginsCtrl{
				Client:              schedFixture.schedClient,
				PriorityClassLister: schedController.priorityClassLister,
			}
			schedPG := schedPGCtrl.newPodGroup(tc.mpiJob)
			if diff := cmp.Diff(tc.wantSchedPG, schedPG, ignoreReferences); len(diff) != 0 {
				t.Errorf("Unexpected scheduler-plugins PodGroup (-want,+got):\n%s", diff)
			}
		})
	}
}

func TestCalcPriorityClassName(t *testing.T) {
	testCases := map[string]struct {
		replicas   map[kubeflow.MPIReplicaType]*kubeflow.ReplicaSpec
		sp         *kubeflow.SchedulingPolicy
		wantPCName string
	}{
		"use schedulingPolicy": {
			replicas: map[kubeflow.MPIReplicaType]*kubeflow.ReplicaSpec{},
			sp: &kubeflow.SchedulingPolicy{
				PriorityClass: "high",
			},
			wantPCName: "high",
		},
		"use launcher": {
			replicas: map[kubeflow.MPIReplicaType]*kubeflow.ReplicaSpec{
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
			replicas: map[kubeflow.MPIReplicaType]*kubeflow.ReplicaSpec{
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
			replicas: map[kubeflow.MPIReplicaType]*kubeflow.ReplicaSpec{
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
			pg := calculatePriorityClassName(tc.replicas, tc.sp)
			if diff := cmp.Diff(tc.wantPCName, pg); len(diff) != 0 {
				t.Errorf("Unexpected priorityClass name (-want,+got):\n%s", diff)
			}
		})
	}
}

func TestDecoratePodTemplateSpec(t *testing.T) {
	jobName := "test-mpijob"
	schedulerPluginsSchedulerName := "default-scheduler"
	tests := map[string]struct {
		wantVolcanoPts, wantSchedPts *corev1.PodTemplateSpec
	}{
		"set schedulerName and podGroup name": {
			wantVolcanoPts: &corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Annotations: map[string]string{
						volcanov1beta1.KubeGroupNameAnnotationKey: jobName,
					},
				},
				Spec: corev1.PodSpec{
					SchedulerName: "volcano",
				},
			},
			wantSchedPts: &corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: map[string]string{
						schedv1alpha1.PodGroupLabel: jobName,
					},
				},
				Spec: corev1.PodSpec{
					SchedulerName: schedulerPluginsSchedulerName,
				},
			},
		},
	}
	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			volcanoInput := &corev1.PodTemplateSpec{}
			volcanoF := newFixture(t, "scheduler-plugins-scheduler")
			volcanoPGCtrl := &VolcanoCtrl{
				Client:        volcanoF.volcanoClient,
				schedulerName: "volcano",
			}
			volcanoPGCtrl.decoratePodTemplateSpec(volcanoInput, jobName)
			if diff := cmp.Diff(tc.wantVolcanoPts, volcanoInput); len(diff) != 0 {
				t.Fatalf("Unexpected decoratePodTemplateSpec for the volcano (-want,+got):\n%s", diff)
			}
			schedInput := &corev1.PodTemplateSpec{}
			schedF := newFixture(t, "scheduler-plugins-scheduler")
			schedPGCtrl := &SchedulerPluginsCtrl{
				Client:        schedF.schedClient,
				schedulerName: schedulerPluginsSchedulerName,
			}
			schedPGCtrl.decoratePodTemplateSpec(schedInput, jobName)
			if diff := cmp.Diff(tc.wantSchedPts, schedInput); len(diff) != 0 {
				t.Fatalf("Unexpected decoratePodTemplateSpec for the scheduler-plugins (-want,+got):\n%s", diff)
			}
		})
	}
}

func TestCalculatePGMinResources(t *testing.T) {
	volcanoTests := map[string]struct {
		job             *kubeflow.MPIJob
		priorityClasses []*schedulingv1.PriorityClass
		minMember       int32
		want            *corev1.ResourceList
	}{
		"minResources is not empty": {
			job: &kubeflow.MPIJob{
				ObjectMeta: metav1.ObjectMeta{
					Name: "test",
				},
				Spec: kubeflow.MPIJobSpec{
					RunPolicy: kubeflow.RunPolicy{
						SchedulingPolicy: &kubeflow.SchedulingPolicy{
							MinResources: minResources,
						},
					},
				},
			},
			want: minResources,
		},
		"schedulingPolicy is nil": {
			job: &kubeflow.MPIJob{
				ObjectMeta: metav1.ObjectMeta{
					Name: "test",
				},
				Spec: kubeflow.MPIJobSpec{},
			},
			want: nil,
		},
		"without priorityClass": {
			minMember: 3,
			job: &kubeflow.MPIJob{
				ObjectMeta: metav1.ObjectMeta{
					Name: "test",
				},
				Spec: kubeflow.MPIJobSpec{
					MPIReplicaSpecs: map[kubeflow.MPIReplicaType]*kubeflow.ReplicaSpec{
						kubeflow.MPIReplicaTypeLauncher: {
							Replicas: ptr.To[int32](1),
							Template: corev1.PodTemplateSpec{
								Spec: corev1.PodSpec{
									Containers: []corev1.Container{
										{
											Resources: corev1.ResourceRequirements{
												Requests: corev1.ResourceList{
													corev1.ResourceCPU:    resource.MustParse("2"),
													corev1.ResourceMemory: resource.MustParse("1Gi"),
												},
											},
										},
									},
								},
							},
						},
						kubeflow.MPIReplicaTypeWorker: {
							Replicas: ptr.To[int32](2),
							Template: corev1.PodTemplateSpec{
								Spec: corev1.PodSpec{
									Containers: []corev1.Container{
										{
											Resources: corev1.ResourceRequirements{
												Requests: corev1.ResourceList{
													corev1.ResourceCPU:    resource.MustParse("10"),
													corev1.ResourceMemory: resource.MustParse("32Gi"),
												},
											},
										},
									},
								},
							},
						},
					},
				},
			},
			want: &corev1.ResourceList{
				corev1.ResourceCPU:    resource.MustParse("22"),
				corev1.ResourceMemory: resource.MustParse("65Gi"),
			},
		},
		"without worker without priorityClass": {
			minMember: 1,
			job: &kubeflow.MPIJob{
				ObjectMeta: metav1.ObjectMeta{
					Name: "test",
				},
				Spec: kubeflow.MPIJobSpec{
					MPIReplicaSpecs: map[kubeflow.MPIReplicaType]*kubeflow.ReplicaSpec{
						kubeflow.MPIReplicaTypeLauncher: {
							Replicas: ptr.To[int32](1),
							Template: corev1.PodTemplateSpec{
								Spec: corev1.PodSpec{
									Containers: []corev1.Container{
										{
											Resources: corev1.ResourceRequirements{
												Requests: corev1.ResourceList{
													corev1.ResourceCPU:    resource.MustParse("2"),
													corev1.ResourceMemory: resource.MustParse("1Gi"),
												},
											},
										},
									},
								},
							},
						},
					},
				},
			},
			want: &corev1.ResourceList{
				corev1.ResourceCPU:    resource.MustParse("2"),
				corev1.ResourceMemory: resource.MustParse("1Gi"),
			},
		},
	}
	for name, tc := range volcanoTests {
		t.Run(name, func(t *testing.T) {
			f := newFixture(t, "volcano-scheduler")
			if tc.priorityClasses != nil {
				for _, pc := range tc.priorityClasses {
					f.setUpPriorityClass(pc)
				}
			}
			jobController, _, _ := f.newController(clock.RealClock{})
			pgCtrl := VolcanoCtrl{Client: f.volcanoClient, PriorityClassLister: jobController.priorityClassLister}
			got := pgCtrl.calculatePGMinResources(&tc.minMember, tc.job)
			if diff := cmp.Diff(tc.want, got); len(diff) != 0 {
				t.Fatalf("Unexpected calculatePGMinResources for the volcano (-want,+got):\n%s", diff)
			}
		})
	}

	schedTests := map[string]struct {
		job             *kubeflow.MPIJob
		minMember       *int32
		priorityClasses []*schedulingv1.PriorityClass
		want            *corev1.ResourceList
	}{
		"schedulingPolicy.minResources isn't empty": {
			job: &kubeflow.MPIJob{
				ObjectMeta: metav1.ObjectMeta{
					Name: "test",
				},
				Spec: kubeflow.MPIJobSpec{
					RunPolicy: kubeflow.RunPolicy{
						SchedulingPolicy: &kubeflow.SchedulingPolicy{
							MinResources: minResources,
						},
					},
				},
			},
			want: minResources,
		},
		"schedulingPolicy.minMember is 0": {
			job: &kubeflow.MPIJob{
				ObjectMeta: metav1.ObjectMeta{
					Name: "test",
				},
			},
			minMember: ptr.To[int32](0),
			want:      nil,
		},
		"without priorityClass": {
			job: &kubeflow.MPIJob{
				ObjectMeta: metav1.ObjectMeta{
					Name: "test",
				},
				Spec: kubeflow.MPIJobSpec{
					MPIReplicaSpecs: map[kubeflow.MPIReplicaType]*kubeflow.ReplicaSpec{
						kubeflow.MPIReplicaTypeLauncher: {
							Replicas: ptr.To[int32](1),
							Template: corev1.PodTemplateSpec{
								Spec: corev1.PodSpec{
									Containers: []corev1.Container{
										{
											Resources: corev1.ResourceRequirements{
												Requests: corev1.ResourceList{
													corev1.ResourceCPU:    resource.MustParse("2"),
													corev1.ResourceMemory: resource.MustParse("1Gi"),
												},
											},
										},
									},
								},
							},
						},
						kubeflow.MPIReplicaTypeWorker: {
							Replicas: ptr.To[int32](2),
							Template: corev1.PodTemplateSpec{
								Spec: corev1.PodSpec{
									Containers: []corev1.Container{
										{
											Resources: corev1.ResourceRequirements{
												Requests: corev1.ResourceList{
													corev1.ResourceCPU:    resource.MustParse("10"),
													corev1.ResourceMemory: resource.MustParse("32Gi"),
												},
											},
										},
										{
											Resources: corev1.ResourceRequirements{
												Requests: corev1.ResourceList{
													corev1.ResourceCPU:    resource.MustParse("50"),
													corev1.ResourceMemory: resource.MustParse("512Gi"),
												},
											},
										},
									},
								},
							},
						},
					},
				},
			},
			want: &corev1.ResourceList{
				corev1.ResourceCPU:    resource.MustParse("122"),
				corev1.ResourceMemory: resource.MustParse("1089Gi"),
			},
		},
		"with non-existence priorityClass": {
			minMember: ptr.To[int32](2),
			job: &kubeflow.MPIJob{
				ObjectMeta: metav1.ObjectMeta{
					Name: "test",
				},
				Spec: kubeflow.MPIJobSpec{
					MPIReplicaSpecs: map[kubeflow.MPIReplicaType]*kubeflow.ReplicaSpec{
						kubeflow.MPIReplicaTypeLauncher: {
							Replicas: ptr.To[int32](1),
							Template: corev1.PodTemplateSpec{
								Spec: corev1.PodSpec{
									PriorityClassName: "non-existence",
									Containers: []corev1.Container{
										{
											Resources: corev1.ResourceRequirements{
												Requests: corev1.ResourceList{
													corev1.ResourceCPU:    resource.MustParse("2"),
													corev1.ResourceMemory: resource.MustParse("2Gi"),
												},
											},
										},
									},
								},
							},
						},
						kubeflow.MPIReplicaTypeWorker: {
							Replicas: ptr.To[int32](2),
							Template: corev1.PodTemplateSpec{
								Spec: corev1.PodSpec{
									PriorityClassName: "non-existence",
									Containers: []corev1.Container{
										{
											Resources: corev1.ResourceRequirements{
												Requests: corev1.ResourceList{
													corev1.ResourceCPU:    resource.MustParse("5"),
													corev1.ResourceMemory: resource.MustParse("16Gi"),
												},
											},
										},
									},
								},
							},
						},
					},
				},
			},
			want: &corev1.ResourceList{
				corev1.ResourceCPU:    resource.MustParse("7"),
				corev1.ResourceMemory: resource.MustParse("18Gi"),
			},
		},
		"with existence priorityClass": {
			priorityClasses: []*schedulingv1.PriorityClass{
				{
					ObjectMeta: metav1.ObjectMeta{
						Name: "high",
					},
					Value: 100_010,
				},
				{
					ObjectMeta: metav1.ObjectMeta{
						Name: "low",
					},
					Value: 10_010,
				},
			},
			minMember: ptr.To[int32](2),
			job: &kubeflow.MPIJob{
				ObjectMeta: metav1.ObjectMeta{
					Name: "test",
				},
				Spec: kubeflow.MPIJobSpec{
					MPIReplicaSpecs: map[kubeflow.MPIReplicaType]*kubeflow.ReplicaSpec{
						kubeflow.MPIReplicaTypeLauncher: {
							Replicas: ptr.To[int32](1),
							Template: corev1.PodTemplateSpec{
								Spec: corev1.PodSpec{
									PriorityClassName: "high",
									Containers: []corev1.Container{
										{
											Resources: corev1.ResourceRequirements{
												Requests: corev1.ResourceList{
													corev1.ResourceCPU:    resource.MustParse("2"),
													corev1.ResourceMemory: resource.MustParse("4Gi"),
												},
											},
										},
									},
								},
							},
						},
						kubeflow.MPIReplicaTypeWorker: {
							Replicas: ptr.To[int32](100),
							Template: corev1.PodTemplateSpec{
								Spec: corev1.PodSpec{
									PriorityClassName: "low",
									Containers: []corev1.Container{
										{
											Resources: corev1.ResourceRequirements{
												Requests: corev1.ResourceList{
													corev1.ResourceCPU:    resource.MustParse("20"),
													corev1.ResourceMemory: resource.MustParse("64Gi"),
												},
											},
										},
									},
								},
							},
						},
					},
				},
			},
			want: &corev1.ResourceList{
				corev1.ResourceCPU:    resource.MustParse("22"),
				corev1.ResourceMemory: resource.MustParse("68Gi"),
			},
		},
	}
	for name, tc := range schedTests {
		t.Run(name, func(t *testing.T) {
			f := newFixture(t, "default-scheduler")
			if tc.priorityClasses != nil {
				for _, pc := range tc.priorityClasses {
					f.setUpPriorityClass(pc)
				}
			}
			jobController, _, _ := f.newController(clock.RealClock{})
			pgCtrl := SchedulerPluginsCtrl{
				Client:              f.schedClient,
				PriorityClassLister: jobController.priorityClassLister,
			}
			got := pgCtrl.calculatePGMinResources(tc.minMember, tc.job)
			if diff := cmp.Diff(tc.want, got); len(diff) != 0 {
				t.Fatalf("Unexpected calculatePGMinResources for the scheduler-plugins (-want,+got):\n%s", diff)
			}
		})
	}
}

func TestAddResources(t *testing.T) {
	tests := map[string]struct {
		minResources corev1.ResourceList
		resources    corev1.ResourceRequirements
		want         corev1.ResourceList
	}{
		"minResources is nil": {
			minResources: nil,
			resources: corev1.ResourceRequirements{
				Requests: corev1.ResourceList{
					corev1.ResourceCPU: resource.MustParse("20"),
				},
			},
			want: nil,
		},
		"add a new resource to minResources": {
			minResources: corev1.ResourceList{},
			resources: corev1.ResourceRequirements{
				Requests: corev1.ResourceList{
					corev1.ResourceCPU: resource.MustParse("20"),
				},
			},
			want: corev1.ResourceList{
				corev1.ResourceCPU: resource.MustParse("20"),
			},
		},
		"increase a quantity of minResources": {
			minResources: corev1.ResourceList{
				corev1.ResourceMemory: resource.MustParse("20Gi"),
			},
			resources: corev1.ResourceRequirements{
				Requests: corev1.ResourceList{
					corev1.ResourceMemory: resource.MustParse("10Gi"),
				},
			},
			want: corev1.ResourceList{
				corev1.ResourceMemory: resource.MustParse("30Gi"),
			},
		},
		"same resource names exist in requests and limits": {
			minResources: corev1.ResourceList{},
			resources: corev1.ResourceRequirements{
				Requests: corev1.ResourceList{
					corev1.ResourceCPU:    resource.MustParse("10"),
					corev1.ResourceMemory: resource.MustParse("16Gi"),
				},
				Limits: corev1.ResourceList{
					"example.com/gpu":     resource.MustParse("8"),
					corev1.ResourceCPU:    resource.MustParse("20"),
					corev1.ResourceMemory: resource.MustParse("32Gi"),
				},
			},
			want: corev1.ResourceList{
				"example.com/gpu":     resource.MustParse("8"),
				corev1.ResourceCPU:    resource.MustParse("10"),
				corev1.ResourceMemory: resource.MustParse("16Gi"),
			},
		},
	}
	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			addResources(tc.minResources, tc.resources, 1)
			if diff := cmp.Diff(tc.want, tc.minResources); len(diff) != 0 {
				t.Fatalf("Unexpected resourceList (-want,+got):\n%s", diff)
			}
		})
	}
}

func TestCalculateMinAvailable(t *testing.T) {
	tests := map[string]struct {
		job  *kubeflow.MPIJob
		want int32
	}{
		"minAvailable isn't empty": {
			job: &kubeflow.MPIJob{
				ObjectMeta: metav1.ObjectMeta{
					Name: "test",
				},
				Spec: kubeflow.MPIJobSpec{
					RunPolicy: kubeflow.RunPolicy{
						SchedulingPolicy: &kubeflow.SchedulingPolicy{
							MinAvailable: ptr.To[int32](2),
						},
					},
					MPIReplicaSpecs: map[kubeflow.MPIReplicaType]*kubeflow.ReplicaSpec{
						kubeflow.MPIReplicaTypeLauncher: {
							Replicas: ptr.To[int32](1),
						},
						kubeflow.MPIReplicaTypeWorker: {
							Replicas: ptr.To[int32](1000),
						},
					},
				},
			},
			want: 2,
		},
		"minAvailable is empty": {
			job: &kubeflow.MPIJob{
				ObjectMeta: metav1.ObjectMeta{
					Name: "test",
				},
				Spec: kubeflow.MPIJobSpec{
					MPIReplicaSpecs: map[kubeflow.MPIReplicaType]*kubeflow.ReplicaSpec{
						kubeflow.MPIReplicaTypeLauncher: {
							Replicas: ptr.To[int32](1),
						},
						kubeflow.MPIReplicaTypeWorker: {
							Replicas: ptr.To[int32](99),
						},
					},
				},
			},
			want: 100,
		},
	}
	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			got := calculateMinAvailable(tc.job)
			if tc.want != *got {
				t.Fatalf("Unexpected calculateMinAvailable: (want: %v, got: %v)", tc.want, *got)
			}
		})
	}
}

func TestReplicasOrder(t *testing.T) {
	var lancherReplic, wokerReplic int32 = 1, 2
	tests := map[string]struct {
		original replicasOrder
		expected replicasOrder
	}{
		"1-lancher, 2-worker, lancher higher priority": {
			original: replicasOrder{
				{priority: 1, replicaType: kubeflow.MPIReplicaTypeLauncher, ReplicaSpec: kubeflow.ReplicaSpec{Replicas: &lancherReplic}},
				{priority: 0, replicaType: kubeflow.MPIReplicaTypeWorker, ReplicaSpec: kubeflow.ReplicaSpec{Replicas: &wokerReplic}},
			},
			expected: replicasOrder{
				{priority: 1, replicaType: kubeflow.MPIReplicaTypeLauncher, ReplicaSpec: kubeflow.ReplicaSpec{Replicas: &lancherReplic}},
				{priority: 0, replicaType: kubeflow.MPIReplicaTypeWorker, ReplicaSpec: kubeflow.ReplicaSpec{Replicas: &wokerReplic}},
			},
		},
		"1-lancher, 2-worker, equal priority": {
			original: replicasOrder{
				{priority: 0, replicaType: kubeflow.MPIReplicaTypeWorker, ReplicaSpec: kubeflow.ReplicaSpec{Replicas: &wokerReplic}},
				{priority: 0, replicaType: kubeflow.MPIReplicaTypeLauncher, ReplicaSpec: kubeflow.ReplicaSpec{Replicas: &lancherReplic}},
			},
			expected: replicasOrder{
				{priority: 0, replicaType: kubeflow.MPIReplicaTypeWorker, ReplicaSpec: kubeflow.ReplicaSpec{Replicas: &wokerReplic}},
				{priority: 0, replicaType: kubeflow.MPIReplicaTypeLauncher, ReplicaSpec: kubeflow.ReplicaSpec{Replicas: &lancherReplic}},
			},
		},
	}
	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			sort.Sort(sort.Reverse(tc.original))
			if !reflect.DeepEqual(tc.original, tc.expected) {
				t.Fatalf("Unexpected sort list (-want,+got):\n-want:%v\n+got:%v", tc.expected, tc.original)
			}
		})
	}
}
