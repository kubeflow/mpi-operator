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

package controller

import (
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	common "github.com/kubeflow/common/pkg/apis/common/v1"
	kubeflow "github.com/kubeflow/mpi-operator/v2/pkg/apis/kubeflow/v2beta1"
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
	// mpiJobEvict
	mpiJobEvict = "MPIJobEvicted"
)

// initializeMPIJobStatuses initializes the ReplicaStatuses for MPIJob.
func initializeMPIJobStatuses(mpiJob *kubeflow.MPIJob, mtype kubeflow.MPIReplicaType) {
	replicaType := common.ReplicaType(mtype)
	if mpiJob.Status.ReplicaStatuses == nil {
		mpiJob.Status.ReplicaStatuses = make(map[common.ReplicaType]*common.ReplicaStatus)
	}

	mpiJob.Status.ReplicaStatuses[replicaType] = &common.ReplicaStatus{}
}

// updateMPIJobConditions updates the conditions of the given mpiJob.
func updateMPIJobConditions(mpiJob *kubeflow.MPIJob, conditionType common.JobConditionType, reason, message string) {
	condition := newCondition(conditionType, reason, message)
	setCondition(&mpiJob.Status, condition)
}

// newCondition creates a new mpiJob condition.
func newCondition(conditionType common.JobConditionType, reason, message string) common.JobCondition {
	return common.JobCondition{
		Type:               conditionType,
		Status:             v1.ConditionTrue,
		LastUpdateTime:     metav1.Now(),
		LastTransitionTime: metav1.Now(),
		Reason:             reason,
		Message:            message,
	}
}

// getCondition returns the condition with the provided type.
func getCondition(status common.JobStatus, condType common.JobConditionType) *common.JobCondition {
	for _, condition := range status.Conditions {
		if condition.Type == condType {
			return &condition
		}
	}
	return nil
}

func hasCondition(status common.JobStatus, condType common.JobConditionType) bool {
	for _, condition := range status.Conditions {
		if condition.Type == condType && condition.Status == v1.ConditionTrue {
			return true
		}
	}
	return false
}

func isFinished(status common.JobStatus) bool {
	return isSucceeded(status) || isFailed(status)
}

func isSucceeded(status common.JobStatus) bool {
	return hasCondition(status, common.JobSucceeded)
}

func isFailed(status common.JobStatus) bool {
	return hasCondition(status, common.JobFailed)
}

// setCondition updates the mpiJob to include the provided condition.
// If the condition that we are about to add already exists
// and has the same status and reason then we are not going to update.
func setCondition(status *common.JobStatus, condition common.JobCondition) {

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
func filterOutCondition(conditions []common.JobCondition, condType common.JobConditionType) []common.JobCondition {
	var newConditions []common.JobCondition
	for _, c := range conditions {
		if condType == common.JobRestarting && c.Type == common.JobRunning {
			continue
		}
		if condType == common.JobRunning && c.Type == common.JobRestarting {
			continue
		}

		if c.Type == condType {
			continue
		}

		// Set the running condition status to be false when current condition failed or succeeded
		if (condType == common.JobFailed || condType == common.JobSucceeded) && (c.Type == common.JobRunning || c.Type == common.JobFailed) {
			c.Status = v1.ConditionFalse
		}

		newConditions = append(newConditions, c)
	}
	return newConditions
}
