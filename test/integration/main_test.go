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

package integration

import (
	"container/list"
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/apimachinery/pkg/watch"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"sigs.k8s.io/controller-runtime/pkg/envtest"
	schedclientset "sigs.k8s.io/scheduler-plugins/pkg/generated/clientset/versioned"
	volcanoclient "volcano.sh/apis/pkg/client/clientset/versioned"

	clientset "github.com/kubeflow/mpi-operator/pkg/client/clientset/versioned"
	"github.com/kubeflow/mpi-operator/test/util"
)

var (
	restConfig *rest.Config
)

func TestMain(m *testing.M) {
	env := &envtest.Environment{
		CRDDirectoryPaths: []string{
			filepath.Join("..", "..", "manifests", "base"),
			filepath.Join("..", "..", "dep-crds", "scheduler-plugins"),
			filepath.Join("..", "..", "dep-crds", "volcano-scheduler"),
		},
	}
	var err error
	restConfig, err = env.Start()
	if err != nil {
		panic(fmt.Sprintf("Failed to start envtest.Environment: %v", err))
	}

	code := m.Run()

	if err = env.Stop(); err != nil {
		panic(fmt.Sprintf("Failed to stop envtest.Environment: %v", err))
	}

	os.Exit(code)
}

type testSetup struct {
	kClient          kubernetes.Interface
	mpiClient        clientset.Interface
	namespace        string
	events           *eventChecker
	gangSchedulerCfg *gangSchedulerConfig
}

type gangSchedulerConfig struct {
	schedulerName string
	volcanoClient volcanoclient.Interface
	schedClient   schedclientset.Interface
}

func newTestSetup(ctx context.Context, t *testing.T) testSetup {
	t.Helper()
	kubeClient, err := kubernetes.NewForConfig(restConfig)
	if err != nil {
		t.Fatalf("Creating kubernetes client: %v", err)
	}
	mpiClient, err := clientset.NewForConfig(restConfig)
	if err != nil {
		t.Fatalf("Creating MPI client: %v", err)
	}
	volcanoClient, err := volcanoclient.NewForConfig(restConfig)
	if err != nil {
		t.Fatalf("Creating Volcano client: %v", err)
	}
	schedClient, err := schedclientset.NewForConfig(restConfig)
	if err != nil {
		t.Fatalf("Creating scheduler-plugins client: %v", err)
	}
	ns, cleanup, err := createTestNamespace(ctx, kubeClient)
	if err != nil {
		t.Fatalf("Creating test namespace: %v", err)
	}
	t.Cleanup(cleanup)
	t.Logf("Using namespace %s", ns)

	evChecker, stop, err := newEventChecker(ctx, kubeClient, ns)
	if err != nil {
		t.Fatalf("Failed to create an event watcher: %v", err)
	}
	t.Cleanup(stop)
	go evChecker.run()
	return testSetup{
		kClient:   kubeClient,
		mpiClient: mpiClient,
		namespace: ns,
		events:    evChecker,
		gangSchedulerCfg: &gangSchedulerConfig{
			volcanoClient: volcanoClient,
			schedClient:   schedClient,
		},
	}
}

func createTestNamespace(ctx context.Context, client kubernetes.Interface) (string, func(), error) {
	ns := &corev1.Namespace{
		ObjectMeta: metav1.ObjectMeta{
			GenerateName: "test-",
		},
	}
	var err error
	ns, err = client.CoreV1().Namespaces().Create(ctx, ns, metav1.CreateOptions{})
	if err != nil {
		return "", nil, err
	}
	cleanup := func() {
		_ = client.CoreV1().Namespaces().Delete(context.Background(), ns.Name, metav1.DeleteOptions{})
	}
	return ns.Name, cleanup, nil
}

type eventChecker struct {
	sync.Mutex
	ch       <-chan watch.Event
	expected list.List
}

func newEventChecker(ctx context.Context, client kubernetes.Interface, ns string) (*eventChecker, func(), error) {
	watch, err := client.CoreV1().Events(ns).Watch(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, nil, err
	}
	stop := func() {
		watch.Stop()
	}
	return &eventChecker{
		ch: watch.ResultChan(),
	}, stop, nil
}

func (c *eventChecker) expect(ev corev1.Event) {
	c.Lock()
	c.expected.PushBack(&ev)
	c.Unlock()
}

func (c *eventChecker) run() {
	for {
		watchEvent, more := <-c.ch
		if !more {
			break
		}
		if watchEvent.Type != watch.Added {
			continue
		}
		ev, ok := watchEvent.Object.(*corev1.Event)
		if !ok {
			continue
		}
		c.Lock()
		if c.expected.Len() != 0 {
			front := c.expected.Front()
			diff := cmp.Diff(front.Value, ev, cmpopts.IgnoreTypes(metav1.ObjectMeta{}), cmpopts.IgnoreFields(corev1.Event{}, "FirstTimestamp", "LastTimestamp", "Count", "Message"), cmpopts.IgnoreFields(corev1.ObjectReference{}, "ResourceVersion"))
			if diff == "" {
				c.expected.Remove(front)
			}
		}
		c.Unlock()
	}
}

func (c *eventChecker) verify(t *testing.T) {
	t.Helper()
	err := wait.PollUntilContextTimeout(context.Background(), util.WaitInterval, wait.ForeverTestTimeout, false, func(ctx context.Context) (bool, error) {
		c.Lock()
		defer c.Unlock()
		return c.expected.Len() == 0, nil
	})
	if err != nil {
		for v := c.expected.Front(); v != nil; v = v.Next() {
			t.Errorf("Unsatisfied event %s", v.Value)
		}
	}
}
