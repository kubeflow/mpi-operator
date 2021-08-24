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
	"os"
	"os/exec"
	"path"
	"testing"
	"time"

	clientset "github.com/kubeflow/mpi-operator/v2/pkg/client/clientset/versioned"
	"github.com/onsi/ginkgo"
	"github.com/onsi/gomega"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/kubernetes"
	controllerruntime "sigs.k8s.io/controller-runtime"
)

const (
	envUseExistingCluster   = "USE_EXISTING_CLUSTER"
	envUseExistingOperator  = "USE_EXISTING_OPERATOR"
	envTestMPIOperatorImage = "TEST_MPI_OPERATOR_IMAGE"
	envTestKindImage        = "TEST_KIND_IMAGE"

	defaultMPIOperatorImage = "mpioperator/mpi-operator:local"
	defaultKindImage        = "kindest/node:v1.21.2"
	openMPIImage            = "mpioperator/mpi-pi:openmpi"
	intelMPIImage           = "mpioperator/mpi-pi:intel"
	rootPath                = "../../.."
	kubectlPath             = rootPath + "/bin/kubectl"
	operatorManifestsPath   = rootPath + "/manifests/overlays/dev"

	mpiOperator = "mpi-operator"

	waitInterval   = 500 * time.Millisecond
	foreverTimeout = 200 * time.Second
)

var (
	useExistingCluster  bool
	useExistingOperator bool
	kindPath            string
	mpiOperatorImage    string
	kindImage           string

	k8sClient kubernetes.Interface
	mpiClient clientset.Interface
)

func init() {
	useExistingCluster = getEnvDefault(envUseExistingCluster, "false") == "true"
	useExistingOperator = getEnvDefault(envUseExistingOperator, "false") == "true"
	mpiOperatorImage = getEnvDefault(envTestMPIOperatorImage, defaultMPIOperatorImage)
	kindImage = getEnvDefault(envTestKindImage, defaultKindImage)
	kindPath = "kind"
	if goPath := os.Getenv("GOPATH"); goPath != "" {
		kindPath = path.Join(goPath, "bin", "kind")
	}
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
	err = runCommand(kindPath, "load", "docker-image", mpiOperatorImage, openMPIImage, intelMPIImage)
	if err != nil {
		return fmt.Errorf("loading container images: %w", err)
	}
	return nil
}

func installOperator() error {
	err := runCommand(kubectlPath, "apply", "-k", operatorManifestsPath)
	if err != nil {
		return fmt.Errorf("applying operator YAMLs: %w", err)
	}
	ctx := context.Background()
	return wait.Poll(waitInterval, foreverTimeout, func() (bool, error) {
		deployment, err := k8sClient.AppsV1().Deployments(mpiOperator).Get(ctx, mpiOperator, metav1.GetOptions{})
		if errors.IsNotFound(err) {
			return false, nil
		}
		if err != nil {
			return false, err
		}
		return deployment.Status.AvailableReplicas != 0, nil
	})
}

func runCommand(name string, args ...string) error {
	cmd := exec.Command(name, args...)
	cmd.Stderr = os.Stderr
	cmd.Stdout = os.Stdout
	return cmd.Run()
}
