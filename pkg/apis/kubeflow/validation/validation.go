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
	"fmt"
	"strings"

	apivalidation "k8s.io/apimachinery/pkg/api/validation"
	"k8s.io/apimachinery/pkg/util/sets"
	apimachineryvalidation "k8s.io/apimachinery/pkg/util/validation"
	"k8s.io/apimachinery/pkg/util/validation/field"

	common "github.com/kubeflow/common/pkg/apis/common/v1"
	kubeflow "github.com/kubeflow/mpi-operator/v2/pkg/apis/kubeflow/v2beta1"
)

var (
	validCleanPolicies = sets.NewString(
		string(common.CleanPodPolicyNone),
		string(common.CleanPodPolicyRunning),
		string(common.CleanPodPolicyAll))

	validMPIImplementations = sets.NewString(
		string(kubeflow.MPIImplementationOpenMPI),
		string(kubeflow.MPIImplementationIntel))

	validRestartPolicies = sets.NewString(
		string(common.RestartPolicyNever),
		string(common.RestartPolicyOnFailure),
	)
)

func ValidateMPIJob(job *kubeflow.MPIJob) field.ErrorList {
	errs := validateMPIJobName(job)
	errs = append(errs, validateMPIJobSpec(&job.Spec, field.NewPath("spec"))...)
	return errs
}

func validateMPIJobName(job *kubeflow.MPIJob) field.ErrorList {
	var allErrs field.ErrorList
	var replicas int32 = 1
	if workerSpec := job.Spec.MPIReplicaSpecs[kubeflow.MPIReplicaTypeWorker]; workerSpec != nil {
		if workerSpec.Replicas != nil && *workerSpec.Replicas > 0 {
			replicas = *workerSpec.Replicas
		}
	}
	maximumPodHostname := fmt.Sprintf("%s-worker-%d", job.Name, replicas-1)
	if errs := apimachineryvalidation.IsDNS1123Label(maximumPodHostname); len(errs) > 0 {
		allErrs = append(allErrs, field.Invalid(field.NewPath("metadata").Child("name"), job.ObjectMeta.Name, fmt.Sprintf("will not able to create pod with invalid DNS label %q: %s", maximumPodHostname, strings.Join(errs, ", "))))
	}
	return allErrs
}

func validateMPIJobSpec(spec *kubeflow.MPIJobSpec, path *field.Path) field.ErrorList {
	errs := validateMPIReplicaSpecs(spec.MPIReplicaSpecs, path.Child("mpiReplicaSpecs"))
	if spec.SlotsPerWorker == nil {
		errs = append(errs, field.Required(path.Child("slotsPerWorker"), "must have number of slots per worker"))
	} else {
		errs = append(errs, apivalidation.ValidateNonnegativeField(int64(*spec.SlotsPerWorker), path.Child("slotsPerWorker"))...)
	}
	errs = append(errs, validateRunPolicy(&spec.RunPolicy, path.Child("runPolicy"))...)
	if spec.SSHAuthMountPath == "" {
		errs = append(errs, field.Required(path.Child("sshAuthMountPath"), "must have a mount path for SSH credentials"))
	}
	if !validMPIImplementations.Has(string(spec.MPIImplementation)) {
		errs = append(errs, field.NotSupported(path.Child("mpiImplementation"), spec.MPIImplementation, validMPIImplementations.List()))
	}
	return errs
}

func validateRunPolicy(policy *common.RunPolicy, path *field.Path) field.ErrorList {
	var errs field.ErrorList
	if policy.CleanPodPolicy == nil {
		errs = append(errs, field.Required(path.Child("cleanPodPolicy"), "must have clean Pod policy"))
	} else if !validCleanPolicies.Has(string(*policy.CleanPodPolicy)) {
		errs = append(errs, field.NotSupported(path.Child("cleanPodPolicy"), *policy.CleanPodPolicy, validCleanPolicies.List()))
	}
	// The remaining fields can be nil.
	if policy.TTLSecondsAfterFinished != nil {
		errs = append(errs, apivalidation.ValidateNonnegativeField(int64(*policy.TTLSecondsAfterFinished), path.Child("ttlSecondsAfterFinished"))...)
	}
	if policy.ActiveDeadlineSeconds != nil {
		errs = append(errs, apivalidation.ValidateNonnegativeField(*policy.ActiveDeadlineSeconds, path.Child("activeDeadlineSeconds"))...)
	}
	if policy.BackoffLimit != nil {
		errs = append(errs, apivalidation.ValidateNonnegativeField(int64(*policy.BackoffLimit), path.Child("backoffLimit"))...)
	}
	return errs
}

func validateMPIReplicaSpecs(replicaSpecs map[kubeflow.MPIReplicaType]*common.ReplicaSpec, path *field.Path) field.ErrorList {
	var errs field.ErrorList
	if replicaSpecs == nil {
		errs = append(errs, field.Required(path, "must have replica specs"))
		return errs
	}
	errs = append(errs, validateLauncherReplicaSpec(replicaSpecs[kubeflow.MPIReplicaTypeLauncher], path.Key(string(kubeflow.MPIReplicaTypeLauncher)))...)
	errs = append(errs, validateWorkerReplicaSpec(replicaSpecs[kubeflow.MPIReplicaTypeWorker], path.Key(string(kubeflow.MPIReplicaTypeWorker)))...)
	return errs
}

func validateLauncherReplicaSpec(spec *common.ReplicaSpec, path *field.Path) field.ErrorList {
	var errs field.ErrorList
	if spec == nil {
		errs = append(errs, field.Required(path, fmt.Sprintf("must have %s replica spec", kubeflow.MPIReplicaTypeLauncher)))
		return errs
	}
	errs = append(errs, validateReplicaSpec(spec, path)...)
	if spec.Replicas != nil && *spec.Replicas != 1 {
		errs = append(errs, field.Invalid(path.Child("replicas"), *spec.Replicas, "must be 1"))
	}
	return errs
}

func validateWorkerReplicaSpec(spec *common.ReplicaSpec, path *field.Path) field.ErrorList {
	var errs field.ErrorList
	if spec == nil {
		return errs
	}
	errs = append(errs, validateReplicaSpec(spec, path)...)
	if spec.Replicas != nil && *spec.Replicas <= 0 {
		errs = append(errs, field.Invalid(path.Child("replicas"), *spec.Replicas, "must be greater than or equal to 1"))
	}
	return errs
}

func validateReplicaSpec(spec *common.ReplicaSpec, path *field.Path) field.ErrorList {
	var errs field.ErrorList
	if spec.Replicas == nil {
		errs = append(errs, field.Required(path.Child("replicas"), "must define number of replicas"))
	}
	if !validRestartPolicies.Has(string(spec.RestartPolicy)) {
		errs = append(errs, field.NotSupported(path.Child("restartPolicy"), spec.RestartPolicy, validRestartPolicies.List()))
	}
	if len(spec.Template.Spec.Containers) == 0 {
		errs = append(errs, field.Required(path.Child("template", "spec", "containers"), "must define at least one container"))
	}
	return errs
}
