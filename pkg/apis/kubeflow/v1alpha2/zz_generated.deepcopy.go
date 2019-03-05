// +build !ignore_autogenerated

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

// Code generated by deepcopy-gen. DO NOT EDIT.

package v1alpha2

import (
	runtime "k8s.io/apimachinery/pkg/runtime"
)

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *MPIJob) DeepCopyInto(out *MPIJob) {
	*out = *in
	out.TypeMeta = in.TypeMeta
	in.ObjectMeta.DeepCopyInto(&out.ObjectMeta)
	in.Spec.DeepCopyInto(&out.Spec)
	in.Status.DeepCopyInto(&out.Status)
	return
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new MPIJob.
func (in *MPIJob) DeepCopy() *MPIJob {
	if in == nil {
		return nil
	}
	out := new(MPIJob)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyObject is an autogenerated deepcopy function, copying the receiver, creating a new runtime.Object.
func (in *MPIJob) DeepCopyObject() runtime.Object {
	if c := in.DeepCopy(); c != nil {
		return c
	}
	return nil
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *MPIJobList) DeepCopyInto(out *MPIJobList) {
	*out = *in
	out.TypeMeta = in.TypeMeta
	out.ListMeta = in.ListMeta
	if in.Items != nil {
		in, out := &in.Items, &out.Items
		*out = make([]MPIJob, len(*in))
		for i := range *in {
			(*in)[i].DeepCopyInto(&(*out)[i])
		}
	}
	return
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new MPIJobList.
func (in *MPIJobList) DeepCopy() *MPIJobList {
	if in == nil {
		return nil
	}
	out := new(MPIJobList)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyObject is an autogenerated deepcopy function, copying the receiver, creating a new runtime.Object.
func (in *MPIJobList) DeepCopyObject() runtime.Object {
	if c := in.DeepCopy(); c != nil {
		return c
	}
	return nil
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *MPIJobSpec) DeepCopyInto(out *MPIJobSpec) {
	*out = *in
	if in.GPUs != nil {
		in, out := &in.GPUs, &out.GPUs
		*out = new(int32)
		**out = **in
	}
	if in.GPUsPerNode != nil {
		in, out := &in.GPUsPerNode, &out.GPUsPerNode
		*out = new(int32)
		**out = **in
	}
	if in.ProcessingUnits != nil {
		in, out := &in.ProcessingUnits, &out.ProcessingUnits
		*out = new(int32)
		**out = **in
	}
	if in.ProcessingUnitsPerNode != nil {
		in, out := &in.ProcessingUnitsPerNode, &out.ProcessingUnitsPerNode
		*out = new(int32)
		**out = **in
	}
	if in.SlotsPerWorker != nil {
		in, out := &in.SlotsPerWorker, &out.SlotsPerWorker
		*out = new(int32)
		**out = **in
	}
	if in.BackoffLimit != nil {
		in, out := &in.BackoffLimit, &out.BackoffLimit
		*out = new(int32)
		**out = **in
	}
	if in.ActiveDeadlineSeconds != nil {
		in, out := &in.ActiveDeadlineSeconds, &out.ActiveDeadlineSeconds
		*out = new(int64)
		**out = **in
	}
	if in.Replicas != nil {
		in, out := &in.Replicas, &out.Replicas
		*out = new(int32)
		**out = **in
	}
	in.Template.DeepCopyInto(&out.Template)
	return
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new MPIJobSpec.
func (in *MPIJobSpec) DeepCopy() *MPIJobSpec {
	if in == nil {
		return nil
	}
	out := new(MPIJobSpec)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *MPIJobStatus) DeepCopyInto(out *MPIJobStatus) {
	*out = *in
	if in.StartTime != nil {
		in, out := &in.StartTime, &out.StartTime
		*out = (*in).DeepCopy()
	}
	if in.CompletionTime != nil {
		in, out := &in.CompletionTime, &out.CompletionTime
		*out = (*in).DeepCopy()
	}
	return
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new MPIJobStatus.
func (in *MPIJobStatus) DeepCopy() *MPIJobStatus {
	if in == nil {
		return nil
	}
	out := new(MPIJobStatus)
	in.DeepCopyInto(out)
	return out
}
