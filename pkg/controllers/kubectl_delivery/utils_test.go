// Copyright 2020 The Kubeflow Authors.
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

package kubectl_delivery

import (
	"bufio"
	"fmt"
	"os"
	"path"
	"path/filepath"
	"strings"
	"testing"
	"time"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime"
	kubeinformers "k8s.io/client-go/informers"
	k8sfake "k8s.io/client-go/kubernetes/fake"
)

var (
	alwaysReady        = func() bool { return true }
	noResyncPeriodFunc = func() time.Duration { return 0 }
)

type fixture struct {
	t *testing.T

	kubeClient *k8sfake.Clientset

	// Objects to put in the store.
	podLister []*corev1.Pod

	// Objects from here are pre-loaded into NewSimpleFake.
	kubeObjects []runtime.Object
}

func newFixture(t *testing.T) *fixture {
	f := &fixture{}
	f.t = t
	f.kubeObjects = []runtime.Object{}
	return f
}

func (f *fixture) newController(namespace string, pods []string) (*KubectlDeliveryController, kubeinformers.SharedInformerFactory) {
	f.kubeClient = k8sfake.NewSimpleClientset(f.kubeObjects...)

	k8sI := kubeinformers.NewSharedInformerFactory(f.kubeClient, noResyncPeriodFunc())
	c := NewKubectlDeliveryController(
		namespace,
		f.kubeClient,
		k8sI.Core().V1().Pods(),
		pods,
	)

	c.podSynced = alwaysReady

	for _, pod := range f.podLister {
		err := k8sI.Core().V1().Pods().Informer().GetIndexer().Add(pod)
		if err != nil {
			fmt.Println("Failed to create pod")
		}
	}

	return c, k8sI
}

func (f *fixture) run(namespace, podName string) {
	f.runController(namespace, podName, true, false)
}

func (f *fixture) runController(namespace, pod string, startInformers bool, expectError bool) {
	c, k8sI := f.newController(namespace, []string{pod})
	if startInformers {
		stopCh := make(chan struct{})
		defer close(stopCh)
		k8sI.Start(stopCh)
	}

	err := c.syncHandler(path.Join(namespace, "/", pod))
	if !expectError && err != nil {
		f.t.Errorf("error syncing pod: %v", err)
	} else if expectError && err == nil {
		f.t.Error("expected error syncing pod, got nil")
	}

	c.lock.Lock()
	defer c.lock.Unlock()
	if len(c.watchedPods) != 0 {
		if expectError {
			f.t.Errorf("Expected watched pods is not nil, but left %v", c.watchedPods)
		} else {
			f.t.Errorf("Expected watched pods is nil, but left %v", c.watchedPods)
		}
	}
}

func (f *fixture) setUpPods(p *corev1.Pod) {
	f.podLister = append(f.podLister, p)
	f.kubeObjects = append(f.kubeObjects, p)
}

// getResolvedHosts will resolve the hosts file to a map object,
// with the hostname as key and IP address as value
func (f *fixture) getResolvedHosts(contentBytes []byte) map[string]string {
	// create a scanner to read content line by line
	hostRecords := make(map[string]string)
	contentStrReader := strings.NewReader(string(contentBytes))
	scanner := bufio.NewScanner(contentStrReader)
	for scanner.Scan() {
		line := scanner.Text()
		if line[0] == '#' { // skip the comment line
			continue
		}
		lines := strings.Fields(line)
		if len(lines) == 0 { // skip the space line
			continue
		}
		if len(lines) == 1 { // the format must has some mistakes
			f.t.Error("Error, generated hosts file has wrong format.")
			continue
		}
		for i := 1; i < len(lines); i++ {
			hostRecords[lines[i]] = lines[0] // use map to record hosts
		}
	}
	return hostRecords
}

// setUpTmpDir will create a temp directory and create a temp hosts file
// with provided file content, and return the path of the directory.
func (f *fixture) setUpTmpDir(dirName string, content []byte) (string, string) {
	p, err := os.MkdirTemp(os.TempDir(), "hosts")
	if err != nil {
		f.t.Fatal(err)
	}
	tmphf := filepath.Join(p, "hosts")
	if err := os.WriteFile(tmphf, content, 0644); err != nil {
		f.t.Fatal(err)
	}
	return p, tmphf
}
