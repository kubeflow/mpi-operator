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
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/apimachinery/pkg/util/wait"
	kubeinformers "k8s.io/client-go/informers"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/reference"

	common "github.com/kubeflow/common/pkg/apis/common/v1"
	kubeflow "github.com/kubeflow/mpi-operator/v2/pkg/apis/kubeflow/v2beta1"
	clientset "github.com/kubeflow/mpi-operator/v2/pkg/client/clientset/versioned"
	"github.com/kubeflow/mpi-operator/v2/pkg/client/clientset/versioned/scheme"
	informers "github.com/kubeflow/mpi-operator/v2/pkg/client/informers/externalversions"
	"github.com/kubeflow/mpi-operator/v2/pkg/controller"
)

const (
	waitInterval = 100 * time.Millisecond
)

func TestMPIJobSuccess(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)
	s := newTestSetup(ctx, t)
	startController(ctx, s.kClient, s.mpiClient)

	mpiJob := &kubeflow.MPIJob{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "job",
			Namespace: s.namespace,
		},
		Spec: kubeflow.MPIJobSpec{
			SlotsPerWorker: newInt32(1),
			RunPolicy: common.RunPolicy{
				CleanPodPolicy: newCleanPodPolicy(common.CleanPodPolicyRunning),
			},
			MPIReplicaSpecs: map[kubeflow.MPIReplicaType]*common.ReplicaSpec{
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
					Replicas: newInt32(2),
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

	workerPods, launcherJob := validateMPIJobDependencies(ctx, t, s.kClient, mpiJob, 2)
	mpiJob = validateMPIJobStatus(ctx, t, s.mpiClient, mpiJob, map[common.ReplicaType]*common.ReplicaStatus{
		common.ReplicaType(kubeflow.MPIReplicaTypeLauncher): {},
		common.ReplicaType(kubeflow.MPIReplicaTypeWorker):   {},
	})
	if !mpiJobHasCondition(mpiJob, common.JobCreated) {
		t.Errorf("MPIJob missing Created condition")
	}
	s.events.verify(t)

	err = updatePodsToPhase(ctx, s.kClient, workerPods, corev1.PodRunning)
	if err != nil {
		t.Fatalf("Updating worker Pods to Running phase: %v", err)
	}
	validateMPIJobStatus(ctx, t, s.mpiClient, mpiJob, map[common.ReplicaType]*common.ReplicaStatus{
		common.ReplicaType(kubeflow.MPIReplicaTypeLauncher): {},
		common.ReplicaType(kubeflow.MPIReplicaTypeWorker): {
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
	validateMPIJobStatus(ctx, t, s.mpiClient, mpiJob, map[common.ReplicaType]*common.ReplicaStatus{
		common.ReplicaType(kubeflow.MPIReplicaTypeLauncher): {
			Active: 1,
		},
		common.ReplicaType(kubeflow.MPIReplicaTypeWorker): {
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
	validateMPIJobDependencies(ctx, t, s.kClient, mpiJob, 0)
	mpiJob = validateMPIJobStatus(ctx, t, s.mpiClient, mpiJob, map[common.ReplicaType]*common.ReplicaStatus{
		common.ReplicaType(kubeflow.MPIReplicaTypeLauncher): {
			Succeeded: 1,
		},
		common.ReplicaType(kubeflow.MPIReplicaTypeWorker): {},
	})
	s.events.verify(t)
	if !mpiJobHasCondition(mpiJob, common.JobSucceeded) {
		t.Errorf("MPIJob doesn't have Succeeded condition after launcher Job succeeded")
	}
}

func TestMPIJobFailure(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)
	s := newTestSetup(ctx, t)
	startController(ctx, s.kClient, s.mpiClient)

	mpiJob := &kubeflow.MPIJob{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "job",
			Namespace: s.namespace,
		},
		Spec: kubeflow.MPIJobSpec{
			SlotsPerWorker: newInt32(1),
			RunPolicy: common.RunPolicy{
				CleanPodPolicy: newCleanPodPolicy(common.CleanPodPolicyRunning),
			},
			MPIReplicaSpecs: map[kubeflow.MPIReplicaType]*common.ReplicaSpec{
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
					Replicas: newInt32(2),
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

	workerPods, launcherJob := validateMPIJobDependencies(ctx, t, s.kClient, mpiJob, 2)

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
	mpiJob = validateMPIJobStatus(ctx, t, s.mpiClient, mpiJob, map[common.ReplicaType]*common.ReplicaStatus{
		common.ReplicaType(kubeflow.MPIReplicaTypeLauncher): {
			Failed: 1,
		},
		common.ReplicaType(kubeflow.MPIReplicaTypeWorker): {
			Active: 2,
		},
	})
	if mpiJobHasCondition(mpiJob, common.JobFailed) {
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
	validateMPIJobDependencies(ctx, t, s.kClient, mpiJob, 0)
	mpiJob = validateMPIJobStatus(ctx, t, s.mpiClient, mpiJob, map[common.ReplicaType]*common.ReplicaStatus{
		common.ReplicaType(kubeflow.MPIReplicaTypeLauncher): {
			Failed: 2,
		},
		common.ReplicaType(kubeflow.MPIReplicaTypeWorker): {},
	})
	s.events.verify(t)
	if !mpiJobHasCondition(mpiJob, common.JobFailed) {
		t.Errorf("MPIJob doesn't have Failed condition after launcher Job fails")
	}
}

func startController(ctx context.Context, kClient kubernetes.Interface, mpiClient clientset.Interface) {
	kubeInformerFactory := kubeinformers.NewSharedInformerFactory(kClient, 0)
	mpiInformerFactory := informers.NewSharedInformerFactory(mpiClient, 0)
	ctrl := controller.NewMPIJobController(
		kClient,
		mpiClient,
		nil,
		kubeInformerFactory.Core().V1().ConfigMaps(),
		kubeInformerFactory.Core().V1().Secrets(),
		kubeInformerFactory.Core().V1().Services(),
		kubeInformerFactory.Batch().V1().Jobs(),
		kubeInformerFactory.Core().V1().Pods(),
		nil,
		mpiInformerFactory.Kubeflow().V2beta1().MPIJobs(),
		"")

	go kubeInformerFactory.Start(ctx.Done())
	go mpiInformerFactory.Start(ctx.Done())

	go ctrl.Run(1, ctx.Done())
}

func validateMPIJobDependencies(ctx context.Context, t *testing.T, kubeClient kubernetes.Interface, job *kubeflow.MPIJob, workers int) ([]corev1.Pod, *batchv1.Job) {
	t.Helper()
	var (
		svc         *corev1.Service
		cfgMap      *corev1.ConfigMap
		secret      *corev1.Secret
		workerPods  []corev1.Pod
		launcherJob *batchv1.Job
	)
	var problems []string
	if err := wait.Poll(waitInterval, wait.ForeverTestTimeout, func() (bool, error) {
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

func validateMPIJobStatus(ctx context.Context, t *testing.T, client clientset.Interface, job *kubeflow.MPIJob, want map[common.ReplicaType]*common.ReplicaStatus) *kubeflow.MPIJob {
	t.Helper()
	var (
		newJob *kubeflow.MPIJob
		err    error
		got    map[common.ReplicaType]*common.ReplicaStatus
	)
	if err := wait.Poll(waitInterval, wait.ForeverTestTimeout, func() (bool, error) {
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

func mpiJobHasCondition(job *kubeflow.MPIJob, cond common.JobConditionType) bool {
	for _, c := range job.Status.Conditions {
		if c.Type == cond {
			return c.Status == corev1.ConditionTrue
		}
	}
	return false
}

func newInt32(v int32) *int32 {
	return &v
}

func newCleanPodPolicy(policy common.CleanPodPolicy) *common.CleanPodPolicy {
	return &policy
}

func eventForJob(event corev1.Event, job *kubeflow.MPIJob) corev1.Event {
	event.Namespace = job.Namespace
	event.Source.Component = "mpi-job-controller"
	ref, err := reference.GetReference(scheme.Scheme, job)
	runtime.Must(err)
	event.InvolvedObject = *ref
	return event
}
