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
	"fmt"
	"io"
	"time"

	"github.com/google/go-cmp/cmp"
	"github.com/onsi/ginkgo"
	"github.com/onsi/gomega"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/util/intstr"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/utils/ptr"
	schedv1alpha1 "sigs.k8s.io/scheduler-plugins/apis/scheduling/v1alpha1"

	kubeflow "github.com/kubeflow/mpi-operator/pkg/apis/kubeflow/v2beta1"
	"github.com/kubeflow/mpi-operator/test/util"
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
				MPIReplicaSpecs: map[kubeflow.MPIReplicaType]*kubeflow.ReplicaSpec{
					kubeflow.MPIReplicaTypeLauncher: {
						RestartPolicy: kubeflow.RestartPolicyOnFailure,
					},
					kubeflow.MPIReplicaTypeWorker: {
						Replicas: ptr.To[int32](2),
					},
				},
			},
		}
	})

	ginkgo.Context("with OpenMPI implementation", func() {
		ginkgo.BeforeEach(func() {
			createMPIJobWithOpenMPI(mpiJob)
		})

		ginkgo.When("has malformed command", func() {
			ginkgo.BeforeEach(func() {
				mpiJob.Spec.RunPolicy.BackoffLimit = ptr.To[int32](1)
			})
			ginkgo.It("should fail", func() {
				mpiJob := createJobAndWaitForCompletion(mpiJob)
				expectConditionToBeTrue(mpiJob, kubeflow.JobFailed)
			})
		})

		ginkgo.When("running as root", func() {
			ginkgo.BeforeEach(func() {
				launcherContainer := &mpiJob.Spec.MPIReplicaSpecs[kubeflow.MPIReplicaTypeLauncher].Template.Spec.Containers[0]
				launcherContainer.Command = append(launcherContainer.Command, "--allow-run-as-root")
			})

			ginkgo.It("should succeed", func() {
				mpiJob := createJobAndWaitForCompletion(mpiJob)
				expectConditionToBeTrue(mpiJob, kubeflow.JobSucceeded)
			})

			ginkgo.When("suspended on creation", func() {
				ginkgo.BeforeEach(func() {
					mpiJob.Spec.RunPolicy.Suspend = ptr.To(true)
				})
				ginkgo.It("should not create pods when suspended and succeed when resumed", func() {
					ctx := context.Background()
					mpiJob := createJob(ctx, mpiJob)

					ginkgo.By("verifying there are no pods (neither launcher nor pods) running for the suspended MPIJob")
					pods, err := k8sClient.CoreV1().Pods(mpiJob.Namespace).List(ctx, metav1.ListOptions{})
					gomega.Expect(err).ToNot(gomega.HaveOccurred())
					gomega.Expect(pods.Items).To(gomega.HaveLen(0))

					mpiJob = resumeJob(ctx, mpiJob)
					mpiJob = waitForCompletion(ctx, mpiJob)
					expectConditionToBeTrue(mpiJob, kubeflow.JobSucceeded)
				})
			})

			ginkgo.When("running with host network", func() {
				ginkgo.BeforeEach(func() {
					mpiJob.Spec.MPIReplicaSpecs[kubeflow.MPIReplicaTypeLauncher].Template.Spec.HostNetwork = true
					mpiJob.Spec.MPIReplicaSpecs[kubeflow.MPIReplicaTypeWorker].Template.Spec.HostNetwork = true
					// The test cluster has only one node.
					// More than one pod cannot use the same host port for sshd.
					mpiJob.Spec.MPIReplicaSpecs[kubeflow.MPIReplicaTypeLauncher].Template.Spec.Containers[0].Args = []string{"/home/mpiuser/pi"}
					mpiJob.Spec.MPIReplicaSpecs[kubeflow.MPIReplicaTypeWorker].Replicas = ptr.To[int32](1)
				})

				ginkgo.It("should succeed", func() {
					mpiJob := createJobAndWaitForCompletion(mpiJob)
					expectConditionToBeTrue(mpiJob, kubeflow.JobSucceeded)
				})
			})
		})

		ginkgo.When("running as non-root", func() {
			ginkgo.BeforeEach(func() {
				mpiJob.Spec.SSHAuthMountPath = "/home/mpiuser/.ssh"
				mpiJob.Spec.MPIReplicaSpecs[kubeflow.MPIReplicaTypeLauncher].Template.Spec.Containers[0].SecurityContext = &corev1.SecurityContext{
					RunAsUser: ptr.To[int64](1000),
				}
				workerContainer := &mpiJob.Spec.MPIReplicaSpecs[kubeflow.MPIReplicaTypeWorker].Template.Spec.Containers[0]
				workerContainer.SecurityContext = &corev1.SecurityContext{
					RunAsUser: ptr.To[int64](1000),
					Capabilities: &corev1.Capabilities{
						Add: []corev1.Capability{"NET_BIND_SERVICE"},
					},
				}
				workerContainer.Command = []string{"/usr/sbin/sshd"}
				workerContainer.Args = []string{"-De", "-f", "/home/mpiuser/.sshd_config"}
			})

			ginkgo.It("should succeed", func() {
				mpiJob := createJobAndWaitForCompletion(mpiJob)
				expectConditionToBeTrue(mpiJob, kubeflow.JobSucceeded)
			})

			ginkgo.It("should not be updated when managed externaly, only created", func() {
				mpiJob.Spec.RunPolicy.ManagedBy = ptr.To(kubeflow.MultiKueueController)
				ctx := context.Background()
				mpiJob = createJob(ctx, mpiJob)

				time.Sleep(util.SleepDurationControllerSyncDelay)
				mpiJob, err := mpiClient.KubeflowV2beta1().MPIJobs(mpiJob.Namespace).Get(ctx, mpiJob.Name, metav1.GetOptions{})
				gomega.Expect(err).To(gomega.BeNil())

				// job should be created, but status should not be updated neither for create nor for any other status
				condition := getJobCondition(mpiJob, kubeflow.JobCreated)
				gomega.Expect(condition).To(gomega.BeNil())
				condition = getJobCondition(mpiJob, kubeflow.JobSucceeded)
				gomega.Expect(condition).To(gomega.BeNil())
				launcherJob, err := getLauncherJob(ctx, mpiJob)
				gomega.Expect(err).To(gomega.BeNil())
				gomega.Expect(launcherJob).To(gomega.BeNil())
				launcherPods, err := getLauncherPods(ctx, mpiJob)
				gomega.Expect(err).To(gomega.BeNil())
				gomega.Expect(len(launcherPods.Items)).To(gomega.Equal(0))
				workerPods, err := getWorkerPods(ctx, mpiJob)
				gomega.Expect(err).To(gomega.BeNil())
				gomega.Expect(len(workerPods.Items)).To(gomega.Equal(0))
				secret, err := getSecretsForJob(ctx, mpiJob)
				gomega.Expect(err).To(gomega.BeNil())
				gomega.Expect(secret).To(gomega.BeNil())
			})

			ginkgo.It("should succeed when explicitly managed by mpi-operator", func() {
				mpiJob.Spec.RunPolicy.ManagedBy = ptr.To(kubeflow.KubeflowJobController)
				mpiJob := createJobAndWaitForCompletion(mpiJob)
				expectConditionToBeTrue(mpiJob, kubeflow.JobSucceeded)
			})
		})
	})

	ginkgo.Context("with Intel Implementation", func() {
		ginkgo.BeforeEach(func() {
			mpiJob.Spec.MPIImplementation = kubeflow.MPIImplementationIntel
			mpiJob.Spec.MPIReplicaSpecs[kubeflow.MPIReplicaTypeLauncher].Template.Spec.Containers = []corev1.Container{
				{
					Name:            "launcher",
					Image:           intelMPIImage,
					ImagePullPolicy: corev1.PullIfNotPresent, // use locally built image.
					Command:         []string{},              // uses entrypoint.
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
					Name:            "worker",
					Image:           intelMPIImage,
					ImagePullPolicy: corev1.PullIfNotPresent, // use locally built image.
					Command:         []string{},              // uses entrypoint.
					Args: []string{
						"/usr/sbin/sshd",
						"-De",
					},
					ReadinessProbe: &corev1.Probe{
						ProbeHandler: corev1.ProbeHandler{
							TCPSocket: &corev1.TCPSocketAction{
								Port: intstr.FromInt(2222),
							},
						},
						InitialDelaySeconds: 3,
					},
				},
			}
		})

		ginkgo.When("running as root", func() {
			ginkgo.It("should succeed", func() {
				mpiJob := createJobAndWaitForCompletion(mpiJob)
				expectConditionToBeTrue(mpiJob, kubeflow.JobSucceeded)
			})
		})

		ginkgo.When("running as non-root", func() {
			ginkgo.BeforeEach(func() {
				mpiJob.Spec.SSHAuthMountPath = "/home/mpiuser/.ssh"

				mpiJob.Spec.MPIReplicaSpecs[kubeflow.MPIReplicaTypeLauncher].Template.Spec.Containers[0].SecurityContext = &corev1.SecurityContext{
					RunAsUser: ptr.To[int64](1000),
				}
				workerContainer := &mpiJob.Spec.MPIReplicaSpecs[kubeflow.MPIReplicaTypeWorker].Template.Spec.Containers[0]
				workerContainer.SecurityContext = &corev1.SecurityContext{
					RunAsUser: ptr.To[int64](1000),
				}
				workerContainer.Args = append(workerContainer.Args, "-f", "/home/mpiuser/.sshd_config")
			})

			ginkgo.It("should succeed", func() {
				mpiJob := createJobAndWaitForCompletion(mpiJob)
				expectConditionToBeTrue(mpiJob, kubeflow.JobSucceeded)
			})
		})
	})

	ginkgo.Context("with MPICH Implementation", func() {
		ginkgo.BeforeEach(func() {
			mpiJob.Spec.MPIImplementation = kubeflow.MPIImplementationMPICH
			mpiJob.Spec.MPIReplicaSpecs[kubeflow.MPIReplicaTypeLauncher].Template.Spec.Containers = []corev1.Container{
				{
					Name:            "launcher",
					Image:           mpichImage,
					ImagePullPolicy: corev1.PullIfNotPresent, // use locally built image.
					Command:         []string{},              // uses entrypoint.
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
					Name:            "worker",
					Image:           mpichImage,
					ImagePullPolicy: corev1.PullIfNotPresent, // use locally built image.
					Command:         []string{},              // uses entrypoint.
					Args: []string{
						"/usr/sbin/sshd",
						"-De",
					},
					ReadinessProbe: &corev1.Probe{
						ProbeHandler: corev1.ProbeHandler{
							TCPSocket: &corev1.TCPSocketAction{
								Port: intstr.FromInt(2222),
							},
						},
						InitialDelaySeconds: 3,
					},
				},
			}
		})

		ginkgo.When("running as root", func() {
			ginkgo.It("should succeed", func() {
				mpiJob := createJobAndWaitForCompletion(mpiJob)
				expectConditionToBeTrue(mpiJob, kubeflow.JobSucceeded)
			})
		})

		ginkgo.When("running as non-root", func() {
			ginkgo.BeforeEach(func() {
				mpiJob.Spec.SSHAuthMountPath = "/home/mpiuser/.ssh"

				mpiJob.Spec.MPIReplicaSpecs[kubeflow.MPIReplicaTypeLauncher].Template.Spec.Containers[0].SecurityContext = &corev1.SecurityContext{
					RunAsUser: ptr.To[int64](1000),
				}
				workerContainer := &mpiJob.Spec.MPIReplicaSpecs[kubeflow.MPIReplicaTypeWorker].Template.Spec.Containers[0]
				workerContainer.SecurityContext = &corev1.SecurityContext{
					RunAsUser: ptr.To[int64](1000),
				}
				workerContainer.Args = append(workerContainer.Args, "-f", "/home/mpiuser/.sshd_config")
			})

			ginkgo.It("should succeed", func() {
				mpiJob := createJobAndWaitForCompletion(mpiJob)
				expectConditionToBeTrue(mpiJob, kubeflow.JobSucceeded)
			})
		})
	})

	ginkgo.Context("with scheduler-plugins", func() {
		const enableGangSchedulingFlag = "--gang-scheduling=scheduler-plugins-scheduler"
		var (
			ctx                    = context.Background()
			unschedulableResources = &corev1.ResourceList{
				corev1.ResourceCPU:    resource.MustParse("100000"),   // unschedulable
				corev1.ResourceMemory: resource.MustParse("100000Gi"), // unschedulable
			}
		)

		ginkgo.BeforeEach(func() {
			// Set up the scheduler-plugins.
			setUpSchedulerPlugins()
			// Set up the mpi-operator so that the scheduler-plugins is used as gang-scheduler.
			setupMPIOperator(ctx, mpiJob, enableGangSchedulingFlag, unschedulableResources)
		})

		ginkgo.AfterEach(func() {
			operator, err := k8sClient.AppsV1().Deployments(mpiOperator).Get(ctx, mpiOperator, metav1.GetOptions{})
			oldOperator := operator.DeepCopy()
			gomega.Expect(err).Should(gomega.Succeed())
			for i, arg := range operator.Spec.Template.Spec.Containers[0].Args {
				if arg == enableGangSchedulingFlag {
					operator.Spec.Template.Spec.Containers[0].Args = append(
						operator.Spec.Template.Spec.Containers[0].Args[:i], operator.Spec.Template.Spec.Containers[0].Args[i+1:]...)
					break
				}
			}
			if diff := cmp.Diff(oldOperator, operator); len(diff) != 0 {
				_, err = k8sClient.AppsV1().Deployments(mpiOperator).Update(ctx, operator, metav1.UpdateOptions{})
				gomega.Expect(err).Should(gomega.Succeed())
				gomega.Eventually(func() bool {
					ok, err := ensureDeploymentAvailableReplicas(ctx, mpiOperator, mpiOperator)
					gomega.Expect(err).Should(gomega.Succeed())
					return ok
				}, foreverTimeout, waitInterval).Should(gomega.BeTrue())
			}
			// Clean up the scheduler-plugins.
			cleanUpSchedulerPlugins()
		})

		ginkgo.It("should create pending pods", func() {
			ginkgo.By("Creating MPIJob")
			mpiJob := createJob(ctx, mpiJob)
			var jobCondition *kubeflow.JobCondition
			gomega.Eventually(func() *kubeflow.JobCondition {
				updatedMPIJob, err := mpiClient.KubeflowV2beta1().MPIJobs(mpiJob.Namespace).Get(ctx, mpiJob.Name, metav1.GetOptions{})
				gomega.Expect(err).Should(gomega.Succeed())
				jobCondition = getJobCondition(updatedMPIJob, kubeflow.JobCreated)
				return jobCondition
			}, foreverTimeout, waitInterval).ShouldNot(gomega.BeNil())
			gomega.Expect(jobCondition.Status).To(gomega.Equal(corev1.ConditionTrue))

			ginkgo.By("Waiting for Pods to created")
			var pods *corev1.PodList
			gomega.Eventually(func() error {
				var err error
				pods, err = k8sClient.CoreV1().Pods(mpiJob.Namespace).List(ctx, metav1.ListOptions{
					LabelSelector: labels.FormatLabels(map[string]string{
						schedv1alpha1.PodGroupLabel: mpiJob.Name,
					}),
				})
				return err
			}, foreverTimeout, waitInterval).Should(gomega.BeNil())
			for _, pod := range pods.Items {
				gomega.Expect(pod.Status.Phase).Should(gomega.Equal(corev1.PodPending))
			}
			pg, err := schedClient.SchedulingV1alpha1().PodGroups(mpiJob.Namespace).Get(ctx, mpiJob.Name, metav1.GetOptions{})
			gomega.Expect(err).NotTo(gomega.HaveOccurred())
			gomega.Expect(pg.Spec.MinResources.Cpu().String()).Should(gomega.BeComparableTo(unschedulableResources.Cpu().String()))
			gomega.Expect(pg.Spec.MinResources.Memory().String()).Should(gomega.BeComparableTo(unschedulableResources.Memory().String()))

			ginkgo.By("Updating MPIJob with schedulable schedulingPolicies")
			gomega.Eventually(func() error {
				updatedJob, err := mpiClient.KubeflowV2beta1().MPIJobs(mpiJob.Namespace).Get(ctx, mpiJob.Name, metav1.GetOptions{})
				gomega.Expect(err).Should(gomega.Succeed())
				updatedJob.Spec.RunPolicy.SchedulingPolicy.MinResources = nil
				_, err = mpiClient.KubeflowV2beta1().MPIJobs(updatedJob.Namespace).Update(ctx, updatedJob, metav1.UpdateOptions{})
				return err
			}, foreverTimeout, waitInterval).Should(gomega.BeNil())

			ginkgo.By("Waiting for MPIJob to running")
			gomega.Eventually(func() corev1.ConditionStatus {
				updatedJob, err := mpiClient.KubeflowV2beta1().MPIJobs(mpiJob.Namespace).Get(ctx, mpiJob.Name, metav1.GetOptions{})
				gomega.Expect(err).Should(gomega.Succeed())
				cond := getJobCondition(updatedJob, kubeflow.JobRunning)
				if cond == nil {
					return corev1.ConditionFalse
				}
				return cond.Status
			}, foreverTimeout, waitInterval).Should(gomega.Equal(corev1.ConditionTrue))
		})
	})

	// volcano e2e tests
	ginkgo.Context("with volcano-scheduler", func() {
		const enableGangSchedulingFlag = "--gang-scheduling=volcano"
		var (
			ctx                    = context.Background()
			unschedulableResources = &corev1.ResourceList{
				corev1.ResourceCPU:    resource.MustParse("100000"),   // unschedulable
				corev1.ResourceMemory: resource.MustParse("100000Gi"), // unschedulable
			}
		)

		ginkgo.BeforeEach(func() {
			// Set up the volcano-scheduler.
			setupVolcanoScheduler()
			// Set up the mpi-operator so that the volcano scheduler is used as gang-scheduler.
			setupMPIOperator(ctx, mpiJob, enableGangSchedulingFlag, unschedulableResources)
		})

		ginkgo.AfterEach(func() {
			operator, err := k8sClient.AppsV1().Deployments(mpiOperator).Get(ctx, mpiOperator, metav1.GetOptions{})
			oldOperator := operator.DeepCopy()
			gomega.Expect(err).Should(gomega.Succeed())
			// disable gang-scheduler in operator
			for i, arg := range operator.Spec.Template.Spec.Containers[0].Args {
				if arg == enableGangSchedulingFlag {
					operator.Spec.Template.Spec.Containers[0].Args = append(
						operator.Spec.Template.Spec.Containers[0].Args[:i], operator.Spec.Template.Spec.Containers[0].Args[i+1:]...)
					break
				}
			}
			if diff := cmp.Diff(oldOperator, operator); len(diff) != 0 {
				_, err = k8sClient.AppsV1().Deployments(mpiOperator).Update(ctx, operator, metav1.UpdateOptions{})
				gomega.Expect(err).Should(gomega.Succeed())
				gomega.Eventually(func() bool {
					ok, err := ensureDeploymentAvailableReplicas(ctx, mpiOperator, mpiOperator)
					gomega.Expect(err).Should(gomega.Succeed())
					return ok
				}, foreverTimeout, waitInterval).Should(gomega.BeTrue())
			}
			// Clean up the volcano.
			cleanUpVolcanoScheduler()
		})

		ginkgo.It("should create pending pods", func() {
			ginkgo.By("Creating MPIJob")
			mpiJob := createJob(ctx, mpiJob)
			var jobCondition *kubeflow.JobCondition
			gomega.Eventually(func() *kubeflow.JobCondition {
				updatedMPIJob, err := mpiClient.KubeflowV2beta1().MPIJobs(mpiJob.Namespace).Get(ctx, mpiJob.Name, metav1.GetOptions{})
				gomega.Expect(err).Should(gomega.Succeed())
				jobCondition = getJobCondition(updatedMPIJob, kubeflow.JobCreated)
				return jobCondition
			}, foreverTimeout, waitInterval).ShouldNot(gomega.BeNil())
			gomega.Expect(jobCondition.Status).To(gomega.Equal(corev1.ConditionTrue))

			ginkgo.By("Waiting for Pods to created")
			var pods *corev1.PodList
			gomega.Eventually(func() error {
				var err error
				pods, err = k8sClient.CoreV1().Pods(mpiJob.Namespace).List(ctx, metav1.ListOptions{
					LabelSelector: labels.FormatLabels(map[string]string{
						kubeflow.JobNameLabel: mpiJob.Name,
					}),
				})
				return err
			}, foreverTimeout, waitInterval).Should(gomega.BeNil())
			for _, pod := range pods.Items {
				gomega.Expect(pod.Status.Phase).Should(gomega.Equal(corev1.PodPending))
			}
			pg, err := volcanoClient.SchedulingV1beta1().PodGroups(mpiJob.Namespace).Get(ctx, mpiJob.Name, metav1.GetOptions{})
			gomega.Expect(err).NotTo(gomega.HaveOccurred())
			gomega.Expect(pg.Spec.MinResources.Cpu().String()).Should(gomega.BeComparableTo(unschedulableResources.Cpu().String()))
			gomega.Expect(pg.Spec.MinResources.Memory().String()).Should(gomega.BeComparableTo(unschedulableResources.Memory().String()))

			ginkgo.By("Updating MPIJob with schedulable schedulingPolicies")
			gomega.Eventually(func() error {
				updatedJob, err := mpiClient.KubeflowV2beta1().MPIJobs(mpiJob.Namespace).Get(ctx, mpiJob.Name, metav1.GetOptions{})
				gomega.Expect(err).Should(gomega.Succeed())
				updatedJob.Spec.RunPolicy.SchedulingPolicy.MinResources = nil
				_, err = mpiClient.KubeflowV2beta1().MPIJobs(updatedJob.Namespace).Update(ctx, updatedJob, metav1.UpdateOptions{})
				return err
			}, foreverTimeout, waitInterval).Should(gomega.BeNil())

			ginkgo.By("Waiting for MPIJob to running")
			gomega.Eventually(func() corev1.ConditionStatus {
				updatedJob, err := mpiClient.KubeflowV2beta1().MPIJobs(mpiJob.Namespace).Get(ctx, mpiJob.Name, metav1.GetOptions{})
				gomega.Expect(err).Should(gomega.Succeed())
				cond := getJobCondition(updatedJob, kubeflow.JobRunning)
				if cond == nil {
					return corev1.ConditionFalse
				}
				return cond.Status
			}, foreverTimeout, waitInterval).Should(gomega.Equal(corev1.ConditionTrue))
		})
	})
})

func resumeJob(ctx context.Context, mpiJob *kubeflow.MPIJob) *kubeflow.MPIJob {
	mpiJob.Spec.RunPolicy.Suspend = ptr.To(false)
	ginkgo.By("Resuming MPIJob")
	mpiJob, err := mpiClient.KubeflowV2beta1().MPIJobs(mpiJob.Namespace).Update(ctx, mpiJob, metav1.UpdateOptions{})
	gomega.Expect(err).ToNot(gomega.HaveOccurred())
	return mpiJob
}

func createJobAndWaitForCompletion(mpiJob *kubeflow.MPIJob) *kubeflow.MPIJob {
	ctx := context.Background()
	mpiJob = createJob(ctx, mpiJob)
	return waitForCompletion(ctx, mpiJob)
}

func createJob(ctx context.Context, mpiJob *kubeflow.MPIJob) *kubeflow.MPIJob {
	ginkgo.By("Creating MPIJob")
	mpiJob, err := mpiClient.KubeflowV2beta1().MPIJobs(mpiJob.Namespace).Create(ctx, mpiJob, metav1.CreateOptions{})
	gomega.Expect(err).ToNot(gomega.HaveOccurred())
	return mpiJob
}

func waitForCompletion(ctx context.Context, mpiJob *kubeflow.MPIJob) *kubeflow.MPIJob {
	var err error

	ginkgo.By("Waiting for MPIJob to finish")
	err = wait.PollUntilContextTimeout(ctx, waitInterval, foreverTimeout, false, func(ctx context.Context) (bool, error) {
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

func getLauncherPods(ctx context.Context, mpiJob *kubeflow.MPIJob) (*corev1.PodList, error) {
	selector := metav1.LabelSelector{
		MatchLabels: map[string]string{
			kubeflow.OperatorNameLabel: kubeflow.OperatorName,
			kubeflow.JobNameLabel:      mpiJob.Name,
			kubeflow.JobRoleLabel:      "launcher",
		},
	}
	launcherPods, err := k8sClient.CoreV1().Pods(mpiJob.Namespace).List(ctx, metav1.ListOptions{
		LabelSelector: metav1.FormatLabelSelector(&selector),
	})
	if err != nil {
		return &corev1.PodList{}, fmt.Errorf("getting launcher Pods: %w", err)
	}
	return launcherPods, nil
}

func getWorkerPods(ctx context.Context, mpiJob *kubeflow.MPIJob) (*corev1.PodList, error) {
	selector := metav1.LabelSelector{
		MatchLabels: map[string]string{
			kubeflow.OperatorNameLabel: kubeflow.OperatorName,
			kubeflow.JobNameLabel:      mpiJob.Name,
			kubeflow.JobRoleLabel:      "worker",
		},
	}
	workerPods, err := k8sClient.CoreV1().Pods(mpiJob.Namespace).List(ctx, metav1.ListOptions{
		LabelSelector: metav1.FormatLabelSelector(&selector),
	})
	if err != nil {
		return &corev1.PodList{}, fmt.Errorf("getting worker Pods: %w", err)
	}
	return workerPods, nil
}

func getSecretsForJob(ctx context.Context, mpiJob *kubeflow.MPIJob) (*corev1.Secret, error) {
	result, err := k8sClient.CoreV1().Secrets(mpiJob.Namespace).List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, err
	}
	for _, obj := range result.Items {
		if metav1.IsControlledBy(&obj, mpiJob) {
			return &obj, nil
		}
	}
	return nil, nil
}

func debugJob(ctx context.Context, mpiJob *kubeflow.MPIJob) error {
	launcherPods, err := getLauncherPods(ctx, mpiJob)
	if err != nil {
		return err
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
	workerPods, err := getWorkerPods(ctx, mpiJob)
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

func expectConditionToBeTrue(mpiJob *kubeflow.MPIJob, condType kubeflow.JobConditionType) {
	condition := getJobCondition(mpiJob, condType)
	gomega.Expect(condition).ToNot(gomega.BeNil())
	gomega.Expect(condition.Status).To(gomega.Equal(corev1.ConditionTrue))
}

func getJobCondition(mpiJob *kubeflow.MPIJob, condType kubeflow.JobConditionType) *kubeflow.JobCondition {
	for _, cond := range mpiJob.Status.Conditions {
		if cond.Type == condType {
			return &cond
		}
	}
	return nil
}

func getLauncherJob(ctx context.Context, mpiJob *kubeflow.MPIJob) (*batchv1.Job, error) {
	result, err := k8sClient.BatchV1().Jobs(mpiJob.Namespace).List(ctx, metav1.ListOptions{})
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

func createMPIJobWithOpenMPI(mpiJob *kubeflow.MPIJob) {
	mpiJob.Spec.MPIReplicaSpecs[kubeflow.MPIReplicaTypeLauncher].Template.Spec.Containers = []corev1.Container{
		{
			Name:            "launcher",
			Image:           openMPIImage,
			ImagePullPolicy: corev1.PullIfNotPresent, // use locally built image.
			Command:         []string{"mpirun"},
			Args: []string{
				"-n",
				"2",
				"/home/mpiuser/pi",
			},
		},
	}
	mpiJob.Spec.MPIReplicaSpecs[kubeflow.MPIReplicaTypeWorker].Template.Spec.Containers = []corev1.Container{
		{
			Name:            "worker",
			Image:           openMPIImage,
			ImagePullPolicy: corev1.PullIfNotPresent, // use locally built image.
		},
	}
}

func setUpSchedulerPlugins() {
	if !useExistingSchedulerPlugins {
		ginkgo.By("Installing scheduler-plugins")
		err := installSchedulerPlugins()
		gomega.Expect(err).Should(gomega.Succeed())
	}
}

func cleanUpSchedulerPlugins() {
	if !useExistingSchedulerPlugins {
		ginkgo.By("Uninstalling scheduler-plugins")
		err := runCommand(helmPath, "uninstall", schedulerPlugins, "--namespace", schedulerPlugins)
		gomega.Expect(err).Should(gomega.Succeed())
		err = runCommand(kubectlPath, "delete", "namespace", schedulerPlugins)
		gomega.Expect(err).Should(gomega.Succeed())
	}
}

func setupVolcanoScheduler() {
	if !useExistingVolcanoScheduler {
		ginkgo.By("Installing volcano-scheduler")
		err := installVolcanoScheduler()
		gomega.Expect(err).Should(gomega.Succeed())
	}
}

func cleanUpVolcanoScheduler() {
	if !useExistingVolcanoScheduler {
		ginkgo.By("Uninstalling volcano-scheduler")
		err := runCommand(kubectlPath, "delete", "-f", volcanoSchedulerManifestPath)
		gomega.Expect(err).Should(gomega.Succeed())
	}
}

// setupMPIOperator scales down and scales up the MPIOperator replication so that set up gang-scheduler takes effect
func setupMPIOperator(ctx context.Context, mpiJob *kubeflow.MPIJob, enableGangSchedulingFlag string, unschedulableResources *corev1.ResourceList) {
	ginkgo.By("Scale-In the deployment to 0")
	operator, err := k8sClient.AppsV1().Deployments(mpiOperator).Get(ctx, mpiOperator, metav1.GetOptions{})
	gomega.Expect(err).Should(gomega.Succeed())
	operator.Spec.Replicas = ptr.To[int32](0)
	_, err = k8sClient.AppsV1().Deployments(mpiOperator).Update(ctx, operator, metav1.UpdateOptions{})
	gomega.Expect(err).Should(gomega.Succeed())
	gomega.Eventually(func() bool {
		isNotZero, err := ensureDeploymentAvailableReplicas(ctx, mpiOperator, mpiOperator)
		gomega.Expect(err).Should(gomega.Succeed())
		return isNotZero
	}, foreverTimeout, waitInterval).Should(gomega.BeFalse())

	ginkgo.By("Update the replicas and args")
	gomega.Eventually(func() error {
		updatedOperator, err := k8sClient.AppsV1().Deployments(mpiOperator).Get(ctx, mpiOperator, metav1.GetOptions{})
		gomega.Expect(err).Should(gomega.Succeed())
		updatedOperator.Spec.Template.Spec.Containers[0].Args = append(updatedOperator.Spec.Template.Spec.Containers[0].Args, enableGangSchedulingFlag)
		updatedOperator.Spec.Replicas = ptr.To[int32](1)
		_, err = k8sClient.AppsV1().Deployments(mpiOperator).Update(ctx, updatedOperator, metav1.UpdateOptions{})
		return err
	}, foreverTimeout, waitInterval).Should(gomega.BeNil())

	ginkgo.By("Should be replicas is 1")
	gomega.Eventually(func() bool {
		isNotZero, err := ensureDeploymentAvailableReplicas(ctx, mpiOperator, mpiOperator)
		gomega.Expect(err).Should(gomega.Succeed())
		return isNotZero
	}, foreverTimeout, waitInterval).Should(gomega.BeTrue())
	createMPIJobWithOpenMPI(mpiJob)
	mpiJob.Spec.RunPolicy.SchedulingPolicy = &kubeflow.SchedulingPolicy{MinResources: unschedulableResources}
}
