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
	"sync"
	"time"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/apimachinery/pkg/util/wait"
	coreinformers "k8s.io/client-go/informers/core/v1"
	"k8s.io/client-go/kubernetes"
	corelisters "k8s.io/client-go/listers/core/v1"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/util/workqueue"
	"k8s.io/klog"
)

// KubectlDeliveryController
type KubectlDeliveryController struct {
	namespace string
	// kubeClient is a standard kubernetes clientset.
	kubeClient kubernetes.Interface

	podLister corelisters.PodLister
	podSynced cache.InformerSynced

	// queue is a rate limited work queue. This is used to queue work to be
	// processed instead of performing it as soon as a change happens. This
	// means we can ensure we only process a fixed amount of resources at a
	// time, and makes it easy to ensure we are never processing the same item
	// simultaneously in two different workers.
	queue workqueue.RateLimitingInterface

	lock        sync.Mutex
	watchedPods map[string]struct{}
}

// NewKubectlDeliveryController
func NewKubectlDeliveryController(
	ns string,
	kubeClient kubernetes.Interface,
	podInformer coreinformers.PodInformer,
	pods []string,
) *KubectlDeliveryController {

	controller := &KubectlDeliveryController{
		namespace:   ns,
		kubeClient:  kubeClient,
		podLister:   podInformer.Lister(),
		podSynced:   podInformer.Informer().HasSynced,
		queue:       workqueue.NewNamedRateLimitingQueue(workqueue.DefaultControllerRateLimiter(), "mpi-operator-kubectl-delivery"),
		watchedPods: make(map[string]struct{}),
	}

	controller.lock.Lock()
	for _, pn := range pods {
		controller.watchedPods[pn] = struct{}{}
	}
	controller.lock.Unlock()

	klog.Infof("watched pods: %v", pods)
	klog.Info("Setting up event handlers")
	// Set up an event handler for when MPIJob resources change.
	podInformer.Informer().AddEventHandler(cache.FilteringResourceEventHandler{
		FilterFunc: func(obj interface{}) bool {
			pod, ok := obj.(*corev1.Pod)
			if !ok {
				return false
			}

			controller.lock.Lock()
			defer controller.lock.Unlock()
			if _, ok := controller.watchedPods[pod.Name]; ok {
				return true
			}
			return false
		},
		Handler: cache.ResourceEventHandlerFuncs{
			UpdateFunc: func(old, new interface{}) {
				controller.enqueue(new)
			},
		},
	})

	return controller
}

// Run will set up the event handlers for types we are interested in, as well
// as syncing informer caches and starting workers. It will block until stopCh
// is closed, at which point it will shutdown the work queue and wait for
// workers to finish processing their current work items.
func (c *KubectlDeliveryController) Run(threadiness int, stopCh <-chan struct{}) error {
	defer runtime.HandleCrash()
	defer c.queue.ShutDown()

	// Start the informer factories to begin populating the informer caches.
	klog.Info("Starting kubectl-delivery controller")

	// Wait for the caches to be synced before starting workers.
	klog.Info("Waiting for informer caches to sync")
	if ok := cache.WaitForCacheSync(stopCh, c.podSynced); !ok {
		return fmt.Errorf("failed to wait for caches to sync")
	}
	// Copy a list of pods to get their ip address
	var workerPods []string
	for name := range c.watchedPods {
		workerPods = append(workerPods, name)
		pod, err := c.podLister.Pods(c.namespace).Get(name)
		if err != nil {
			continue
		}
		if pod.Status.Phase == corev1.PodRunning && isPodReadyConditionTrue(pod.Status) {
			c.lock.Lock()
			delete(c.watchedPods, pod.Name)
			c.lock.Unlock()
		}
	}
	klog.Info("Starting workers")
	// Launch workers to process MPIJob resources.
	for i := 0; i < threadiness; i++ {
		go wait.Until(c.runWorker, 100*time.Millisecond, stopCh)
	}

	klog.Info("Started workers")
	ticker := time.NewTicker(500 * time.Millisecond)
	for {
		select {
		case <-stopCh:
			return nil
		case <-ticker.C:
			if len(c.watchedPods) == 0 {
				if err := c.generateHosts("/etc/hosts", "/opt/kube/hosts", workerPods); err != nil {
					return fmt.Errorf("Error generating hosts file: %v", err)
				}
				klog.Info("Shutting down workers")
				return nil
			}
			break
		}
	}
}

// generateHosts will get and record all workers' ip address in a hosts-format
// file, which would be sent to each worker pod before launching the remote
// process manager. It will create and write file to filePath, and will use
// pod lister to get ip address, so syncing is required before this.
func (c *KubectlDeliveryController) generateHosts(localHostsPath string, filePath string, workerPods []string) error {
	var hosts string
	// First, open local hosts file to read launcher pod ip
	fd, err := os.Open(localHostsPath)
	if err != nil {
		return fmt.Errorf("can't open file[%s]: %v", localHostsPath, err)
	}
	defer fd.Close()
	// Read the last line of hosts file -- the ip address of localhost
	scanner := bufio.NewScanner(fd)
	for scanner.Scan() {
		hosts = scanner.Text()
	}
	// Use client-go to find up ip addresses of each node
	for index := range workerPods {
		pod, err := c.podLister.Pods(c.namespace).Get(workerPods[index])
		if err != nil {
			return fmt.Errorf("can't get IP address of node[%s]", workerPods[index])
		}
		hosts = fmt.Sprintf("%s\n%s\t%s", hosts, pod.Status.PodIP, pod.Name)
	}
	// Write the hosts-format ip record to volume, and will be sent to worker later.
	fp, err := os.OpenFile(filePath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
	if err != nil {
		return fmt.Errorf("can't create file[%s]: %v", filePath, err)
	}
	defer fp.Close()
	if _, err := fp.WriteString(hosts); err != nil {
		return fmt.Errorf("can't write file[%s]: %v", filePath, err)
	}
	return nil
}

// runWorker is a long-running function that will continually call the
// processNextWorkItem function in order to read and process a message on the
// work queue.
func (c *KubectlDeliveryController) runWorker() {
	for c.processNextWorkItem() {
	}
}

// processNextWorkItem will read a single work item off the work queue and
// attempt to process it, by calling the syncHandler.
func (c *KubectlDeliveryController) processNextWorkItem() bool {
	obj, shutdown := c.queue.Get()

	if shutdown {
		return false
	}

	// We wrap this block in a func so we can defer c.queue.Done.
	err := func(obj interface{}) error {
		// We call Done here so the work queue knows we have finished
		// processing this item. We also must remember to call Forget if we
		// do not want this work item being re-queued. For example, we do
		// not call Forget if a transient error occurs, instead the item is
		// put back on the work queue and attempted again after a back-off
		// period.
		defer c.queue.Done(obj)
		var key string
		var ok bool
		// We expect strings to come off the work queue. These are of the
		// form namespace/name. We do this as the delayed nature of the
		// work queue means the items in the informer cache may actually be
		// more up to date that when the item was initially put onto the
		// work queue.
		if key, ok = obj.(string); !ok {
			// As the item in the work queue is actually invalid, we call
			// Forget here else we'd go into a loop of attempting to
			// process a work item that is invalid.
			c.queue.Forget(obj)
			runtime.HandleError(fmt.Errorf("expected string in workqueue but got %#v", obj))
			return nil
		}
		// Run the syncHandler, passing it the namespace/name string of the
		// MPIJob resource to be synced.
		if err := c.syncHandler(key); err != nil {
			c.queue.AddRateLimited(key)
			return fmt.Errorf("error syncing '%s': %s", key, err.Error())
		}
		// Finally, if no error occurs we Forget this item so it does not
		// get queued again until another change happens.
		c.queue.Forget(obj)
		klog.Infof("Successfully synced '%s'", key)
		return nil
	}(obj)

	if err != nil {
		runtime.HandleError(err)
		return true
	}
	return true
}

// syncHandler compares the actual state with the desired, and attempts to
// converge the two. It then updates the Status block of the MPIJob resource
// with the current status of the resource.
func (c *KubectlDeliveryController) syncHandler(key string) error {
	startTime := time.Now()
	defer func() {
		klog.Infof("Finished syncing job %q (%v)", key, time.Since(startTime))
	}()

	// Convert the namespace/name string into a distinct namespace and name.
	namespace, name, err := cache.SplitMetaNamespaceKey(key)
	if err != nil {
		runtime.HandleError(fmt.Errorf("invalid resource key: %s", key))
		return nil
	}
	if len(namespace) == 0 || len(name) == 0 {
		return fmt.Errorf("invalid job key %q: either namespace or name is missing", key)
	}

	// Get the Pod with this namespace/name.
	pod, err := c.podLister.Pods(namespace).Get(name)
	if err != nil {
		// The MPIJob may no longer exist, in which case we stop processing.
		if errors.IsNotFound(err) {
			klog.V(4).Infof("Pod has been deleted: %v", key)
			return nil
		}
		return err
	}

	if pod.Status.Phase == corev1.PodRunning && isPodReadyConditionTrue(pod.Status) {
		c.lock.Lock()
		defer c.lock.Unlock()
		delete(c.watchedPods, pod.Name)
	}

	return nil
}

func (c *KubectlDeliveryController) enqueue(obj interface{}) {
	var key string
	var err error
	if key, err = cache.MetaNamespaceKeyFunc(obj); err != nil {
		runtime.HandleError(err)
		return
	}
	c.queue.AddRateLimited(key)
}

// IsPodReadyConditionTrue returns true if a pod is ready; false otherwise.
func isPodReadyConditionTrue(status corev1.PodStatus) bool {
	_, condition := getPodConditionFromList(status.Conditions, corev1.PodReady)
	return condition != nil && condition.Status == corev1.ConditionTrue
}

// GetPodConditionFromList extracts the provided condition from the given list of condition and
// returns the index of the condition and the condition. Returns -1 and nil if the condition is not present.
func getPodConditionFromList(conditions []corev1.PodCondition, conditionType corev1.PodConditionType) (int, *corev1.PodCondition) {
	if conditions == nil {
		return -1, nil
	}
	for i := range conditions {
		if conditions[i].Type == conditionType {
			return i, &conditions[i]
		}
	}
	return -1, nil
}
