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

package v1alpha2

import (
	"k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	kubeflow "github.com/kubeflow/mpi-operator/pkg/apis/kubeflow/v1alpha2"
)

const (
	// mpiJobCreatedReason is added in a mpijob when it is created.
	mpiJobCreatedReason = "MPIJobCreated"
	// mpiJobSucceededReason is added in a mpijob when it is succeeded.
	mpiJobSucceededReason = "MPIJobSucceeded"
	// mpiJobRunningReason is added in a mpijob when it is running.
	mpiJobRunningReason = "MPIJobRunning"
	// mpiJobFailedReason is added in a mpijob when it is failed.
	mpiJobFailedReason = "MPIJobFailed"
	// mpiJobRestartingReason is added in a mpijob when it is restarting.
	mpiJobRestartingReason = "MPIJobRestarting"
)

// initializeMPIJobStatuses initializes the ReplicaStatuses for MPIJob.
func initializeMPIJobStatuses(mpiJob *kubeflow.MPIJob, mtype kubeflow.MPIReplicaType) {
	replicaType := kubeflow.ReplicaType(mtype)
	if mpiJob.Status.ReplicaStatuses == nil {
		mpiJob.Status.ReplicaStatuses = make(map[kubeflow.ReplicaType]*kubeflow.ReplicaStatus)
	}

	mpiJob.Status.ReplicaStatuses[replicaType] = &kubeflow.ReplicaStatus{}
}

// updateMPIJobConditions updates the conditions of the given mpiJob.
func updateMPIJobConditions(mpiJob *kubeflow.MPIJob, conditionType kubeflow.JobConditionType, reason, message string) error {
	condition := newCondition(conditionType, reason, message)
	setCondition(&mpiJob.Status, condition)
	return nil
}

// newCondition creates a new mpiJob condition.
func newCondition(conditionType kubeflow.JobConditionType, reason, message string) kubeflow.JobCondition {
	return kubeflow.JobCondition{
		Type:               conditionType,
		Status:             v1.ConditionTrue,
		LastUpdateTime:     metav1.Now(),
		LastTransitionTime: metav1.Now(),
		Reason:             reason,
		Message:            message,
	}
}

// getCondition returns the condition with the provided type.
func getCondition(status kubeflow.JobStatus, condType kubeflow.JobConditionType) *kubeflow.JobCondition {
	for _, condition := range status.Conditions {
		if condition.Type == condType {
			return &condition
		}
	}
	return nil
}

func hasCondition(status kubeflow.JobStatus, condType kubeflow.JobConditionType) bool {
	for _, condition := range status.Conditions {
		if condition.Type == condType && condition.Status == v1.ConditionTrue {
			return true
		}
	}
	return false
}

func isSucceeded(status kubeflow.JobStatus) bool {
	return hasCondition(status, kubeflow.JobSucceeded)
}

func isFailed(status kubeflow.JobStatus) bool {
	return hasCondition(status, kubeflow.JobFailed)
}

// setCondition updates the mpiJob to include the provided condition.
// If the condition that we are about to add already exists
// and has the same status and reason then we are not going to update.
func setCondition(status *kubeflow.JobStatus, condition kubeflow.JobCondition) {
	// Do nothing if MPIJobStatus have failed condition
	if isFailed(*status) {
		return
	}

	currentCond := getCondition(*status, condition.Type)

	// Do nothing if condition doesn't change
	if currentCond != nil && currentCond.Status == condition.Status && currentCond.Reason == condition.Reason {
		return
	}

	// Do not update lastTransitionTime if the status of the condition doesn't change.
	if currentCond != nil && currentCond.Status == condition.Status {
		condition.LastTransitionTime = currentCond.LastTransitionTime
	}

	// Append the updated condition
	newConditions := filterOutCondition(status.Conditions, condition.Type)
	status.Conditions = append(newConditions, condition)
}

// filterOutCondition returns a new slice of mpiJob conditions without conditions with the provided type.
func filterOutCondition(conditions []kubeflow.JobCondition, condType kubeflow.JobConditionType) []kubeflow.JobCondition {
	var newConditions []kubeflow.JobCondition
	for _, c := range conditions {
		if condType == kubeflow.JobRestarting && c.Type == kubeflow.JobRunning {
			continue
		}
		if condType == kubeflow.JobRunning && c.Type == kubeflow.JobRestarting {
			continue
		}

		if c.Type == condType {
			continue
		}

		// Set the running condition status to be false when current condition failed or succeeded
		if (condType == kubeflow.JobFailed || condType == kubeflow.JobSucceeded) && c.Type == kubeflow.JobRunning {
			c.Status = v1.ConditionFalse
		}

		newConditions = append(newConditions, c)
	}
	return newConditions
}
