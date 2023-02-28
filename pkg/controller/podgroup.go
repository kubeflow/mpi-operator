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
	common "github.com/kubeflow/common/pkg/apis/common/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	volcanov1beta1 "volcano.sh/apis/pkg/apis/scheduling/v1beta1"

	kubeflow "github.com/kubeflow/mpi-operator/pkg/apis/kubeflow/v2beta1"
)

// newPodGroup will create a new PodGroup for an MPIJob
// resource. If the parameters set in the schedulingPolicy aren't empty, it will pass them to a new PodGroup;
// if they are empty, it will set the default values in the following:
//
//	minMember: NUM(workers) + 1
//	queue: A "scheduling.volcano.sh/queue-name" annotation value.
//	priorityClass: A value returned from the calcPriorityClassName function.
//	minResources: nil
//
// It also sets the appropriate OwnerReferences on the resource so
// handleObject can discover the MPIJob resource that 'owns' it.
func newPodGroup(mpiJob *kubeflow.MPIJob) *volcanov1beta1.PodGroup {
	minMember := workerReplicas(mpiJob) + 1
	queueName := mpiJob.Annotations[volcanov1beta1.QueueNameAnnotationKey]
	var minResources *corev1.ResourceList
	if schedulingPolicy := mpiJob.Spec.RunPolicy.SchedulingPolicy; schedulingPolicy != nil {
		if schedulingPolicy.MinAvailable != nil {
			minMember = *schedulingPolicy.MinAvailable
		}
		if len(schedulingPolicy.Queue) != 0 {
			queueName = schedulingPolicy.Queue
		}
		if schedulingPolicy.MinResources != nil {
			minResources = schedulingPolicy.MinResources
		}
	}
	return &volcanov1beta1.PodGroup{
		ObjectMeta: metav1.ObjectMeta{
			Name:      mpiJob.Name,
			Namespace: mpiJob.Namespace,
			OwnerReferences: []metav1.OwnerReference{
				*metav1.NewControllerRef(mpiJob, kubeflow.SchemeGroupVersionKind),
			},
		},
		Spec: volcanov1beta1.PodGroupSpec{
			MinMember:         minMember,
			Queue:             queueName,
			PriorityClassName: calcPriorityClassName(mpiJob.Spec.MPIReplicaSpecs, mpiJob.Spec.RunPolicy.SchedulingPolicy),
			MinResources:      minResources,
		},
	}
}

// calcPriorityClassName calculates the priorityClass name needed for podGroup according to the following priorities:
//  1. .spec.runPolicy.schedulingPolicy.priorityClass
//  2. .spec.mpiReplicaSecs[Launcher].template.spec.priorityClassName
//  3. .spec.mpiReplicaSecs[Worker].template.spec.priorityClassName
func calcPriorityClassName(
	replicas map[kubeflow.MPIReplicaType]*common.ReplicaSpec,
	schedulingPolicy *kubeflow.SchedulingPolicy,
) string {
	if schedulingPolicy != nil && len(schedulingPolicy.PriorityClass) != 0 {
		return schedulingPolicy.PriorityClass
	} else if l := replicas[kubeflow.MPIReplicaTypeLauncher]; l != nil && len(l.Template.Spec.PriorityClassName) != 0 {
		return l.Template.Spec.PriorityClassName
	} else if w := replicas[kubeflow.MPIReplicaTypeWorker]; w != nil && len(w.Template.Spec.PriorityClassName) != 0 {
		return w.Template.Spec.PriorityClassName
	} else {
		return ""
	}
}
