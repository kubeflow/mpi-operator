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
	"io/ioutil"
	"os"
	"path/filepath"
	"strconv"
	"testing"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// Test successful task
func TestWaitingPodRunning(t *testing.T) {
	var (
		namespace string = "default"
		podName   string = "test"
	)
	f := newFixture(t)
	fakePod := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: namespace,
			Name:      podName,
		},
	}
	fakePod.Status.Phase = corev1.PodRunning
	f.setUpPods(fakePod)
	f.run(namespace, podName)
}

// Test hosts file generating function
func TestGeneratingHostsFile(t *testing.T) {
	namespace := "default"
	podNames := []string{"test", "tester-2", "worker_3", "pod4"}
	// content of fake local hosts file
	content := []byte(`# this line is a comment
	127.0.0.1       localhost
	::1							localhost
	234.98.76.54		uselesshost
	10.234.56.78		launcher.fullname.test	launcher`)

	// content of excepted outputs
	exceptedHosts := make(map[string]string)
	exceptedHosts["launcher"] = "10.234.56.78"
	exceptedHosts["launcher.fullname.test"] = "10.234.56.78"
	f := newFixture(t)
	// add fake worker pods
	fakePod := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: namespace,
		},
	}
	fakePod.Status.Phase = corev1.PodRunning
	for index := range podNames {
		fakePod.Status.PodIP = "123.123.123." + strconv.Itoa(index)
		fakePod.ObjectMeta.Name = podNames[index]
		exceptedHosts[fakePod.ObjectMeta.Name] = fakePod.Status.PodIP
		f.setUpPods(fakePod.DeepCopy())
	}
	// set up temp directory for testing about files
	p, tmphf := f.setUpTmpDir("hostsGenerating", content)
	defer os.RemoveAll(p) // clean up the temp directory
	tmpof := filepath.Join(p, "output")
	c, _ := f.newController(namespace, podNames)
	// generate new hosts file
	err := c.generateHosts(tmphf, tmpof, podNames)
	if err != nil {
		t.Errorf("Error, cannot generating hosts of worker pods, errs: %v", err)
	}
	// get the output file content
	outputContent, err := ioutil.ReadFile(tmpof)
	if err != nil {
		t.Fatal(err)
	}
	// slice the content of output to avoid space/tab interference
	generatedHosts := f.getResolvedHosts(outputContent)
	// check the output
	for hostname, exceptedIP := range exceptedHosts {
		if resolvedIP, ok := generatedHosts[hostname]; !ok || resolvedIP != exceptedIP {
			t.Errorf("Error, generated hosts file incorrect. Host: %s, excepted: %s, resolved: %s", hostname, exceptedIP, resolvedIP)
		}
	}
}
