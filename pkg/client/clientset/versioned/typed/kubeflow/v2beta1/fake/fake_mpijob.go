// Copyright 2025 The Kubeflow Authors.
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

// Code generated by client-gen. DO NOT EDIT.

package fake

import (
	v2beta1 "github.com/kubeflow/mpi-operator/pkg/apis/kubeflow/v2beta1"
	kubeflowv2beta1 "github.com/kubeflow/mpi-operator/pkg/client/applyconfiguration/kubeflow/v2beta1"
	typedkubeflowv2beta1 "github.com/kubeflow/mpi-operator/pkg/client/clientset/versioned/typed/kubeflow/v2beta1"
	gentype "k8s.io/client-go/gentype"
)

// fakeMPIJobs implements MPIJobInterface
type fakeMPIJobs struct {
	*gentype.FakeClientWithListAndApply[*v2beta1.MPIJob, *v2beta1.MPIJobList, *kubeflowv2beta1.MPIJobApplyConfiguration]
	Fake *FakeKubeflowV2beta1
}

func newFakeMPIJobs(fake *FakeKubeflowV2beta1, namespace string) typedkubeflowv2beta1.MPIJobInterface {
	return &fakeMPIJobs{
		gentype.NewFakeClientWithListAndApply[*v2beta1.MPIJob, *v2beta1.MPIJobList, *kubeflowv2beta1.MPIJobApplyConfiguration](
			fake.Fake,
			namespace,
			v2beta1.SchemeGroupVersion.WithResource("mpijobs"),
			v2beta1.SchemeGroupVersion.WithKind("MPIJob"),
			func() *v2beta1.MPIJob { return &v2beta1.MPIJob{} },
			func() *v2beta1.MPIJobList { return &v2beta1.MPIJobList{} },
			func(dst, src *v2beta1.MPIJobList) { dst.ListMeta = src.ListMeta },
			func(list *v2beta1.MPIJobList) []*v2beta1.MPIJob { return gentype.ToPointerSlice(list.Items) },
			func(list *v2beta1.MPIJobList, items []*v2beta1.MPIJob) { list.Items = gentype.FromPointerSlice(items) },
		),
		fake,
	}
}
