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

package e2e

import (
	"context"

	common "github.com/kubeflow/common/pkg/apis/common/v1"
	"github.com/onsi/ginkgo"
	"github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/wait"

	kubeflow "github.com/kubeflow/mpi-operator/v2/pkg/apis/kubeflow/v2beta1"
)

var _ = ginkgo.Describe("MPIJob", func() {
	var (
		namespace string
		mpiJob    *kubeflow.MPIJob
	)

	ginkgo.BeforeEach(func() {
		ns := &corev1.Namespace{
			ObjectMeta: metav1.ObjectMeta{
				GenerateName: "e2e-",
			},
		}
		var err error
		ns, err = k8sClient.CoreV1().Namespaces().Create(context.Background(), ns, metav1.CreateOptions{})
		gomega.Expect(err).ToNot(gomega.HaveOccurred())
		namespace = ns.Name
	})

	ginkgo.AfterEach(func() {
		if namespace != "" {
			err := k8sClient.CoreV1().Namespaces().Delete(context.Background(), namespace, metav1.DeleteOptions{})
			if !errors.IsNotFound(err) {
				gomega.Expect(err).ToNot(gomega.HaveOccurred())
			}
		}
	})

	ginkgo.BeforeEach(func() {
		mpiJob = &kubeflow.MPIJob{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "pi",
				Namespace: namespace,
			},
			Spec: kubeflow.MPIJobSpec{
				MPIReplicaSpecs: map[kubeflow.MPIReplicaType]*common.ReplicaSpec{
					kubeflow.MPIReplicaTypeLauncher: {},
					kubeflow.MPIReplicaTypeWorker: {
						Replicas: newInt32(2),
					},
				},
			},
		}
	})

	ginkgo.Context("with OpenMPI implementation", func() {

		ginkgo.BeforeEach(func() {
			mpiJob.Spec.MPIReplicaSpecs[kubeflow.MPIReplicaTypeLauncher].Template.Spec.Containers = []corev1.Container{
				{
					Name:    "launcher",
					Image:   openMPIImage,
					Command: []string{"mpirun"},
					Args: []string{
						"-n",
						"2",
						"/home/mpiuser/pi",
					},
				},
			}
			mpiJob.Spec.MPIReplicaSpecs[kubeflow.MPIReplicaTypeWorker].Template.Spec.Containers = []corev1.Container{
				{
					Name:  "worker",
					Image: openMPIImage,
				},
			}
		})

		ginkgo.Context("when running as non-root user", func() {

			ginkgo.BeforeEach(func() {
				mpiJob.Spec.SSHAuthMountPath = "/home/mpiuser/.ssh"
				mpiJob.Spec.MPIReplicaSpecs[kubeflow.MPIReplicaTypeLauncher].Template.Spec.Containers[0].SecurityContext = &corev1.SecurityContext{
					RunAsUser: newInt64(1000),
				}
				workerContainer := &mpiJob.Spec.MPIReplicaSpecs[kubeflow.MPIReplicaTypeWorker].Template.Spec.Containers[0]
				workerContainer.SecurityContext = &corev1.SecurityContext{
					RunAsUser: newInt64(1000),
					Capabilities: &corev1.Capabilities{
						Add: []corev1.Capability{"NET_BIND_SERVICE"},
					},
				}
				workerContainer.Command = []string{"/usr/sbin/sshd"}
				workerContainer.Args = []string{"-De", "-f", "/home/mpiuser/.sshd_config"}
			})

			ginkgo.It("should succeed", func() {
				ctx := context.Background()
				var err error
				ginkgo.By("Creating MPIJob")
				mpiJob, err = mpiClient.KubeflowV2beta1().MPIJobs(namespace).Create(ctx, mpiJob, metav1.CreateOptions{})
				gomega.Expect(err).ToNot(gomega.HaveOccurred())

				ginkgo.By("Waiting for MPIJob to finish")
				err = wait.Poll(waitInterval, foreverTimeout, func() (bool, error) {
					updatedJob, err := mpiClient.KubeflowV2beta1().MPIJobs(namespace).Get(ctx, mpiJob.Name, metav1.GetOptions{})
					if err != nil {
						return false, err
					}
					mpiJob = updatedJob
					return mpiJob.Status.CompletionTime != nil, nil
				})
				gomega.Expect(err).ToNot(gomega.HaveOccurred())

				var succeedCond *common.JobCondition
				for _, cond := range mpiJob.Status.Conditions {
					if cond.Type == common.JobSucceeded {
						succeedCond = &cond
						break
					}
				}
				gomega.Expect(succeedCond).ToNot(gomega.BeNil())
				gomega.Expect(succeedCond.Status).To(gomega.Equal(corev1.ConditionTrue))
			})
		})
	})
})

func newInt32(v int32) *int32 {
	return &v
}

func newInt64(v int64) *int64 {
	return &v
}
