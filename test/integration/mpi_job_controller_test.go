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

package integration

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	schedulingv1 "k8s.io/api/scheduling/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/apimachinery/pkg/util/wait"
	kubeinformers "k8s.io/client-go/informers"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/reference"
	"k8s.io/client-go/util/workqueue"
	"k8s.io/utils/ptr"
	schedv1alpha1 "sigs.k8s.io/scheduler-plugins/apis/scheduling/v1alpha1"
	schedclientset "sigs.k8s.io/scheduler-plugins/pkg/generated/clientset/versioned"
	volcanov1beta1 "volcano.sh/apis/pkg/apis/scheduling/v1beta1"
	volcanoclient "volcano.sh/apis/pkg/client/clientset/versioned"

	kubeflow "github.com/kubeflow/mpi-operator/pkg/apis/kubeflow/v2beta1"
	clientset "github.com/kubeflow/mpi-operator/pkg/client/clientset/versioned"
	"github.com/kubeflow/mpi-operator/pkg/client/clientset/versioned/scheme"
	informers "github.com/kubeflow/mpi-operator/pkg/client/informers/externalversions"
	"github.com/kubeflow/mpi-operator/pkg/controller"
	"github.com/kubeflow/mpi-operator/test/util"
)

func TestMPIJobSuccess(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)
	s := newTestSetup(ctx, t)
	startController(ctx, s.kClient, s.mpiClient, nil)

	mpiJob := &kubeflow.MPIJob{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "job",
			Namespace: s.namespace,
		},
		Spec: kubeflow.MPIJobSpec{
			SlotsPerWorker: ptr.To[int32](1),
			RunPolicy: kubeflow.RunPolicy{
				CleanPodPolicy: ptr.To(kubeflow.CleanPodPolicyRunning),
			},
			MPIReplicaSpecs: map[kubeflow.MPIReplicaType]*kubeflow.ReplicaSpec{
				kubeflow.MPIReplicaTypeLauncher: {
					Template: corev1.PodTemplateSpec{
						Spec: corev1.PodSpec{
							Containers: []corev1.Container{
								{
									Name:  "main",
									Image: "mpi-image",
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
									Name:  "main",
									Image: "mpi-image",
								},
							},
						},
					},
				},
			},
		},
	}
	var err error
	mpiJob, err = s.mpiClient.KubeflowV2beta1().MPIJobs(s.namespace).Create(ctx, mpiJob, metav1.CreateOptions{})
	if err != nil {
		t.Fatalf("Failed sending job to apiserver: %v", err)
	}

	s.events.expect(eventForJob(corev1.Event{
		Type:   corev1.EventTypeNormal,
		Reason: "MPIJobCreated",
	}, mpiJob))

	workerPods, launcherJob := validateMPIJobDependencies(ctx, t, s.kClient, mpiJob, 2, nil)
	mpiJob = validateMPIJobStatus(ctx, t, s.mpiClient, mpiJob, map[kubeflow.MPIReplicaType]*kubeflow.ReplicaStatus{
		kubeflow.MPIReplicaTypeLauncher: {},
		kubeflow.MPIReplicaTypeWorker:   {},
	})
	if !mpiJobHasCondition(mpiJob, kubeflow.JobCreated) {
		t.Errorf("MPIJob missing Created condition")
	}
	s.events.verify(t)

	err = updatePodsToPhase(ctx, s.kClient, workerPods, corev1.PodRunning)
	if err != nil {
		t.Fatalf("Updating worker Pods to Running phase: %v", err)
	}
	validateMPIJobStatus(ctx, t, s.mpiClient, mpiJob, map[kubeflow.MPIReplicaType]*kubeflow.ReplicaStatus{
		kubeflow.MPIReplicaTypeLauncher: {},
		kubeflow.MPIReplicaTypeWorker: {
			Active: 2,
		},
	})

	s.events.expect(eventForJob(corev1.Event{
		Type:   corev1.EventTypeNormal,
		Reason: "MPIJobRunning",
	}, mpiJob))
	launcherPod, err := createPodForJob(ctx, s.kClient, launcherJob)
	if err != nil {
		t.Fatalf("Failed to create mock pod for launcher Job: %v", err)
	}
	err = updatePodsToPhase(ctx, s.kClient, []corev1.Pod{*launcherPod}, corev1.PodRunning)
	if err != nil {
		t.Fatalf("Updating launcher Pods to Running phase: %v", err)
	}
	validateMPIJobStatus(ctx, t, s.mpiClient, mpiJob, map[kubeflow.MPIReplicaType]*kubeflow.ReplicaStatus{
		kubeflow.MPIReplicaTypeLauncher: {
			Active: 1,
		},
		kubeflow.MPIReplicaTypeWorker: {
			Active: 2,
		},
	})
	s.events.verify(t)

	s.events.expect(eventForJob(corev1.Event{
		Type:   corev1.EventTypeNormal,
		Reason: "MPIJobSucceeded",
	}, mpiJob))
	launcherJob.Status.Conditions = append(launcherJob.Status.Conditions, batchv1.JobCondition{
		Type:   batchv1.JobComplete,
		Status: corev1.ConditionTrue,
	})
	launcherJob.Status.Succeeded = 1
	launcherJob.Status.CompletionTime = &metav1.Time{Time: time.Now()}
	_, err = s.kClient.BatchV1().Jobs(launcherJob.Namespace).UpdateStatus(ctx, launcherJob, metav1.UpdateOptions{})
	if err != nil {
		t.Fatalf("Updating launcher Job Complete condition: %v", err)
	}
	validateMPIJobDependencies(ctx, t, s.kClient, mpiJob, 0, nil)
	mpiJob = validateMPIJobStatus(ctx, t, s.mpiClient, mpiJob, map[kubeflow.MPIReplicaType]*kubeflow.ReplicaStatus{
		kubeflow.MPIReplicaTypeLauncher: {
			Succeeded: 1,
		},
		kubeflow.MPIReplicaTypeWorker: {},
	})
	s.events.verify(t)
	if !mpiJobHasCondition(mpiJob, kubeflow.JobSucceeded) {
		t.Errorf("MPIJob doesn't have Succeeded condition after launcher Job succeeded")
	}
}

func TestMPIJobWaitWorkers(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)
	s := newTestSetup(ctx, t)
	startController(ctx, s.kClient, s.mpiClient, nil)

	mpiJob := &kubeflow.MPIJob{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "job",
			Namespace: s.namespace,
		},
		Spec: kubeflow.MPIJobSpec{
			SlotsPerWorker:         ptr.To[int32](1),
			LauncherCreationPolicy: "WaitForWorkersReady",
			RunPolicy: kubeflow.RunPolicy{
				CleanPodPolicy: ptr.To(kubeflow.CleanPodPolicyRunning),
			},
			MPIReplicaSpecs: map[kubeflow.MPIReplicaType]*kubeflow.ReplicaSpec{
				kubeflow.MPIReplicaTypeLauncher: {
					Template: corev1.PodTemplateSpec{
						Spec: corev1.PodSpec{
							Containers: []corev1.Container{
								{
									Name:  "main",
									Image: "mpi-image",
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
									Name:  "main",
									Image: "mpi-image",
								},
							},
						},
					},
				},
			},
		},
	}
	var err error
	mpiJob, err = s.mpiClient.KubeflowV2beta1().MPIJobs(s.namespace).Create(ctx, mpiJob, metav1.CreateOptions{})
	if err != nil {
		t.Fatalf("Failed sending job to apiserver: %v", err)
	}

	s.events.expect(eventForJob(corev1.Event{
		Type:   corev1.EventTypeNormal,
		Reason: "MPIJobCreated",
	}, mpiJob))

	mpiJob = validateMPIJobStatus(ctx, t, s.mpiClient, mpiJob, map[kubeflow.MPIReplicaType]*kubeflow.ReplicaStatus{
		kubeflow.MPIReplicaTypeWorker: {},
	})
	if !mpiJobHasCondition(mpiJob, kubeflow.JobCreated) {
		t.Errorf("MPIJob missing Created condition")
	}
	s.events.verify(t)

	workerPods, err := getPodsForJob(ctx, s.kClient, mpiJob)
	if err != nil {
		t.Fatalf("Cannot get worker pods from job: %v", err)
	}

	err = updatePodsToPhase(ctx, s.kClient, workerPods, corev1.PodRunning)
	if err != nil {
		t.Fatalf("Updating worker Pods to Running phase: %v", err)
	}

	// No launcher here, workers are running, but not ready yet
	validateMPIJobStatus(ctx, t, s.mpiClient, mpiJob, map[kubeflow.MPIReplicaType]*kubeflow.ReplicaStatus{
		kubeflow.MPIReplicaTypeWorker: {
			Active: 2,
		},
	})

	_, err = s.kClient.BatchV1().Jobs(s.namespace).Get(ctx, "job-launcher", metav1.GetOptions{})
	if !apierrors.IsNotFound(err) {
		t.Fatalf("Launcher is running before workers")
	}

	err = updatePodsCondition(ctx, s.kClient, workerPods, corev1.PodCondition{
		Type:   corev1.PodReady,
		Status: corev1.ConditionTrue,
	})
	if err != nil {
		t.Fatalf("Updating worker Pods to Ready: %v", err)
	}

	validateMPIJobStatus(ctx, t, s.mpiClient, mpiJob, map[kubeflow.MPIReplicaType]*kubeflow.ReplicaStatus{
		kubeflow.MPIReplicaTypeLauncher: {},
		kubeflow.MPIReplicaTypeWorker: {
			Active: 2,
		},
	})

	s.events.expect(eventForJob(corev1.Event{
		Type:   corev1.EventTypeNormal,
		Reason: "MPIJobRunning",
	}, mpiJob))

	launcherJob, err := getLauncherJobForMPIJob(ctx, s.kClient, mpiJob)
	if err != nil {
		t.Fatalf("Cannot get launcher job from job: %v", err)
	}

	launcherPod, err := createPodForJob(ctx, s.kClient, launcherJob)
	if err != nil {
		t.Fatalf("Failed to create mock pod for launcher Job: %v", err)
	}
	err = updatePodsToPhase(ctx, s.kClient, []corev1.Pod{*launcherPod}, corev1.PodRunning)
	if err != nil {
		t.Fatalf("Updating launcher Pods to Running phase: %v", err)
	}
	validateMPIJobStatus(ctx, t, s.mpiClient, mpiJob, map[kubeflow.MPIReplicaType]*kubeflow.ReplicaStatus{
		kubeflow.MPIReplicaTypeLauncher: {
			Active: 1,
		},
		kubeflow.MPIReplicaTypeWorker: {
			Active: 2,
		},
	})
	s.events.verify(t)
}

func TestMPIJobResumingAndSuspending(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)
	s := newTestSetup(ctx, t)
	startController(ctx, s.kClient, s.mpiClient, nil)

	mpiJob := &kubeflow.MPIJob{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "job",
			Namespace: s.namespace,
		},
		Spec: kubeflow.MPIJobSpec{
			SlotsPerWorker: ptr.To[int32](1),
			RunPolicy: kubeflow.RunPolicy{
				CleanPodPolicy: ptr.To(kubeflow.CleanPodPolicyRunning),
				Suspend:        ptr.To(true),
			},
			MPIReplicaSpecs: map[kubeflow.MPIReplicaType]*kubeflow.ReplicaSpec{
				kubeflow.MPIReplicaTypeLauncher: {
					Template: corev1.PodTemplateSpec{
						Spec: corev1.PodSpec{
							Containers: []corev1.Container{
								{
									Name:  "main",
									Image: "mpi-image",
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
									Name:  "main",
									Image: "mpi-image",
								},
							},
						},
					},
				},
			},
		},
	}
	// 1. Create suspended MPIJob
	var err error
	mpiJob, err = s.mpiClient.KubeflowV2beta1().MPIJobs(s.namespace).Create(ctx, mpiJob, metav1.CreateOptions{})
	if err != nil {
		t.Fatalf("Failed sending job to apiserver: %v", err)
	}
	s.events.expect(eventForJob(corev1.Event{
		Type:   corev1.EventTypeNormal,
		Reason: "MPIJobCreated",
	}, mpiJob))

	s.events.expect(eventForJob(corev1.Event{
		Type:   corev1.EventTypeNormal,
		Reason: "MPIJobSuspended",
	}, mpiJob))

	_, launcherJob := validateMPIJobDependencies(ctx, t, s.kClient, mpiJob, 0, nil)
	mpiJob = validateMPIJobStatus(ctx, t, s.mpiClient, mpiJob, map[kubeflow.MPIReplicaType]*kubeflow.ReplicaStatus{
		kubeflow.MPIReplicaTypeLauncher: {},
		kubeflow.MPIReplicaTypeWorker:   {},
	})
	if !mpiJobHasCondition(mpiJob, kubeflow.JobCreated) {
		t.Errorf("MPIJob missing Created condition")
	}
	if !mpiJobHasCondition(mpiJob, kubeflow.JobSuspended) {
		t.Errorf("MPIJob missing Suspended condition")
	}
	if !isJobSuspended(launcherJob) {
		t.Errorf("LauncherJob is suspended")
	}
	if mpiJob.Status.StartTime != nil {
		t.Errorf("MPIJob has unexpected start time: %v", mpiJob.Status.StartTime)
	}

	s.events.verify(t)

	// 2. Resume the MPIJob
	mpiJob.Spec.RunPolicy.Suspend = ptr.To(false)
	mpiJob, err = s.mpiClient.KubeflowV2beta1().MPIJobs(mpiJob.Namespace).Update(ctx, mpiJob, metav1.UpdateOptions{})
	if err != nil {
		t.Fatalf("Failed to resume the MPIJob: %v", err)
	}
	s.events.expect(eventForJob(corev1.Event{
		Type:   corev1.EventTypeNormal,
		Reason: "MPIJobResumed",
	}, mpiJob))

	workerPods, launcherJob := validateMPIJobDependencies(ctx, t, s.kClient, mpiJob, 2, nil)

	mpiJob = validateMPIJobStatus(ctx, t, s.mpiClient, mpiJob, map[kubeflow.MPIReplicaType]*kubeflow.ReplicaStatus{
		kubeflow.MPIReplicaTypeLauncher: {},
		kubeflow.MPIReplicaTypeWorker:   {},
	})
	if mpiJob.Status.StartTime == nil {
		t.Errorf("MPIJob is missing startTime")
	}
	if isJobSuspended(launcherJob) {
		t.Errorf("LauncherJob is suspended")
	}
	if !mpiJobHasConditionWithStatus(mpiJob, kubeflow.JobSuspended, corev1.ConditionFalse) {
		t.Errorf("MPIJob has unexpected Suspended condition")
	}

	s.events.verify(t)

	// 3. Set the pods to be running
	err = updatePodsToPhase(ctx, s.kClient, workerPods, corev1.PodRunning)
	if err != nil {
		t.Fatalf("Updating worker Pods to Running phase: %v", err)
	}
	s.events.expect(eventForJob(corev1.Event{
		Type:   corev1.EventTypeNormal,
		Reason: "MPIJobRunning",
	}, mpiJob))
	launcherPod, err := createPodForJob(ctx, s.kClient, launcherJob)
	if err != nil {
		t.Fatalf("Failed to create mock pod for launcher Job: %v", err)
	}
	err = updatePodsToPhase(ctx, s.kClient, []corev1.Pod{*launcherPod}, corev1.PodRunning)
	if err != nil {
		t.Fatalf("Updating launcher Pods to Running phase: %v", err)
	}
	mpiJob = validateMPIJobStatus(ctx, t, s.mpiClient, mpiJob, map[kubeflow.MPIReplicaType]*kubeflow.ReplicaStatus{
		kubeflow.MPIReplicaTypeLauncher: {
			Active: 1,
		},
		kubeflow.MPIReplicaTypeWorker: {
			Active: 2,
		},
	})
	s.events.verify(t)

	// 4. Suspend the running MPIJob
	mpiJob.Spec.RunPolicy.Suspend = ptr.To(true)
	mpiJob, err = s.mpiClient.KubeflowV2beta1().MPIJobs(mpiJob.Namespace).Update(ctx, mpiJob, metav1.UpdateOptions{})
	if err != nil {
		t.Fatalf("Failed to suspend the MPIJob: %v", err)
	}
	err = s.kClient.CoreV1().Pods(launcherPod.Namespace).Delete(ctx, launcherPod.Name, metav1.DeleteOptions{})
	if err != nil {
		t.Fatalf("Failed to delete mock pod for launcher Job: %v", err)
	}
	_, launcherJob = validateMPIJobDependencies(ctx, t, s.kClient, mpiJob, 0, nil)
	mpiJob = validateMPIJobStatus(ctx, t, s.mpiClient, mpiJob, map[kubeflow.MPIReplicaType]*kubeflow.ReplicaStatus{
		kubeflow.MPIReplicaTypeLauncher: {},
		kubeflow.MPIReplicaTypeWorker:   {},
	})
	if !isJobSuspended(launcherJob) {
		t.Errorf("LauncherJob is not suspended")
	}
	if !mpiJobHasCondition(mpiJob, kubeflow.JobSuspended) {
		t.Errorf("MPIJob missing Suspended condition")
	}
	if !mpiJobHasConditionWithStatus(mpiJob, kubeflow.JobRunning, corev1.ConditionFalse) {
		t.Errorf("MPIJob has unexpected Running condition")
	}
}

func TestMPIJobFailure(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)
	s := newTestSetup(ctx, t)
	startController(ctx, s.kClient, s.mpiClient, nil)

	mpiJob := &kubeflow.MPIJob{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "job",
			Namespace: s.namespace,
		},
		Spec: kubeflow.MPIJobSpec{
			SlotsPerWorker: ptr.To[int32](1),
			RunPolicy: kubeflow.RunPolicy{
				CleanPodPolicy: ptr.To(kubeflow.CleanPodPolicyRunning),
			},
			MPIReplicaSpecs: map[kubeflow.MPIReplicaType]*kubeflow.ReplicaSpec{
				kubeflow.MPIReplicaTypeLauncher: {
					Template: corev1.PodTemplateSpec{
						Spec: corev1.PodSpec{
							Containers: []corev1.Container{
								{
									Name:  "main",
									Image: "mpi-image",
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
									Name:  "main",
									Image: "mpi-image",
								},
							},
						},
					},
				},
			},
		},
	}

	var err error
	mpiJob, err = s.mpiClient.KubeflowV2beta1().MPIJobs(s.namespace).Create(ctx, mpiJob, metav1.CreateOptions{})
	if err != nil {
		t.Fatalf("Failed sending job to apiserver: %v", err)
	}

	workerPods, launcherJob := validateMPIJobDependencies(ctx, t, s.kClient, mpiJob, 2, nil)

	s.events.expect(eventForJob(corev1.Event{
		Type:   corev1.EventTypeNormal,
		Reason: "MPIJobRunning",
	}, mpiJob))
	err = updatePodsToPhase(ctx, s.kClient, workerPods, corev1.PodRunning)
	if err != nil {
		t.Fatalf("Updating worker Pods to Running phase: %v", err)
	}
	launcherPod, err := createPodForJob(ctx, s.kClient, launcherJob)
	if err != nil {
		t.Fatalf("Failed to create mock pod for launcher Job: %v", err)
	}
	err = updatePodsToPhase(ctx, s.kClient, []corev1.Pod{*launcherPod}, corev1.PodRunning)
	if err != nil {
		t.Fatalf("Updating launcher Pods to Running phase: %v", err)
	}
	s.events.verify(t)

	// A failed launcher Pod doesn't mark MPIJob as failed.
	launcherPod, err = s.kClient.CoreV1().Pods(launcherPod.Namespace).Get(ctx, launcherPod.Name, metav1.GetOptions{})
	if err != nil {
		t.Fatalf("Failed obtaining updated launcher Pod: %v", err)
	}
	err = updatePodsToPhase(ctx, s.kClient, []corev1.Pod{*launcherPod}, corev1.PodFailed)
	if err != nil {
		t.Fatalf("Failed to update launcher Pod to Running phase: %v", err)
	}
	launcherJob.Status.Failed = 1
	launcherJob, err = s.kClient.BatchV1().Jobs(launcherPod.Namespace).UpdateStatus(ctx, launcherJob, metav1.UpdateOptions{})
	if err != nil {
		t.Fatalf("Failed to update launcher Job failed pods: %v", err)
	}
	mpiJob = validateMPIJobStatus(ctx, t, s.mpiClient, mpiJob, map[kubeflow.MPIReplicaType]*kubeflow.ReplicaStatus{
		kubeflow.MPIReplicaTypeLauncher: {
			Failed: 1,
		},
		kubeflow.MPIReplicaTypeWorker: {
			Active: 2,
		},
	})
	if mpiJobHasCondition(mpiJob, kubeflow.JobFailed) {
		t.Errorf("MPIJob has Failed condition when a launcher Pod fails")
	}

	s.events.expect(eventForJob(corev1.Event{
		Type:   corev1.EventTypeWarning,
		Reason: "MPIJobFailed",
	}, mpiJob))
	launcherJob.Status.Conditions = append(launcherJob.Status.Conditions, batchv1.JobCondition{
		Type:   batchv1.JobFailed,
		Status: corev1.ConditionTrue,
	})
	launcherJob.Status.Failed = 2
	_, err = s.kClient.BatchV1().Jobs(launcherJob.Namespace).UpdateStatus(ctx, launcherJob, metav1.UpdateOptions{})
	if err != nil {
		t.Fatalf("Updating launcher Job Failed condition: %v", err)
	}
	validateMPIJobDependencies(ctx, t, s.kClient, mpiJob, 0, nil)
	mpiJob = validateMPIJobStatus(ctx, t, s.mpiClient, mpiJob, map[kubeflow.MPIReplicaType]*kubeflow.ReplicaStatus{
		kubeflow.MPIReplicaTypeLauncher: {
			Failed: 2,
		},
		kubeflow.MPIReplicaTypeWorker: {},
	})
	s.events.verify(t)
	if !mpiJobHasCondition(mpiJob, kubeflow.JobFailed) {
		t.Errorf("MPIJob doesn't have Failed condition after launcher Job fails")
	}
}

func TestMPIJobWithSchedulerPlugins(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)
	s := newTestSetup(ctx, t)
	gangSchedulerCfg := &gangSchedulerConfig{
		schedulerName: "default-scheduler",
		schedClient:   s.gangSchedulerCfg.schedClient,
	}
	startController(ctx, s.kClient, s.mpiClient, gangSchedulerCfg)

	mpiJob := &kubeflow.MPIJob{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "job",
			Namespace: s.namespace,
		},
		Spec: kubeflow.MPIJobSpec{
			SlotsPerWorker: ptr.To[int32](1),
			RunPolicy: kubeflow.RunPolicy{
				CleanPodPolicy: ptr.To(kubeflow.CleanPodPolicyRunning),
				SchedulingPolicy: &kubeflow.SchedulingPolicy{
					ScheduleTimeoutSeconds: ptr.To[int32](900),
				},
			},
			MPIReplicaSpecs: map[kubeflow.MPIReplicaType]*kubeflow.ReplicaSpec{
				kubeflow.MPIReplicaTypeLauncher: {
					Template: corev1.PodTemplateSpec{
						Spec: corev1.PodSpec{
							PriorityClassName: "test-pc",
							Containers: []corev1.Container{
								{
									Name:  "main",
									Image: "mpi-image",
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
									Name:  "main",
									Image: "mpi-image",
								},
							},
						},
					},
				},
			},
		},
	}
	priorityClass := &schedulingv1.PriorityClass{
		TypeMeta: metav1.TypeMeta{
			APIVersion: schedulingv1.SchemeGroupVersion.String(),
			Kind:       "PriorityClass",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name: "test-pc",
		},
		Value: 100_000,
	}
	// 1. Create PriorityClass
	var err error
	_, err = s.kClient.SchedulingV1().PriorityClasses().Create(ctx, priorityClass, metav1.CreateOptions{})
	if err != nil {
		t.Fatalf("Failed sending priorityClass to apiserver: %v", err)
	}

	// 2. Create MPIJob
	mpiJob, err = s.mpiClient.KubeflowV2beta1().MPIJobs(s.namespace).Create(ctx, mpiJob, metav1.CreateOptions{})
	if err != nil {
		t.Fatalf("Failed sending job to apiserver: %v", err)
	}

	s.events.expect(eventForJob(corev1.Event{
		Type:   corev1.EventTypeNormal,
		Reason: "MPIJobCreated",
	}, mpiJob))

	validateMPIJobDependencies(ctx, t, s.kClient, mpiJob, 2, gangSchedulerCfg)
	mpiJob = validateMPIJobStatus(ctx, t, s.mpiClient, mpiJob, map[kubeflow.MPIReplicaType]*kubeflow.ReplicaStatus{
		kubeflow.MPIReplicaTypeLauncher: {},
		kubeflow.MPIReplicaTypeWorker:   {},
	})
	if !mpiJobHasCondition(mpiJob, kubeflow.JobCreated) {
		t.Errorf("MPIJob missing Created condition")
	}
	s.events.verify(t)

	// 3. Update SchedulingPolicy of MPIJob
	updatedScheduleTimeSeconds := int32(10)
	mpiJob.Spec.RunPolicy.SchedulingPolicy.ScheduleTimeoutSeconds = &updatedScheduleTimeSeconds
	mpiJob, err = s.mpiClient.KubeflowV2beta1().MPIJobs(s.namespace).Update(ctx, mpiJob, metav1.UpdateOptions{})
	if err != nil {
		t.Errorf("Failed sending job to apiserver: %v", err)
	}
	if err = wait.PollUntilContextTimeout(ctx, util.WaitInterval, wait.ForeverTestTimeout, false, func(ctx context.Context) (bool, error) {
		pg, err := getSchedPodGroup(ctx, gangSchedulerCfg.schedClient, mpiJob)
		if err != nil {
			return false, err
		}
		if pg == nil {
			return false, nil
		}
		return *pg.Spec.ScheduleTimeoutSeconds == updatedScheduleTimeSeconds, nil
	}); err != nil {
		t.Errorf("Failed updating scheduler-plugins PodGroup: %v", err)
	}
}

func TestMPIJobWithVolcanoScheduler(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)
	s := newTestSetup(ctx, t)
	gangSchedulerCfg := &gangSchedulerConfig{
		schedulerName: "volcano",
		volcanoClient: s.gangSchedulerCfg.volcanoClient,
	}
	startController(ctx, s.kClient, s.mpiClient, gangSchedulerCfg)

	prioClass := "test-pc-volcano"
	mpiJob := &kubeflow.MPIJob{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "job-by-volcano",
			Namespace: s.namespace,
		},
		Spec: kubeflow.MPIJobSpec{
			SlotsPerWorker: ptr.To[int32](1),
			RunPolicy: kubeflow.RunPolicy{
				CleanPodPolicy: ptr.To(kubeflow.CleanPodPolicyRunning),
				SchedulingPolicy: &kubeflow.SchedulingPolicy{
					Queue:         "default",
					PriorityClass: prioClass,
				},
			},
			MPIReplicaSpecs: map[kubeflow.MPIReplicaType]*kubeflow.ReplicaSpec{
				kubeflow.MPIReplicaTypeLauncher: {
					Template: corev1.PodTemplateSpec{
						Spec: corev1.PodSpec{
							PriorityClassName: prioClass,
							Containers: []corev1.Container{
								{
									Name:  "main",
									Image: "mpi-image",
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
									Name:  "main",
									Image: "mpi-image",
								},
							},
						},
					},
				},
			},
		},
	}
	priorityClass := &schedulingv1.PriorityClass{
		TypeMeta: metav1.TypeMeta{
			APIVersion: schedulingv1.SchemeGroupVersion.String(),
			Kind:       "PriorityClass",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name: prioClass,
		},
		Value: 100_000,
	}
	// 1. Create PriorityClass
	var err error
	_, err = s.kClient.SchedulingV1().PriorityClasses().Create(ctx, priorityClass, metav1.CreateOptions{})
	if err != nil {
		t.Fatalf("Failed sending priorityClass to apiserver: %v", err)
	}

	// 2. Create MPIJob
	mpiJob, err = s.mpiClient.KubeflowV2beta1().MPIJobs(s.namespace).Create(ctx, mpiJob, metav1.CreateOptions{})
	if err != nil {
		t.Fatalf("Failed sending job to apiserver: %v", err)
	}

	s.events.expect(eventForJob(corev1.Event{
		Type:   corev1.EventTypeNormal,
		Reason: "MPIJobCreated",
	}, mpiJob))

	validateMPIJobDependencies(ctx, t, s.kClient, mpiJob, 2, gangSchedulerCfg)
	mpiJob = validateMPIJobStatus(ctx, t, s.mpiClient, mpiJob, map[kubeflow.MPIReplicaType]*kubeflow.ReplicaStatus{
		kubeflow.MPIReplicaTypeLauncher: {},
		kubeflow.MPIReplicaTypeWorker:   {},
	})
	if !mpiJobHasCondition(mpiJob, kubeflow.JobCreated) {
		t.Errorf("MPIJob missing Created condition")
	}
	s.events.verify(t)

	// 3. Update SchedulingPolicy of MPIJob
	updatedMinAvaiable := int32(2)
	updatedQueueName := "queue-for-mpijob"
	mpiJob.Spec.RunPolicy.SchedulingPolicy.MinAvailable = &updatedMinAvaiable
	mpiJob.Spec.RunPolicy.SchedulingPolicy.Queue = updatedQueueName
	mpiJob, err = s.mpiClient.KubeflowV2beta1().MPIJobs(s.namespace).Update(ctx, mpiJob, metav1.UpdateOptions{})
	if err != nil {
		t.Errorf("Failed sending job to apiserver: %v", err)
	}
	if err = wait.PollUntilContextTimeout(ctx, util.WaitInterval, wait.ForeverTestTimeout, false, func(ctx context.Context) (bool, error) {
		pg, err := getVolcanoPodGroup(ctx, gangSchedulerCfg.volcanoClient, mpiJob)
		if err != nil {
			return false, err
		}
		if pg == nil {
			return false, nil
		}
		return pg.Spec.MinMember == updatedMinAvaiable && pg.Spec.Queue == updatedQueueName, nil
	}); err != nil {
		t.Errorf("Failed updating volcano PodGroup: %v", err)
	}
}

func TestMPIJobManagedExternally(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)
	s := newTestSetup(ctx, t)
	startController(ctx, s.kClient, s.mpiClient, nil)

	mpiJob := &kubeflow.MPIJob{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "job",
			Namespace: s.namespace,
		},
		Spec: kubeflow.MPIJobSpec{
			SlotsPerWorker: ptr.To[int32](1),
			RunPolicy: kubeflow.RunPolicy{
				CleanPodPolicy: ptr.To(kubeflow.CleanPodPolicyRunning),
				ManagedBy:      ptr.To(kubeflow.MultiKueueController),
			},
			MPIReplicaSpecs: map[kubeflow.MPIReplicaType]*kubeflow.ReplicaSpec{
				kubeflow.MPIReplicaTypeLauncher: {
					Template: corev1.PodTemplateSpec{
						Spec: corev1.PodSpec{
							Containers: []corev1.Container{
								{
									Name:  "main",
									Image: "mpi-image",
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
									Name:  "main",
									Image: "mpi-image",
								},
							},
						},
					},
				},
			},
		},
	}

	// 1. The job must be created
	var err error
	mpiJob, err = s.mpiClient.KubeflowV2beta1().MPIJobs(s.namespace).Create(ctx, mpiJob, metav1.CreateOptions{})
	if err != nil {
		t.Fatalf("Failed sending job to apiserver: %v", err)
	}

	time.Sleep(util.SleepDurationControllerSyncDelay)
	// 2. Status is not getting updated
	mpiJob = validateMPIJobStatus(ctx, t, s.mpiClient, mpiJob, nil)
	if mpiJob.Status.StartTime != nil {
		t.Errorf("MPIJob should be missing startTime")
	}
	// 3. There should be no conditions, even the one for create
	if mpiJobHasCondition(mpiJob, kubeflow.JobCreated) {
		t.Errorf("MPIJob shouldn't have any condition")
	}
	// 4. No Jobs or Services created
	lp, err := getLauncherJobForMPIJob(ctx, s.kClient, mpiJob)
	if err != nil {
		t.Fatalf("Failed getting launcher jobs: %v", err)
	}
	if lp != nil {
		t.Fatalf("There should be no launcher jobs from job: %v", lp)
	}
	svcs, err := getServiceForJob(ctx, s.kClient, mpiJob)
	if err != nil {
		t.Fatalf("Failed getting services for the job: %v", err)
	}
	if svcs != nil {
		t.Fatalf("There should be no services from job: %v", svcs)
	}

}

func startController(
	ctx context.Context,
	kClient kubernetes.Interface,
	mpiClient clientset.Interface,
	gangSchedulerCfg *gangSchedulerConfig,
) {
	kubeInformerFactory := kubeinformers.NewSharedInformerFactory(kClient, 0)
	mpiInformerFactory := informers.NewSharedInformerFactory(mpiClient, 0)
	workqueueRateLimiter := workqueue.DefaultTypedControllerRateLimiter[any]()
	var (
		volcanoClient volcanoclient.Interface
		schedClient   schedclientset.Interface
		schedulerName string
	)
	if gangSchedulerCfg != nil {
		schedulerName = gangSchedulerCfg.schedulerName
		if gangSchedulerCfg.volcanoClient != nil {
			volcanoClient = gangSchedulerCfg.volcanoClient
		} else if gangSchedulerCfg.schedClient != nil {
			schedClient = gangSchedulerCfg.schedClient
		}
	}
	ctrl, err := controller.NewMPIJobController(
		kClient,
		mpiClient,
		volcanoClient,
		schedClient,
		kubeInformerFactory.Core().V1().ConfigMaps(),
		kubeInformerFactory.Core().V1().Secrets(),
		kubeInformerFactory.Core().V1().Services(),
		kubeInformerFactory.Batch().V1().Jobs(),
		kubeInformerFactory.Core().V1().Pods(),
		kubeInformerFactory.Scheduling().V1().PriorityClasses(),
		mpiInformerFactory.Kubeflow().V2beta1().MPIJobs(),
		metav1.NamespaceAll, schedulerName,
		workqueueRateLimiter,
	)
	if err != nil {
		panic(err)
	}

	go kubeInformerFactory.Start(ctx.Done())
	go mpiInformerFactory.Start(ctx.Done())
	if ctrl.PodGroupCtrl != nil {
		ctrl.PodGroupCtrl.StartInformerFactory(ctx.Done())
	}

	go func() {
		if err := ctrl.Run(1, ctx.Done()); err != nil {
			panic(err)
		}
	}()
}

func validateMPIJobDependencies(
	ctx context.Context,
	t *testing.T,
	kubeClient kubernetes.Interface,
	job *kubeflow.MPIJob,
	workers int,
	gangSchedulingCfg *gangSchedulerConfig,
) ([]corev1.Pod, *batchv1.Job) {
	t.Helper()
	var (
		svc         *corev1.Service
		cfgMap      *corev1.ConfigMap
		secret      *corev1.Secret
		workerPods  []corev1.Pod
		launcherJob *batchv1.Job
		podGroup    metav1.Object
	)
	var problems []string
	if err := wait.PollUntilContextTimeout(ctx, util.WaitInterval, wait.ForeverTestTimeout, false, func(ctx context.Context) (bool, error) {
		problems = nil
		var err error
		svc, err = getServiceForJob(ctx, kubeClient, job)
		if err != nil {
			return false, err
		}
		if svc == nil {
			problems = append(problems, "Service not found")
		}
		cfgMap, err = getConfigMapForJob(ctx, kubeClient, job)
		if err != nil {
			return false, err
		}
		if cfgMap == nil {
			problems = append(problems, "ConfigMap not found")
		}
		secret, err = getSecretForJob(ctx, kubeClient, job)
		if err != nil {
			return false, err
		}
		if secret == nil {
			problems = append(problems, "Secret not found")
		}
		workerPods, err = getPodsForJob(ctx, kubeClient, job)
		if err != nil {
			return false, err
		}
		if workers != len(workerPods) {
			problems = append(problems, fmt.Sprintf("got %d workers, want %d", len(workerPods), workers))
		}
		launcherJob, err = getLauncherJobForMPIJob(ctx, kubeClient, job)
		if err != nil {
			return false, err
		}
		if launcherJob == nil {
			problems = append(problems, "Launcher Job not found")
		}
		if cfg := gangSchedulingCfg; cfg != nil {
			if cfg.schedulerName == "volcano" {
				podGroup, err = getVolcanoPodGroup(ctx, cfg.volcanoClient, job)
				if err != nil {
					return false, err
				}
				if podGroup == nil {
					problems = append(problems, "Volcano PodGroup not found")
				}
			} else if len(cfg.schedulerName) != 0 {
				podGroup, err = getSchedPodGroup(ctx, cfg.schedClient, job)
				if err != nil {
					return false, err
				}
				if podGroup == nil {
					problems = append(problems, "Scheduler Plugins PodGroup not found")
				}
			}
		}

		if len(problems) == 0 {
			return true, nil
		}
		return false, nil
	}); err != nil {
		for _, p := range problems {
			t.Error(p)
		}
		t.Fatalf("Waiting for job dependencies: %v", err)
	}
	svcSelector, err := labels.ValidatedSelectorFromSet(svc.Spec.Selector)
	if err != nil {
		t.Fatalf("Invalid workers Service selector: %v", err)
	}
	for _, p := range workerPods {
		if !svcSelector.Matches(labels.Set(p.Labels)) {
			t.Errorf("Workers Service selector doesn't match pod %s", p.Name)
		}
		if !hasVolumeForSecret(&p.Spec, secret) {
			t.Errorf("Secret %s not mounted in Pod %s", secret.Name, p.Name)
		}
	}
	if !hasVolumeForSecret(&launcherJob.Spec.Template.Spec, secret) {
		t.Errorf("Secret %s not mounted in launcher Job", secret.Name)
	}
	if !hasVolumeForConfigMap(&launcherJob.Spec.Template.Spec, cfgMap) {
		t.Errorf("ConfigMap %s not mounted in launcher Job", secret.Name)
	}
	return workerPods, launcherJob
}

func validateMPIJobStatus(ctx context.Context, t *testing.T, client clientset.Interface, job *kubeflow.MPIJob, want map[kubeflow.MPIReplicaType]*kubeflow.ReplicaStatus) *kubeflow.MPIJob {
	t.Helper()
	var (
		newJob *kubeflow.MPIJob
		err    error
		got    map[kubeflow.MPIReplicaType]*kubeflow.ReplicaStatus
	)
	if err := wait.PollUntilContextTimeout(ctx, util.WaitInterval, wait.ForeverTestTimeout, false, func(ctx context.Context) (bool, error) {
		newJob, err = client.KubeflowV2beta1().MPIJobs(job.Namespace).Get(ctx, job.Name, metav1.GetOptions{})
		if err != nil {
			return false, err
		}
		got = newJob.Status.ReplicaStatuses
		return cmp.Equal(want, got), nil
	}); err != nil {
		diff := cmp.Diff(want, got)
		t.Fatalf("Waiting for Job status: %v\n(-want,+got)\n%s", err, diff)
	}
	return newJob
}

func updatePodsToPhase(ctx context.Context, client kubernetes.Interface, pods []corev1.Pod, phase corev1.PodPhase) error {
	for i, p := range pods {
		p.Status.Phase = phase
		newPod, err := client.CoreV1().Pods(p.Namespace).UpdateStatus(ctx, &p, metav1.UpdateOptions{})
		if err != nil {
			return err
		}
		pods[i] = *newPod
	}
	return nil
}

func updatePodsCondition(ctx context.Context, client kubernetes.Interface, pods []corev1.Pod, condition corev1.PodCondition) error {
	for i, p := range pods {
		p.Status.Conditions = append(p.Status.Conditions, condition)
		newPod, err := client.CoreV1().Pods(p.Namespace).UpdateStatus(ctx, &p, metav1.UpdateOptions{})
		if err != nil {
			return err
		}
		pods[i] = *newPod
	}
	return nil
}

func getServiceForJob(ctx context.Context, client kubernetes.Interface, job *kubeflow.MPIJob) (*corev1.Service, error) {
	result, err := client.CoreV1().Services(job.Namespace).List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, err
	}
	for _, obj := range result.Items {
		if metav1.IsControlledBy(&obj, job) {
			return &obj, nil
		}
	}
	return nil, nil
}

func getConfigMapForJob(ctx context.Context, client kubernetes.Interface, job *kubeflow.MPIJob) (*corev1.ConfigMap, error) {
	result, err := client.CoreV1().ConfigMaps(job.Namespace).List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, err
	}
	for _, obj := range result.Items {
		if metav1.IsControlledBy(&obj, job) {
			return &obj, nil
		}
	}
	return nil, nil
}

func getSecretForJob(ctx context.Context, client kubernetes.Interface, job *kubeflow.MPIJob) (*corev1.Secret, error) {
	result, err := client.CoreV1().Secrets(job.Namespace).List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, err
	}
	for _, obj := range result.Items {
		if metav1.IsControlledBy(&obj, job) {
			return &obj, nil
		}
	}
	return nil, nil
}

func getPodsForJob(ctx context.Context, client kubernetes.Interface, job *kubeflow.MPIJob) ([]corev1.Pod, error) {
	result, err := client.CoreV1().Pods(job.Namespace).List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, err
	}
	var pods []corev1.Pod
	for _, p := range result.Items {
		if p.DeletionTimestamp == nil && metav1.IsControlledBy(&p, job) {
			pods = append(pods, p)
		}
	}
	return pods, nil
}

func getLauncherJobForMPIJob(ctx context.Context, client kubernetes.Interface, mpiJob *kubeflow.MPIJob) (*batchv1.Job, error) {
	result, err := client.BatchV1().Jobs(mpiJob.Namespace).List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, err
	}
	for _, j := range result.Items {
		if metav1.IsControlledBy(&j, mpiJob) {
			return &j, nil
		}
	}
	return nil, nil
}

func getSchedPodGroup(ctx context.Context, client schedclientset.Interface, job *kubeflow.MPIJob) (*schedv1alpha1.PodGroup, error) {
	result, err := client.SchedulingV1alpha1().PodGroups(job.Namespace).List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, err
	}
	for _, pg := range result.Items {
		if metav1.IsControlledBy(&pg, job) {
			return &pg, nil
		}
	}
	return nil, nil
}

func getVolcanoPodGroup(ctx context.Context, client volcanoclient.Interface, job *kubeflow.MPIJob) (*volcanov1beta1.PodGroup, error) {
	result, err := client.SchedulingV1beta1().PodGroups(job.Namespace).List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, err
	}
	for _, pg := range result.Items {
		if metav1.IsControlledBy(&pg, job) {
			return &pg, nil
		}
	}
	return nil, nil
}

func createPodForJob(ctx context.Context, client kubernetes.Interface, job *batchv1.Job) (*corev1.Pod, error) {
	pod := &corev1.Pod{
		ObjectMeta: job.Spec.Template.ObjectMeta,
		Spec:       job.Spec.Template.Spec,
	}
	pod.GenerateName = job.Name + "-"
	pod.Namespace = job.Namespace
	pod.OwnerReferences = []metav1.OwnerReference{
		*metav1.NewControllerRef(job, batchv1.SchemeGroupVersion.WithKind("Job")),
	}
	for k, v := range job.Spec.Selector.MatchLabels {
		pod.Labels[k] = v
	}
	return client.CoreV1().Pods(job.Namespace).Create(ctx, pod, metav1.CreateOptions{})
}

func hasVolumeForSecret(podSpec *corev1.PodSpec, secret *corev1.Secret) bool {
	for _, v := range podSpec.Volumes {
		if v.Secret != nil && v.Secret.SecretName == secret.Name {
			return true
		}
	}
	return false
}

func hasVolumeForConfigMap(podSpec *corev1.PodSpec, cm *corev1.ConfigMap) bool {
	for _, v := range podSpec.Volumes {
		if v.ConfigMap != nil && v.ConfigMap.Name == cm.Name {
			return true
		}
	}
	return false
}

func mpiJobHasCondition(job *kubeflow.MPIJob, cond kubeflow.JobConditionType) bool {
	return mpiJobHasConditionWithStatus(job, cond, corev1.ConditionTrue)
}

func mpiJobHasConditionWithStatus(job *kubeflow.MPIJob, cond kubeflow.JobConditionType, status corev1.ConditionStatus) bool {
	for _, c := range job.Status.Conditions {
		if c.Type == cond && c.Status == status {
			return true
		}
	}
	return false
}

func isJobSuspended(job *batchv1.Job) bool {
	return ptr.Deref(job.Spec.Suspend, false)
}

func eventForJob(event corev1.Event, job *kubeflow.MPIJob) corev1.Event {
	event.Namespace = job.Namespace
	event.Source.Component = "mpi-job-controller"
	event.ReportingController = "mpi-job-controller"
	ref, err := reference.GetReference(scheme.Scheme, job)
	runtime.Must(err)
	event.InvolvedObject = *ref
	return event
}
