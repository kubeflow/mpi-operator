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
	"os"
	"os/exec"
	"testing"
	"time"

	"github.com/onsi/ginkgo"
	"github.com/onsi/gomega"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/kubernetes"
	controllerruntime "sigs.k8s.io/controller-runtime"
	schedclientset "sigs.k8s.io/scheduler-plugins/pkg/generated/clientset/versioned"
	volcanoclient "volcano.sh/apis/pkg/client/clientset/versioned"

	clientset "github.com/kubeflow/mpi-operator/pkg/client/clientset/versioned"
)

const (
	envUseExistingCluster      = "USE_EXISTING_CLUSTER"
	envUseExistingOperator     = "USE_EXISTING_OPERATOR"
	envTestMPIOperatorImage    = "TEST_MPI_OPERATOR_IMAGE"
	envTestOpenMPIImage        = "TEST_OPENMPI_IMAGE"
	envTestIntelMPIImage       = "TEST_INTELMPI_IMAGE"
	envTestMPICHImage          = "TEST_MPICH_IMAGE"
	envTestKindImage           = "TEST_KIND_IMAGE"
	envSchedulerPluginsVersion = "SCHEDULER_PLUGINS_VERSION"
	envVolcanoSchedulerVersion = "VOLCANO_SCHEDULER_VERSION"

	defaultMPIOperatorImage = "mpioperator/mpi-operator:local"
	defaultKindImage        = "kindest/node:v1.31.0"
	defaultOpenMPIImage     = "mpioperator/mpi-pi:openmpi"
	defaultIntelMPIImage    = "mpioperator/mpi-pi:intel"
	defaultMPICHImage       = "mpioperator/mpi-pi:mpich"
	rootPath                = "../.."
	kubectlPath             = rootPath + "/bin/kubectl"
	kindPath                = rootPath + "/bin/kind"
	helmPath                = rootPath + "/bin/helm"
	operatorManifestsPath   = rootPath + "/manifests/overlays/dev"

	schedulerPluginsManifestPath   = rootPath + "/dep-manifests/scheduler-plugins/"
	volcanoSchedulerManifestPath   = rootPath + "/dep-manifests/volcano-scheduler/" // all in one yaml of volcano-development.yaml
	envUseExistingSchedulerPlugins = "USE_EXISTING_SCHEDULER_PLUGINS"
	envUseExistingVolcanoScheduler = "USE_EXISTING_VOLCANO_SCHEDULER"
	defaultSchedulerPluginsVersion = "v0.29.8"
	defaultVolcanoSchedulerVersion = "v1.10.0"

	mpiOperator      = "mpi-operator"
	schedulerPlugins = "scheduler-plugins"

	waitInterval   = 500 * time.Millisecond
	foreverTimeout = 200 * time.Second
)

var (
	useExistingCluster          bool
	useExistingOperator         bool
	useExistingSchedulerPlugins bool
	useExistingVolcanoScheduler bool
	mpiOperatorImage            string
	openMPIImage                string
	intelMPIImage               string
	mpichImage                  string
	kindImage                   string
	schedulerPluginsVersion     string
	volcanoSchedulerVersion     string

	k8sClient     kubernetes.Interface
	mpiClient     clientset.Interface
	schedClient   schedclientset.Interface
	volcanoClient volcanoclient.Interface
)

func init() {
	useExistingCluster = getEnvDefault(envUseExistingCluster, "false") == "true"
	useExistingOperator = getEnvDefault(envUseExistingOperator, "false") == "true"
	useExistingSchedulerPlugins = getEnvDefault(envUseExistingSchedulerPlugins, "false") == "true"
	useExistingVolcanoScheduler = getEnvDefault(envUseExistingVolcanoScheduler, "false") == "true"
	mpiOperatorImage = getEnvDefault(envTestMPIOperatorImage, defaultMPIOperatorImage)
	openMPIImage = getEnvDefault(envTestOpenMPIImage, defaultOpenMPIImage)
	intelMPIImage = getEnvDefault(envTestIntelMPIImage, defaultIntelMPIImage)
	mpichImage = getEnvDefault(envTestMPICHImage, defaultMPICHImage)
	kindImage = getEnvDefault(envTestKindImage, defaultKindImage)
	schedulerPluginsVersion = getEnvDefault(envSchedulerPluginsVersion, defaultSchedulerPluginsVersion)
	volcanoSchedulerVersion = getEnvDefault(envVolcanoSchedulerVersion, defaultVolcanoSchedulerVersion)
}

func TestE2E(t *testing.T) {
	gomega.RegisterFailHandler(ginkgo.Fail)
	ginkgo.RunSpecs(t, "E2e Suite")
}

var _ = ginkgo.SynchronizedBeforeSuite(func() []byte {
	if !useExistingCluster {
		ginkgo.By("Creating a local cluster")
		err := bootstrapKindCluster()
		gomega.Expect(err).ToNot(gomega.HaveOccurred())
	}
	ginkgo.By("Obtaining clients")
	restConfig, err := controllerruntime.GetConfig()
	gomega.Expect(err).ToNot(gomega.HaveOccurred())

	k8sClient, err = kubernetes.NewForConfig(restConfig)
	gomega.Expect(err).ToNot(gomega.HaveOccurred())

	mpiClient, err = clientset.NewForConfig(restConfig)
	gomega.Expect(err).ToNot(gomega.HaveOccurred())

	schedClient, err = schedclientset.NewForConfig(restConfig)
	gomega.Expect(err).ToNot(gomega.HaveOccurred())

	volcanoClient, err = volcanoclient.NewForConfig(restConfig)
	gomega.Expect(err).NotTo(gomega.HaveOccurred())

	if !useExistingOperator {
		ginkgo.By("Installing operator")
		err = installOperator()
		gomega.Expect(err).ToNot(gomega.HaveOccurred())
	}
	return nil
}, func([]byte) {})

var _ = ginkgo.SynchronizedAfterSuite(func() {
	if !useExistingCluster {
		ginkgo.By("Deleting local cluster")
		err := runCommand(kindPath, "delete", "cluster")
		gomega.Expect(err).ToNot(gomega.HaveOccurred())
	} else if !useExistingOperator {
		ginkgo.By("Uninstalling operator")
		err := runCommand(kubectlPath, "delete", "-k", operatorManifestsPath)
		gomega.Expect(err).ToNot(gomega.HaveOccurred())
	}
}, func() {})

func getEnvDefault(key, defaultVal string) string {
	v, ok := os.LookupEnv(key)
	if ok {
		return v
	}
	return defaultVal
}

func bootstrapKindCluster() error {
	err := runCommand(kindPath, "create", "cluster", "--image", kindImage)
	if err != nil {
		return fmt.Errorf("creating kind cluster: %w", err)
	}
	err = runCommand(kindPath, "load", "docker-image", mpiOperatorImage, openMPIImage, intelMPIImage, mpichImage)
	if err != nil {
		return fmt.Errorf("loading container images: %w", err)
	}
	return nil
}

func installOperator() error {
	err := runCommand(kubectlPath, "apply", "--server-side", "-k", operatorManifestsPath)
	if err != nil {
		return fmt.Errorf("applying operator YAMLs: %w", err)
	}
	return wait.PollUntilContextTimeout(context.Background(), waitInterval, foreverTimeout, false, func(ctx context.Context) (bool, error) {
		return ensureDeploymentAvailableReplicas(ctx, mpiOperator, mpiOperator)
	})
}

func installSchedulerPlugins() error {
	// TODO: Remove flags to overwrite images once the scheduler-plugins with the appropriate helm charts is released.
	// In the specific scheudler-plugins version such as v0.26.7, manifests are incorrect.
	// So we overwrite images.
	overwriteControllerImage := fmt.Sprintf("controller.image=registry.k8s.io/scheduler-plugins/controller:%s", schedulerPluginsVersion)
	overwriteSchedulerImage := fmt.Sprintf("scheduler.image=registry.k8s.io/scheduler-plugins/kube-scheduler:%s", schedulerPluginsVersion)
	err := runCommand(helmPath, "install", "scheduler-plugins", schedulerPluginsManifestPath, "--create-namespace", "--namespace", schedulerPlugins,
		"--set", overwriteSchedulerImage, "--set", overwriteControllerImage)
	if err != nil {
		return fmt.Errorf("installing scheduler-plugins Helm Chart: %w", err)
	}
	return wait.PollUntilContextTimeout(context.Background(), waitInterval, foreverTimeout, false, func(ctx context.Context) (bool, error) {
		controllerName := fmt.Sprintf("%s-controller", schedulerPlugins)
		if ok, err := ensureDeploymentAvailableReplicas(ctx, schedulerPlugins, controllerName); !ok || err != nil {
			return false, err
		}
		schedulerName := fmt.Sprintf("%s-scheduler", schedulerPlugins)
		if ok, err := ensureDeploymentAvailableReplicas(ctx, schedulerPlugins, schedulerName); !ok || err != nil {
			return false, err
		}
		return true, nil
	})
}

func installVolcanoScheduler() error {
	err := runCommand(kubectlPath, "apply", "-f", volcanoSchedulerManifestPath)
	if err != nil {
		return fmt.Errorf("failed to install volcano scheduler : %w", err)
	}

	volcanoNamespace := "volcano-system"
	return wait.PollUntilContextTimeout(context.Background(), waitInterval, foreverTimeout, false, func(ctx context.Context) (bool, error) {
		if ok, err := ensureDeploymentAvailableReplicas(ctx, volcanoNamespace, "volcano-scheduler"); !ok || err != nil {
			return false, err
		}
		if ok, err := ensureDeploymentAvailableReplicas(ctx, volcanoNamespace, "volcano-controllers"); !ok || err != nil {
			return false, err
		}
		if ok, err := ensureDeploymentAvailableReplicas(ctx, volcanoNamespace, "volcano-admission"); !ok || err != nil {
			return false, err
		}
		return true, nil
	})
}

func runCommand(name string, args ...string) error {
	cmd := exec.Command(name, args...)
	cmd.Stderr = os.Stderr
	cmd.Stdout = os.Stdout
	return cmd.Run()
}

func ensureDeploymentAvailableReplicas(ctx context.Context, namespace, name string) (bool, error) {
	deployment, err := k8sClient.AppsV1().Deployments(namespace).Get(ctx, name, metav1.GetOptions{})
	if errors.IsNotFound(err) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	return deployment.Status.AvailableReplicas != 0, nil
}
