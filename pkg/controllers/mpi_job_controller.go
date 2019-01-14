// Copyright 2018 The Kubeflow Authors.
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

package controllers

import (
	"bytes"
	"fmt"
	"time"

	"github.com/golang/glog"
	appsv1 "k8s.io/api/apps/v1"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/apimachinery/pkg/util/wait"
	appsinformers "k8s.io/client-go/informers/apps/v1"
	batchinformers "k8s.io/client-go/informers/batch/v1"
	coreinformers "k8s.io/client-go/informers/core/v1"
	rbacinformers "k8s.io/client-go/informers/rbac/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/kubernetes/scheme"
	typedcorev1 "k8s.io/client-go/kubernetes/typed/core/v1"
	appslisters "k8s.io/client-go/listers/apps/v1"
	batchlisters "k8s.io/client-go/listers/batch/v1"
	corelisters "k8s.io/client-go/listers/core/v1"
	rbaclisters "k8s.io/client-go/listers/rbac/v1"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/tools/record"
	"k8s.io/client-go/util/workqueue"

	kubeflow "github.com/kubeflow/mpi-operator/pkg/apis/kubeflow/v1alpha1"
	clientset "github.com/kubeflow/mpi-operator/pkg/client/clientset/versioned"
	kubeflowScheme "github.com/kubeflow/mpi-operator/pkg/client/clientset/versioned/scheme"
	informers "github.com/kubeflow/mpi-operator/pkg/client/informers/externalversions/kubeflow/v1alpha1"
	listers "github.com/kubeflow/mpi-operator/pkg/client/listers/kubeflow/v1alpha1"
)

const (
	controllerAgentName = "mpi-job-controller"
	configSuffix        = "-config"
	configVolumeName    = "mpi-job-config"
	configMountPath     = "/etc/mpi"
	kubexecScriptName   = "kubexec.sh"
	hostfileName        = "hostfile"
	kubectlDeliveryName = "kubectl-delivery"
	kubectlTargetDirEnv = "TARGET_DIR"
	kubectlVolumeName   = "mpi-job-kubectl"
	kubectlMountPath    = "/opt/kube"
	launcher            = "launcher"
	worker              = "worker"
	launcherSuffix      = "-launcher"
	workerSuffix        = "-worker"
	gpuResourceName     = "nvidia.com/gpu"
	labelGroupName      = "group_name"
	labelMPIJobName     = "mpi_job_name"
	labelMPIRoleType    = "mpi_role_type"
)

const (
	// SuccessSynced is used as part of the Event 'reason' when an MPIJob is
	// synced.
	SuccessSynced = "Synced"
	// ErrResourceExists is used as part of the Event 'reason' when an MPIJob
	// fails to sync due to dependent resources of the same name already
	// existing.
	ErrResourceExists = "ErrResourceExists"

	// MessageResourceExists is the message used for Events when a resource
	// fails to sync due to dependent resources already existing.
	MessageResourceExists = "Resource %q of Kind %q already exists and is not managed by MPIJob"
	// MessageResourceSynced is the message used for an Event fired when an
	// MPIJob is synced successfully.
	MessageResourceSynced = "MPIJob synced successfully"

	// LabelNodeRoleMaster specifies that a node is a master
	LabelNodeRoleMaster = "node-role.kubernetes.io/master"
)

// MPIJobController is the controller implementation for MPIJob resources.
type MPIJobController struct {
	// kubeClient is a standard kubernetes clientset.
	kubeClient kubernetes.Interface
	// kubeflowClient is a clientset for our own API group.
	kubeflowClient clientset.Interface

	configMapLister      corelisters.ConfigMapLister
	configMapSynced      cache.InformerSynced
	serviceAccountLister corelisters.ServiceAccountLister
	serviceAccountSynced cache.InformerSynced
	roleLister           rbaclisters.RoleLister
	roleSynced           cache.InformerSynced
	roleBindingLister    rbaclisters.RoleBindingLister
	roleBindingSynced    cache.InformerSynced
	statefulSetLister    appslisters.StatefulSetLister
	statefulSetSynced    cache.InformerSynced
	jobLister            batchlisters.JobLister
	jobSynced            cache.InformerSynced
	mpiJobLister         listers.MPIJobLister
	mpiJobSynced         cache.InformerSynced

	// queue is a rate limited work queue. This is used to queue work to be
	// processed instead of performing it as soon as a change happens. This
	// means we can ensure we only process a fixed amount of resources at a
	// time, and makes it easy to ensure we are never processing the same item
	// simultaneously in two different workers.
	queue workqueue.RateLimitingInterface
	// recorder is an event recorder for recording Event resources to the
	// Kubernetes API.
	recorder record.EventRecorder
	// The maximum number of GPUs per node.
	gpusPerNode int
	// The container image used to deliver the kubectl binary.
	kubectlDeliveryImage string
}

// NewMPIJobController returns a new MPIJob controller.
func NewMPIJobController(
	kubeClient kubernetes.Interface,
	kubeflowClient clientset.Interface,
	configMapInformer coreinformers.ConfigMapInformer,
	serviceAccountInformer coreinformers.ServiceAccountInformer,
	roleInformer rbacinformers.RoleInformer,
	roleBindingInformer rbacinformers.RoleBindingInformer,
	statefulSetInformer appsinformers.StatefulSetInformer,
	jobInformer batchinformers.JobInformer,
	mpiJobInformer informers.MPIJobInformer,
	gpusPerNode int,
	kubectlDeliveryImage string) *MPIJobController {

	// Create event broadcaster.
	// Add mpi-job-controller types to the default Kubernetes Scheme so Events
	// can be logged for mpi-job-controller types.
	kubeflowScheme.AddToScheme(scheme.Scheme)
	glog.V(4).Info("Creating event broadcaster")
	eventBroadcaster := record.NewBroadcaster()
	eventBroadcaster.StartLogging(glog.Infof)
	eventBroadcaster.StartRecordingToSink(&typedcorev1.EventSinkImpl{Interface: kubeClient.CoreV1().Events("")})
	recorder := eventBroadcaster.NewRecorder(scheme.Scheme, corev1.EventSource{Component: controllerAgentName})

	controller := &MPIJobController{
		kubeClient:           kubeClient,
		kubeflowClient:       kubeflowClient,
		configMapLister:      configMapInformer.Lister(),
		configMapSynced:      configMapInformer.Informer().HasSynced,
		serviceAccountLister: serviceAccountInformer.Lister(),
		serviceAccountSynced: serviceAccountInformer.Informer().HasSynced,
		roleLister:           roleInformer.Lister(),
		roleSynced:           roleInformer.Informer().HasSynced,
		roleBindingLister:    roleBindingInformer.Lister(),
		roleBindingSynced:    roleBindingInformer.Informer().HasSynced,
		statefulSetLister:    statefulSetInformer.Lister(),
		statefulSetSynced:    statefulSetInformer.Informer().HasSynced,
		jobLister:            jobInformer.Lister(),
		jobSynced:            jobInformer.Informer().HasSynced,
		mpiJobLister:         mpiJobInformer.Lister(),
		mpiJobSynced:         mpiJobInformer.Informer().HasSynced,
		queue:                workqueue.NewNamedRateLimitingQueue(workqueue.DefaultControllerRateLimiter(), "MPIJobs"),
		recorder:             recorder,
		gpusPerNode:          gpusPerNode,
		kubectlDeliveryImage: kubectlDeliveryImage,
	}

	glog.Info("Setting up event handlers")
	// Set up an event handler for when MPIJob resources change.
	mpiJobInformer.Informer().AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc: controller.enqueueMPIJob,
		UpdateFunc: func(old, new interface{}) {
			controller.enqueueMPIJob(new)
		},
	})

	// Set up an event handler for when dependent resources change. This
	// handler will lookup the owner of the given resource, and if it is
	// owned by an MPIJob resource will enqueue that MPIJob resource for
	// processing. This way, we don't need to implement custom logic for
	// handling dependent resources. More info on this pattern:
	// https://github.com/kubernetes/community/blob/8cafef897a22026d42f5e5bb3f104febe7e29830/contributors/devel/controllers.md
	configMapInformer.Informer().AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc: controller.handleObject,
		UpdateFunc: func(old, new interface{}) {
			newConfigMap := new.(*corev1.ConfigMap)
			oldConfigMap := old.(*corev1.ConfigMap)
			if newConfigMap.ResourceVersion == oldConfigMap.ResourceVersion {
				// Periodic re-sync will send update events for all known
				// ConfigMaps. Two different versions of the same ConfigMap
				// will always have different RVs.
				return
			}
			controller.handleObject(new)
		},
		DeleteFunc: controller.handleObject,
	})
	serviceAccountInformer.Informer().AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc: controller.handleObject,
		UpdateFunc: func(old, new interface{}) {
			newServiceAccount := new.(*corev1.ServiceAccount)
			oldServiceAccount := old.(*corev1.ServiceAccount)
			if newServiceAccount.ResourceVersion == oldServiceAccount.ResourceVersion {
				// Periodic re-sync will send update events for all known
				// ServiceAccounts. Two different versions of the same ServiceAccount
				// will always have different RVs.
				return
			}
			controller.handleObject(new)
		},
		DeleteFunc: controller.handleObject,
	})
	roleInformer.Informer().AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc: controller.handleObject,
		UpdateFunc: func(old, new interface{}) {
			newRole := new.(*rbacv1.Role)
			oldRole := old.(*rbacv1.Role)
			if newRole.ResourceVersion == oldRole.ResourceVersion {
				// Periodic re-sync will send update events for all known
				// Roles. Two different versions of the same Role
				// will always have different RVs.
				return
			}
			controller.handleObject(new)
		},
		DeleteFunc: controller.handleObject,
	})
	roleBindingInformer.Informer().AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc: controller.handleObject,
		UpdateFunc: func(old, new interface{}) {
			newRoleBinding := new.(*rbacv1.RoleBinding)
			oldRoleBinding := old.(*rbacv1.RoleBinding)
			if newRoleBinding.ResourceVersion == oldRoleBinding.ResourceVersion {
				// Periodic re-sync will send update events for all known
				// RoleBindings. Two different versions of the same RoleBinding
				// will always have different RVs.
				return
			}
			controller.handleObject(new)
		},
		DeleteFunc: controller.handleObject,
	})
	statefulSetInformer.Informer().AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc: controller.handleObject,
		UpdateFunc: func(old, new interface{}) {
			newStatefulSet := new.(*appsv1.StatefulSet)
			oldStatefulSet := old.(*appsv1.StatefulSet)
			if newStatefulSet.ResourceVersion == oldStatefulSet.ResourceVersion {
				// Periodic re-sync will send update events for all known
				// StatefulSets. Two different versions of the same StatefulSet
				// will always have different RVs.
				return
			}
			controller.handleObject(new)
		},
		DeleteFunc: controller.handleObject,
	})
	jobInformer.Informer().AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc: controller.handleObject,
		UpdateFunc: func(old, new interface{}) {
			newJob := new.(*batchv1.Job)
			oldJob := old.(*batchv1.Job)
			if newJob.ResourceVersion == oldJob.ResourceVersion {
				// Periodic re-sync will send update events for all known Jobs.
				// Two different versions of the same Job will always have
				// different RVs.
				return
			}
			controller.handleObject(new)
		},
		DeleteFunc: controller.handleObject,
	})

	return controller
}

// Run will set up the event handlers for types we are interested in, as well
// as syncing informer caches and starting workers. It will block until stopCh
// is closed, at which point it will shutdown the work queue and wait for
// workers to finish processing their current work items.
func (c *MPIJobController) Run(threadiness int, stopCh <-chan struct{}) error {
	defer runtime.HandleCrash()
	defer c.queue.ShutDown()

	// Start the informer factories to begin populating the informer caches.
	glog.Info("Starting MPIJob controller")

	// Wait for the caches to be synced before starting workers.
	glog.Info("Waiting for informer caches to sync")
	if ok := cache.WaitForCacheSync(stopCh, c.configMapSynced, c.serviceAccountSynced, c.roleSynced, c.roleBindingSynced, c.statefulSetSynced, c.jobSynced, c.mpiJobSynced); !ok {
		return fmt.Errorf("failed to wait for caches to sync")
	}

	glog.Info("Starting workers")
	// Launch workers to process MPIJob resources.
	for i := 0; i < threadiness; i++ {
		go wait.Until(c.runWorker, time.Second, stopCh)
	}

	glog.Info("Started workers")
	<-stopCh
	glog.Info("Shutting down workers")

	return nil
}

// runWorker is a long-running function that will continually call the
// processNextWorkItem function in order to read and process a message on the
// work queue.
func (c *MPIJobController) runWorker() {
	for c.processNextWorkItem() {
	}
}

// processNextWorkItem will read a single work item off the work queue and
// attempt to process it, by calling the syncHandler.
func (c *MPIJobController) processNextWorkItem() bool {
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
			return fmt.Errorf("error syncing '%s': %s", key, err.Error())
		}
		// Finally, if no error occurs we Forget this item so it does not
		// get queued again until another change happens.
		c.queue.Forget(obj)
		glog.Infof("Successfully synced '%s'", key)
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
func (c *MPIJobController) syncHandler(key string) error {
	// Convert the namespace/name string into a distinct namespace and name.
	namespace, name, err := cache.SplitMetaNamespaceKey(key)
	if err != nil {
		runtime.HandleError(fmt.Errorf("invalid resource key: %s", key))
		return nil
	}

	// Get the MPIJob with this namespace/name.
	mpiJob, err := c.mpiJobLister.MPIJobs(namespace).Get(name)
	// The MPIJob may no longer exist, in which case we stop processing.
	if errors.IsNotFound(err) {
		runtime.HandleError(fmt.Errorf("mpi job '%s' in work queue no longer exists", key))
		return nil
	}
	if err != nil {
		return err
	}

	// Get the launcher Job for this MPIJob.
	launcher, err := c.getLauncherJob(mpiJob)
	if err != nil {
		return err
	}
	// We're done if the launcher either succeeded or failed.
	done := launcher != nil && (launcher.Status.Succeeded == 1 || launcher.Status.Failed == 1)

	workerReplicas, gpusPerWorker, err := allocateGPUs(mpiJob, c.gpusPerNode, done)
	if err != nil {
		runtime.HandleError(err)
		return nil
	}

	if !done {
		// Get the ConfigMap for this MPIJob.
		if config, err := c.getOrCreateConfigMap(mpiJob, workerReplicas, gpusPerWorker); config == nil || err != nil {
			return err
		}

		// Get the launcher ServiceAccount for this MPIJob.
		if sa, err := c.getOrCreateLauncherServiceAccount(mpiJob); sa == nil || err != nil {
			return err
		}

		// Get the launcher Role for this MPIJob.
		if r, err := c.getOrCreateLauncherRole(mpiJob, workerReplicas); r == nil || err != nil {
			return err
		}

		// Get the launcher RoleBinding for this MPIJob.
		if rb, err := c.getLauncherRoleBinding(mpiJob); rb == nil || err != nil {
			return err
		}
	}

	worker, err := c.getOrCreateWorkerStatefulSet(mpiJob, workerReplicas, gpusPerWorker)
	if err != nil {
		return err
	}

	// If the worker is ready, start the launcher.
	workerReady := workerReplicas == 0 || int(worker.Status.ReadyReplicas) == workerReplicas
	if workerReady && launcher == nil {
		launcher, err = c.kubeClient.BatchV1().Jobs(namespace).Create(newLauncher(mpiJob, c.kubectlDeliveryImage))
		if err != nil {
			return err
		}
	}

	// Finally, we update the status block of the MPIJob resource to reflect the
	// current state of the world.
	err = c.updateMPIJobStatus(mpiJob, launcher, worker)
	if err != nil {
		return err
	}

	c.recorder.Event(mpiJob, corev1.EventTypeNormal, SuccessSynced, MessageResourceSynced)
	return nil
}

// getLauncherJob gets the launcher Job controlled by this MPIJob.
func (c *MPIJobController) getLauncherJob(mpiJob *kubeflow.MPIJob) (*batchv1.Job, error) {
	launcher, err := c.jobLister.Jobs(mpiJob.Namespace).Get(mpiJob.Name + launcherSuffix)
	if errors.IsNotFound(err) {
		return nil, nil
	}
	if err != nil {
		// If an error occurs during Get, we'll requeue the item so we can
		// attempt processing again later. This could have been caused by a
		// temporary network failure, or any other transient reason.
		return nil, err
	}

	// If the launcher is not controlled by this MPIJob resource, we should log
	// a warning to the event recorder and return.
	if !metav1.IsControlledBy(launcher, mpiJob) {
		msg := fmt.Sprintf(MessageResourceExists, launcher.Name, launcher.Kind)
		c.recorder.Event(mpiJob, corev1.EventTypeWarning, ErrResourceExists, msg)
		return launcher, fmt.Errorf(msg)
	}

	return launcher, nil
}

// allocateGPUs allocates the worker replicas and GPUs per worker.
func allocateGPUs(mpiJob *kubeflow.MPIJob, gpusPerNode int, done bool) (workerReplicas int, gpusPerWorker int, err error) {
	workerReplicas = 0
	gpusPerWorker = 0
	err = nil
	if mpiJob.Spec.GPUs != nil {
		totalGPUs := int(*mpiJob.Spec.GPUs)
		if totalGPUs < gpusPerNode {
			workerReplicas = 1
			gpusPerWorker = totalGPUs
		} else if totalGPUs%gpusPerNode == 0 {
			workerReplicas = totalGPUs / gpusPerNode
			gpusPerWorker = gpusPerNode
		} else {
			err = fmt.Errorf("specified #GPUs is not a multiple of GPUs per node (%d)", gpusPerNode)
		}
	} else if mpiJob.Spec.Replicas != nil {
		workerReplicas = int(*mpiJob.Spec.Replicas)
		container := mpiJob.Spec.Template.Spec.Containers[0]
		if container.Resources.Limits != nil {
			if val, ok := container.Resources.Limits[gpuResourceName]; ok {
				gpus, _ := val.AsInt64()
				gpusPerWorker = int(gpus)
			}
		}
	}
	if done {
		workerReplicas = 0
	}
	return workerReplicas, gpusPerWorker, err
}

// getOrCreateConfigMap gets the ConfigMap controlled by this MPIJob, or creates
// one if it doesn't exist.
func (c *MPIJobController) getOrCreateConfigMap(mpiJob *kubeflow.MPIJob, workerReplicas int, gpusPerWorker int) (*corev1.ConfigMap, error) {
	cm, err := c.configMapLister.ConfigMaps(mpiJob.Namespace).Get(mpiJob.Name + configSuffix)
	// If the ConfigMap doesn't exist, we'll create it.
	if errors.IsNotFound(err) {
		cm, err = c.kubeClient.CoreV1().ConfigMaps(mpiJob.Namespace).Create(newConfigMap(mpiJob, workerReplicas, gpusPerWorker))
	}
	// If an error occurs during Get/Create, we'll requeue the item so we
	// can attempt processing again later. This could have been caused by a
	// temporary network failure, or any other transient reason.
	if err != nil {
		return nil, err
	}
	// If the ConfigMap is not controlled by this MPIJob resource, we
	// should log a warning to the event recorder and return.
	if !metav1.IsControlledBy(cm, mpiJob) {
		msg := fmt.Sprintf(MessageResourceExists, cm.Name, cm.Kind)
		c.recorder.Event(mpiJob, corev1.EventTypeWarning, ErrResourceExists, msg)
		return nil, fmt.Errorf(msg)
	}

	return cm, nil
}

// getOrCreateLauncherServiceAccount gets the launcher ServiceAccount controlled
// by this MPIJob, or creates one if it doesn't exist.
func (c *MPIJobController) getOrCreateLauncherServiceAccount(mpiJob *kubeflow.MPIJob) (*corev1.ServiceAccount, error) {
	sa, err := c.serviceAccountLister.ServiceAccounts(mpiJob.Namespace).Get(mpiJob.Name + launcherSuffix)
	// If the ServiceAccount doesn't exist, we'll create it.
	if errors.IsNotFound(err) {
		sa, err = c.kubeClient.CoreV1().ServiceAccounts(mpiJob.Namespace).Create(newLauncherServiceAccount(mpiJob))
	}
	// If an error occurs during Get/Create, we'll requeue the item so we
	// can attempt processing again later. This could have been caused by a
	// temporary network failure, or any other transient reason.
	if err != nil {
		return nil, err
	}
	// If the launcher ServiceAccount is not controlled by this MPIJob resource, we
	// should log a warning to the event recorder and return.
	if !metav1.IsControlledBy(sa, mpiJob) {
		msg := fmt.Sprintf(MessageResourceExists, sa.Name, sa.Kind)
		c.recorder.Event(mpiJob, corev1.EventTypeWarning, ErrResourceExists, msg)
		return nil, fmt.Errorf(msg)
	}

	return sa, nil
}

// getOrCreateLauncherRole gets the launcher Role controlled by this MPIJob.
func (c *MPIJobController) getOrCreateLauncherRole(mpiJob *kubeflow.MPIJob, workerReplicas int) (*rbacv1.Role, error) {
	role, err := c.roleLister.Roles(mpiJob.Namespace).Get(mpiJob.Name + launcherSuffix)
	// If the Role doesn't exist, we'll create it.
	if errors.IsNotFound(err) {
		role, err = c.kubeClient.RbacV1().Roles(mpiJob.Namespace).Create(newLauncherRole(mpiJob, workerReplicas))
	}
	// If an error occurs during Get/Create, we'll requeue the item so we
	// can attempt processing again later. This could have been caused by a
	// temporary network failure, or any other transient reason.
	if err != nil {
		return nil, err
	}
	// If the launcher Role is not controlled by this MPIJob resource, we
	// should log a warning to the event recorder and return.
	if !metav1.IsControlledBy(role, mpiJob) {
		msg := fmt.Sprintf(MessageResourceExists, role.Name, role.Kind)
		c.recorder.Event(mpiJob, corev1.EventTypeWarning, ErrResourceExists, msg)
		return nil, fmt.Errorf(msg)
	}

	return role, nil
}

// getLauncherRoleBinding gets the launcher RoleBinding controlled by this
// MPIJob, or creates one if it doesn't exist.
func (c *MPIJobController) getLauncherRoleBinding(mpiJob *kubeflow.MPIJob) (*rbacv1.RoleBinding, error) {
	rb, err := c.roleBindingLister.RoleBindings(mpiJob.Namespace).Get(mpiJob.Name + launcherSuffix)
	// If the RoleBinding doesn't exist, we'll create it.
	if errors.IsNotFound(err) {
		rb, err = c.kubeClient.RbacV1().RoleBindings(mpiJob.Namespace).Create(newLauncherRoleBinding(mpiJob))
	}
	// If an error occurs during Get/Create, we'll requeue the item so we
	// can attempt processing again later. This could have been caused by a
	// temporary network failure, or any other transient reason.
	if err != nil {
		return nil, err
	}
	// If the launcher RoleBinding is not controlled by this MPIJob resource, we
	// should log a warning to the event recorder and return.
	if !metav1.IsControlledBy(rb, mpiJob) {
		msg := fmt.Sprintf(MessageResourceExists, rb.Name, rb.Kind)
		c.recorder.Event(mpiJob, corev1.EventTypeWarning, ErrResourceExists, msg)
		return nil, fmt.Errorf(msg)
	}

	return rb, nil
}

// getOrCreateWorkerStatefulSet gets the worker StatefulSet controlled by this
// MPIJob, or creates one if it doesn't exist.
func (c *MPIJobController) getOrCreateWorkerStatefulSet(mpiJob *kubeflow.MPIJob, workerReplicas int, gpusPerWorker int) (*appsv1.StatefulSet, error) {
	worker, err := c.statefulSetLister.StatefulSets(mpiJob.Namespace).Get(mpiJob.Name + workerSuffix)
	// If the StatefulSet doesn't exist, we'll create it.
	if errors.IsNotFound(err) && workerReplicas > 0 {
		worker, err = c.kubeClient.AppsV1().StatefulSets(mpiJob.Namespace).Create(newWorker(mpiJob, int32(workerReplicas), gpusPerWorker))
	}
	// If an error occurs during Get/Create, we'll requeue the item so we
	// can attempt processing again later. This could have been caused by a
	// temporary network failure, or any other transient reason.
	if err != nil && !errors.IsNotFound(err) {
		return nil, err
	}

	// If the worker is not controlled by this MPIJob resource, we should log
	// a warning to the event recorder and return.
	if worker != nil && !metav1.IsControlledBy(worker, mpiJob) {
		msg := fmt.Sprintf(MessageResourceExists, worker.Name, worker.Kind)
		c.recorder.Event(mpiJob, corev1.EventTypeWarning, ErrResourceExists, msg)
		return nil, fmt.Errorf(msg)
	}

	// If the worker is out of date, update the worker.
	if worker != nil && int(*worker.Spec.Replicas) != workerReplicas {
		worker, err = c.kubeClient.AppsV1().StatefulSets(mpiJob.Namespace).Update(newWorker(mpiJob, int32(workerReplicas), gpusPerWorker))
		// If an error occurs during Update, we'll requeue the item so we can
		// attempt processing again later. This could have been caused by a
		// temporary network failure, or any other transient reason.
		if err != nil {
			return nil, err
		}
	}

	return worker, nil
}

func (c *MPIJobController) updateMPIJobStatus(mpiJob *kubeflow.MPIJob, launcher *batchv1.Job, worker *appsv1.StatefulSet) error {
	// NEVER modify objects from the store. It's a read-only, local cache.
	// You can use DeepCopy() to make a deep copy of original object and modify this copy
	// Or create a copy manually for better performance
	mpiJobCopy := mpiJob.DeepCopy()
	if launcher != nil {
		if launcher.Status.Active > 0 {
			mpiJobCopy.Status.LauncherStatus = kubeflow.LauncherActive
		} else if launcher.Status.Succeeded > 0 {
			mpiJobCopy.Status.LauncherStatus = kubeflow.LauncherSucceeded
		} else if launcher.Status.Failed > 0 {
			mpiJobCopy.Status.LauncherStatus = kubeflow.LauncherFailed
		}
	}
	if worker != nil {
		mpiJobCopy.Status.WorkerReplicas = worker.Status.ReadyReplicas
	}
	// Until #38113 is merged, we must use Update instead of UpdateStatus to
	// update the Status block of the MPIJob resource. UpdateStatus will not
	// allow changes to the Spec of the resource, which is ideal for ensuring
	// nothing other than resource status has been updated.
	_, err := c.kubeflowClient.KubeflowV1alpha1().MPIJobs(mpiJob.Namespace).Update(mpiJobCopy)
	return err
}

// enqueueMPIJob takes a MPIJob resource and converts it into a namespace/name
// string which is then put onto the work queue. This method should *not* be
// passed resources of any type other than MPIJob.
func (c *MPIJobController) enqueueMPIJob(obj interface{}) {
	var key string
	var err error
	if key, err = cache.MetaNamespaceKeyFunc(obj); err != nil {
		runtime.HandleError(err)
		return
	}
	c.queue.AddRateLimited(key)
}

// handleObject will take any resource implementing metav1.Object and attempt
// to find the MPIJob resource that 'owns' it. It does this by looking at the
// objects metadata.ownerReferences field for an appropriate OwnerReference.
// It then enqueues that MPIJob resource to be processed. If the object does not
// have an appropriate OwnerReference, it will simply be skipped.
func (c *MPIJobController) handleObject(obj interface{}) {
	var object metav1.Object
	var ok bool
	if object, ok = obj.(metav1.Object); !ok {
		tombstone, ok := obj.(cache.DeletedFinalStateUnknown)
		if !ok {
			runtime.HandleError(fmt.Errorf("error decoding object, invalid type"))
			return
		}
		object, ok = tombstone.Obj.(metav1.Object)
		if !ok {
			runtime.HandleError(fmt.Errorf("error decoding object tombstone, invalid type"))
			return
		}
		glog.V(4).Infof("Recovered deleted object '%s' from tombstone", object.GetName())
	}
	glog.V(4).Infof("Processing object: %s", object.GetName())
	if ownerRef := metav1.GetControllerOf(object); ownerRef != nil {
		// If this object is not owned by a MPIJob, we should not do anything
		// more with it.
		if ownerRef.Kind != "MPIJob" {
			return
		}

		mpiJob, err := c.mpiJobLister.MPIJobs(object.GetNamespace()).Get(ownerRef.Name)
		if err != nil {
			glog.V(4).Infof("ignoring orphaned object '%s' of mpi job '%s'", object.GetSelfLink(), ownerRef.Name)
			return
		}

		c.enqueueMPIJob(mpiJob)
		return
	}
}

// newConfigMap creates a new ConfigMap containing configurations for an MPIJob
// resource. It also sets the appropriate OwnerReferences on the resource so
// handleObject can discover the MPIJob resource that 'owns' it.
func newConfigMap(mpiJob *kubeflow.MPIJob, workerReplicas int, gpusPerWorker int) *corev1.ConfigMap {
	kubexec := fmt.Sprintf(`#!/bin/sh
set -x
POD_NAME=$1
shift
%s/kubectl exec ${POD_NAME} -- /bin/sh -c "$*"
`, kubectlMountPath)

	// If no GPU is specified, default to 1 slot.
	slots := 1
	if gpusPerWorker > 0 {
		slots = gpusPerWorker
	}
	var buffer bytes.Buffer
	for i := 0; i < workerReplicas; i++ {
		buffer.WriteString(fmt.Sprintf("%s%s-%d slots=%d\n", mpiJob.Name, workerSuffix, i, slots))
	}

	return &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      mpiJob.Name + configSuffix,
			Namespace: mpiJob.Namespace,
			OwnerReferences: []metav1.OwnerReference{
				*metav1.NewControllerRef(mpiJob, kubeflow.SchemeGroupVersionKind),
			},
		},
		Data: map[string]string{
			hostfileName:      buffer.String(),
			kubexecScriptName: kubexec,
		},
	}
}

// newLauncherServiceAccount creates a new launcher ServiceAccount for an MPIJob
// resource. It also sets the appropriate OwnerReferences on the resource so
// handleObject can discover the MPIJob resource that 'owns' it.
func newLauncherServiceAccount(mpiJob *kubeflow.MPIJob) *corev1.ServiceAccount {
	return &corev1.ServiceAccount{
		ObjectMeta: metav1.ObjectMeta{
			Name:      mpiJob.Name + launcherSuffix,
			Namespace: mpiJob.Namespace,
			Labels: map[string]string{
				"app": mpiJob.Name,
			},
			OwnerReferences: []metav1.OwnerReference{
				*metav1.NewControllerRef(mpiJob, kubeflow.SchemeGroupVersionKind),
			},
		},
	}
}

// newLauncherRole creates a new launcher Role for an MPIJob resource. It also
// sets the appropriate OwnerReferences on the resource so handleObject can
// discover the MPIJob resource that 'owns' it.
func newLauncherRole(mpiJob *kubeflow.MPIJob, workerReplicas int) *rbacv1.Role {
	var podNames []string
	for i := 0; i < workerReplicas; i++ {
		podNames = append(podNames, fmt.Sprintf("%s%s-%d", mpiJob.Name, workerSuffix, i))
	}
	return &rbacv1.Role{
		ObjectMeta: metav1.ObjectMeta{
			Name:      mpiJob.Name + launcherSuffix,
			Namespace: mpiJob.Namespace,
			Labels: map[string]string{
				"app": mpiJob.Name,
			},
			OwnerReferences: []metav1.OwnerReference{
				*metav1.NewControllerRef(mpiJob, kubeflow.SchemeGroupVersionKind),
			},
		},
		Rules: []rbacv1.PolicyRule{
			{
				Verbs:         []string{"get"},
				APIGroups:     []string{""},
				Resources:     []string{"pods"},
				ResourceNames: podNames,
			},
			{
				Verbs:         []string{"create"},
				APIGroups:     []string{""},
				Resources:     []string{"pods/exec"},
				ResourceNames: podNames,
			},
		},
	}
}

// newLauncherRoleBinding creates a new launcher RoleBinding for an MPIJob
// resource. It also sets the appropriate OwnerReferences on the resource so
// handleObject can discover the MPIJob resource that 'owns' it.
func newLauncherRoleBinding(mpiJob *kubeflow.MPIJob) *rbacv1.RoleBinding {
	launcherName := mpiJob.Name + launcherSuffix
	return &rbacv1.RoleBinding{
		ObjectMeta: metav1.ObjectMeta{
			Name:      launcherName,
			Namespace: mpiJob.Namespace,
			Labels: map[string]string{
				"app": mpiJob.Name,
			},
			OwnerReferences: []metav1.OwnerReference{
				*metav1.NewControllerRef(mpiJob, kubeflow.SchemeGroupVersionKind),
			},
		},
		Subjects: []rbacv1.Subject{
			{
				Kind:      rbacv1.ServiceAccountKind,
				Name:      launcherName,
				Namespace: mpiJob.Namespace,
			},
		},
		RoleRef: rbacv1.RoleRef{
			APIGroup: rbacv1.GroupName,
			Kind:     "Role",
			Name:     launcherName,
		},
	}
}

// newWorker creates a new worker StatefulSet for an MPIJob resource. It also
// sets the appropriate OwnerReferences on the resource so handleObject can
// discover the MPIJob resource that 'owns' it.
func newWorker(mpiJob *kubeflow.MPIJob, desiredReplicas int32, gpus int) *appsv1.StatefulSet {
	labels := map[string]string{
		labelGroupName:   "kubeflow.org",
		labelMPIJobName:  mpiJob.Name,
		labelMPIRoleType: worker,
	}

	podSpec := mpiJob.Spec.Template.DeepCopy()
	// keep the labels which are set in PodTemplate
	if len(podSpec.Labels) == 0 {
		podSpec.Labels = make(map[string]string)
	}

	for key, value := range labels {
		podSpec.Labels[key] = value
	}
	// always set restartPolicy to restartAlways for statefulset
	podSpec.Spec.RestartPolicy = corev1.RestartPolicyAlways

	container := podSpec.Spec.Containers[0]
	container.Command = []string{"sleep"}
	container.Args = []string{"365d"}
	if container.Resources.Limits == nil {
		container.Resources.Limits = make(corev1.ResourceList)
	}
	container.Resources.Limits[gpuResourceName] = *resource.NewQuantity(int64(gpus), resource.DecimalExponent)

	// We need the kubexec.sh script here because Open MPI checks for the path
	// in every rank.
	container.VolumeMounts = append(container.VolumeMounts, corev1.VolumeMount{
		Name:      configVolumeName,
		MountPath: configMountPath,
	})
	podSpec.Spec.Containers[0] = container

	scriptMode := int32(0555)
	podSpec.Spec.Volumes = append(podSpec.Spec.Volumes, corev1.Volume{
		Name: configVolumeName,
		VolumeSource: corev1.VolumeSource{
			ConfigMap: &corev1.ConfigMapVolumeSource{
				LocalObjectReference: corev1.LocalObjectReference{
					Name: mpiJob.Name + configSuffix,
				},
				Items: []corev1.KeyToPath{
					{
						Key:  kubexecScriptName,
						Path: kubexecScriptName,
						Mode: &scriptMode,
					},
				},
			},
		},
	})

	// set default BackoffLimit
	if mpiJob.Spec.BackoffLimit == nil {
		mpiJob.Spec.BackoffLimit = new(int32)
		*mpiJob.Spec.BackoffLimit = 6
	}

	return &appsv1.StatefulSet{
		ObjectMeta: metav1.ObjectMeta{
			Name:      mpiJob.Name + workerSuffix,
			Namespace: mpiJob.Namespace,
			Labels:    labels,
			OwnerReferences: []metav1.OwnerReference{
				*metav1.NewControllerRef(mpiJob, kubeflow.SchemeGroupVersionKind),
			},
		},
		Spec: appsv1.StatefulSetSpec{
			PodManagementPolicy: appsv1.ParallelPodManagement,
			Replicas:            &desiredReplicas,
			Selector: &metav1.LabelSelector{
				MatchLabels: labels,
			},
			ServiceName: mpiJob.Name + workerSuffix,
			Template:    *podSpec,
		},
	}
}

// newLauncher creates a new launcher Job for an MPIJob resource. It also sets
// the appropriate OwnerReferences on the resource so handleObject can discover
// the MPIJob resource that 'owns' it.
func newLauncher(mpiJob *kubeflow.MPIJob, kubectlDeliveryImage string) *batchv1.Job {
	launcherName := mpiJob.Name + launcherSuffix
	labels := map[string]string{
		labelGroupName:   "kubeflow.org",
		labelMPIJobName:  mpiJob.Name,
		labelMPIRoleType: launcher,
	}

	podSpec := mpiJob.Spec.Template.DeepCopy()
	// copy the labels and annotations to pod from PodTemplate
	if len(podSpec.Labels) == 0 {
		podSpec.Labels = make(map[string]string)
	}
	for key, value := range labels {
		podSpec.Labels[key] = value
	}

	podSpec.Spec.ServiceAccountName = launcherName
	podSpec.Spec.InitContainers = append(podSpec.Spec.InitContainers, corev1.Container{
		Name:  kubectlDeliveryName,
		Image: kubectlDeliveryImage,
		Env: []corev1.EnvVar{
			{
				Name:  kubectlTargetDirEnv,
				Value: kubectlMountPath,
			},
		},
		VolumeMounts: []corev1.VolumeMount{
			{
				Name:      kubectlVolumeName,
				MountPath: kubectlMountPath,
			},
		},
	})
	container := podSpec.Spec.Containers[0]
	container.Env = append(container.Env,
		corev1.EnvVar{
			Name:  "OMPI_MCA_plm_rsh_agent",
			Value: fmt.Sprintf("%s/%s", configMountPath, kubexecScriptName),
		},
		corev1.EnvVar{
			Name:  "OMPI_MCA_orte_default_hostfile",
			Value: fmt.Sprintf("%s/%s", configMountPath, hostfileName),
		})
	// Not assign any resource limits and requests to launcher pod
	container.Resources.Limits = nil
	container.Resources.Requests = nil

	// determine if run the launcher on the master node
	if mpiJob.Spec.LauncherOnMaster {

		// support Tolerate
		podSpec.Spec.Tolerations = []corev1.Toleration{
			{
				Key:    LabelNodeRoleMaster,
				Effect: corev1.TaintEffectNoSchedule,
			},
		}
		// prefer to assign pod to master node
		podSpec.Spec.Affinity = &corev1.Affinity{
			NodeAffinity: &corev1.NodeAffinity{
				RequiredDuringSchedulingIgnoredDuringExecution: &corev1.NodeSelector{
					NodeSelectorTerms: []corev1.NodeSelectorTerm{
						{
							MatchExpressions: []corev1.NodeSelectorRequirement{
								{
									Key:      LabelNodeRoleMaster,
									Operator: corev1.NodeSelectorOpExists,
								},
							},
						},
					},
				},
			},
		}
	}

	container.VolumeMounts = append(container.VolumeMounts,
		corev1.VolumeMount{
			Name:      kubectlVolumeName,
			MountPath: kubectlMountPath,
		},
		corev1.VolumeMount{
			Name:      configVolumeName,
			MountPath: configMountPath,
		})
	podSpec.Spec.Containers[0] = container
	// Only a `RestartPolicy` equal to `Never` or `OnFailure` is allowed for `Job`.
	if podSpec.Spec.RestartPolicy != corev1.RestartPolicyNever {
		podSpec.Spec.RestartPolicy = corev1.RestartPolicyOnFailure
	}
	scriptsMode := int32(0555)
	hostfileMode := int32(0444)
	podSpec.Spec.Volumes = append(podSpec.Spec.Volumes,
		corev1.Volume{
			Name: kubectlVolumeName,
			VolumeSource: corev1.VolumeSource{
				EmptyDir: &corev1.EmptyDirVolumeSource{},
			},
		},
		corev1.Volume{
			Name: configVolumeName,
			VolumeSource: corev1.VolumeSource{
				ConfigMap: &corev1.ConfigMapVolumeSource{
					LocalObjectReference: corev1.LocalObjectReference{
						Name: mpiJob.Name + configSuffix,
					},
					Items: []corev1.KeyToPath{
						{
							Key:  kubexecScriptName,
							Path: kubexecScriptName,
							Mode: &scriptsMode,
						},
						{
							Key:  hostfileName,
							Path: hostfileName,
							Mode: &hostfileMode,
						},
					},
				},
			},
		})

	return &batchv1.Job{
		ObjectMeta: metav1.ObjectMeta{
			Name:      launcherName,
			Namespace: mpiJob.Namespace,
			Labels:    labels,
			OwnerReferences: []metav1.OwnerReference{
				*metav1.NewControllerRef(mpiJob, kubeflow.SchemeGroupVersionKind),
			},
		},
		Spec: batchv1.JobSpec{
			BackoffLimit:          mpiJob.Spec.BackoffLimit,
			ActiveDeadlineSeconds: mpiJob.Spec.ActiveDeadlineSeconds,
			Template:              *podSpec,
		},
	}
}
