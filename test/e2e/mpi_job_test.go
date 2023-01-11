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

// +build e2e

package e2e

import (
	"context"
	"fmt"
	"io"

	common "github.com/kubeflow/common/pkg/apis/common/v1"
	kubeflow "github.com/kubeflow/mpi-operator/v2/pkg/apis/kubeflow/v2beta1"
	"github.com/onsi/ginkgo"
	"github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
	"k8s.io/apimachinery/pkg/util/wait"
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
					kubeflow.MPIReplicaTypeLauncher: {
						RestartPolicy: common.RestartPolicyOnFailure,
					},
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

		ginkgo.When("has malformed command", func() {
			ginkgo.BeforeEach(func() {
				mpiJob.Spec.RunPolicy.BackoffLimit = newInt32(1)
			})
			ginkgo.It("should fail", func() {
				mpiJob := createJobAndWaitForCompletion(mpiJob)
				expectConditionToBeTrue(mpiJob, common.JobFailed)
			})
		})

		ginkgo.When("running as root", func() {
			ginkgo.BeforeEach(func() {
				launcherContainer := &mpiJob.Spec.MPIReplicaSpecs[kubeflow.MPIReplicaTypeLauncher].Template.Spec.Containers[0]
				launcherContainer.Command = append(launcherContainer.Command, "--allow-run-as-root")
			})

			ginkgo.It("should succeed", func() {
				mpiJob := createJobAndWaitForCompletion(mpiJob)
				expectConditionToBeTrue(mpiJob, common.JobSucceeded)
			})

			ginkgo.When("running with host network", func() {
				ginkgo.BeforeEach(func() {
					mpiJob.Spec.MPIReplicaSpecs[kubeflow.MPIReplicaTypeLauncher].Template.Spec.HostNetwork = true
					mpiJob.Spec.MPIReplicaSpecs[kubeflow.MPIReplicaTypeWorker].Template.Spec.HostNetwork = true
					// The test cluster has only one node.
					// More than one pod cannot use the same host port for sshd.
					mpiJob.Spec.MPIReplicaSpecs[kubeflow.MPIReplicaTypeLauncher].Template.Spec.Containers[0].Args = []string{"/home/mpiuser/pi"}
					mpiJob.Spec.MPIReplicaSpecs[kubeflow.MPIReplicaTypeWorker].Replicas = newInt32(1)
				})

				ginkgo.It("should succeed", func() {
					mpiJob := createJobAndWaitForCompletion(mpiJob)
					expectConditionToBeTrue(mpiJob, common.JobSucceeded)
				})
			})
		})

		ginkgo.When("running as non-root", func() {
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
				mpiJob := createJobAndWaitForCompletion(mpiJob)
				expectConditionToBeTrue(mpiJob, common.JobSucceeded)
			})
		})

	})

	ginkgo.Context("with Intel Implementation", func() {
		ginkgo.When("running as root", func() {
			ginkgo.BeforeEach(func() {
				mpiJob.Spec.MPIImplementation = kubeflow.MPIImplementationIntel
				mpiJob.Spec.MPIReplicaSpecs[kubeflow.MPIReplicaTypeLauncher].Template.Spec.Containers = []corev1.Container{
					{
						Name:    "launcher",
						Image:   intelMPIImage,
						Command: []string{}, // uses entrypoint.
						Args: []string{
							"mpirun",
							"-n",
							"2",
							"/home/mpiuser/pi",
						},
					},
				}
				mpiJob.Spec.MPIReplicaSpecs[kubeflow.MPIReplicaTypeWorker].Template.Spec.Containers = []corev1.Container{
					{
						Name:    "worker",
						Image:   intelMPIImage,
						Command: []string{}, // uses entrypoint.
						Args: []string{
							"/usr/sbin/sshd",
							"-De",
						},
						ReadinessProbe: &corev1.Probe{
							Handler: corev1.Handler{
								TCPSocket: &corev1.TCPSocketAction{
									Port: intstr.FromInt(2222),
								},
							},
							InitialDelaySeconds: 3,
						},
					},
				}
			})

			ginkgo.It("should succeed", func() {
				mpiJob := createJobAndWaitForCompletion(mpiJob)
				expectConditionToBeTrue(mpiJob, common.JobSucceeded)
			})
		})

	})
})

func createJobAndWaitForCompletion(mpiJob *kubeflow.MPIJob) *kubeflow.MPIJob {
	ctx := context.Background()
	var err error
	ginkgo.By("Creating MPIJob")
	mpiJob, err = mpiClient.KubeflowV2beta1().MPIJobs(mpiJob.Namespace).Create(ctx, mpiJob, metav1.CreateOptions{})
	gomega.Expect(err).ToNot(gomega.HaveOccurred())

	ginkgo.By("Waiting for MPIJob to finish")
	err = wait.Poll(waitInterval, foreverTimeout, func() (bool, error) {
		updatedJob, err := mpiClient.KubeflowV2beta1().MPIJobs(mpiJob.Namespace).Get(ctx, mpiJob.Name, metav1.GetOptions{})
		if err != nil {
			return false, err
		}
		mpiJob = updatedJob
		return mpiJob.Status.CompletionTime != nil, nil
	})
	if err != nil {
		err := debugJob(ctx, mpiJob)
		if err != nil {
			fmt.Fprintf(ginkgo.GinkgoWriter, "Failed to debug job: %v\n", err)
		}
	}
	gomega.Expect(err).ToNot(gomega.HaveOccurred())
	return mpiJob
}

func debugJob(ctx context.Context, mpiJob *kubeflow.MPIJob) error {
	selector := metav1.LabelSelector{
		MatchLabels: map[string]string{
			common.OperatorNameLabel: kubeflow.OperatorName,
			common.JobNameLabel:      mpiJob.Name,
			common.JobRoleLabel:      "launcher",
		},
	}
	launcherPods, err := k8sClient.CoreV1().Pods(mpiJob.Namespace).List(ctx, metav1.ListOptions{
		LabelSelector: metav1.FormatLabelSelector(&selector),
	})
	if err != nil {
		return fmt.Errorf("getting launcher Pods: %w", err)
	}
	if len(launcherPods.Items) == 0 {
		return fmt.Errorf("no launcher Pods found")
	}
	lastPod := launcherPods.Items[0]
	for _, p := range launcherPods.Items[1:] {
		if p.CreationTimestamp.After(p.CreationTimestamp.Time) {
			lastPod = p
		}
	}
	err = podLogs(ctx, &lastPod)
	if err != nil {
		return fmt.Errorf("obtaining launcher logs: %w", err)
	}

	selector.MatchLabels[common.JobRoleLabel] = "worker"
	workerPods, err := k8sClient.CoreV1().Pods(mpiJob.Namespace).List(ctx, metav1.ListOptions{
		LabelSelector: metav1.FormatLabelSelector(&selector),
	})
	if err != nil {
		return fmt.Errorf("getting worker Pods: %w", err)
	}
	for _, p := range workerPods.Items {
		err = podLogs(ctx, &p)
		if err != nil {
			return fmt.Errorf("obtaining worker logs: %w", err)
		}
	}
	return nil
}

func podLogs(ctx context.Context, p *corev1.Pod) error {
	req := k8sClient.CoreV1().Pods(p.Namespace).GetLogs(p.Name, &corev1.PodLogOptions{})
	stream, err := req.Stream(ctx)
	if err != nil {
		return fmt.Errorf("reading logs: %v", err)
	}
	defer stream.Close()
	fmt.Fprintf(ginkgo.GinkgoWriter, "== BEGIN %s pod logs ==\n", p.Name)
	_, err = io.Copy(ginkgo.GinkgoWriter, stream)
	if err != nil {
		return fmt.Errorf("writing logs: %v", err)
	}
	fmt.Fprintf(ginkgo.GinkgoWriter, "\n== END %s pod logs ==\n", p.Name)
	return nil
}

func expectConditionToBeTrue(mpiJob *kubeflow.MPIJob, condType common.JobConditionType) {
	var condition *common.JobCondition
	for _, cond := range mpiJob.Status.Conditions {
		if cond.Type == condType {
			condition = &cond
			break
		}
	}
	gomega.Expect(condition).ToNot(gomega.BeNil())
	gomega.Expect(condition.Status).To(gomega.Equal(corev1.ConditionTrue))
}

func newInt32(v int32) *int32 {
	return &v
}

func newInt64(v int64) *int64 {
	return &v
}
