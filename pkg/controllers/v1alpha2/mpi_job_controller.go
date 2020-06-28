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

package v1alpha2

import (
	"bytes"
	"fmt"
	"reflect"
	"time"

	"k8s.io/apimachinery/pkg/api/resource"

	podgroupv1alpha1 "github.com/kubernetes-sigs/kube-batch/pkg/apis/scheduling/v1alpha1"
	kubebatchclient "github.com/kubernetes-sigs/kube-batch/pkg/client/clientset/versioned"
	podgroupsinformer "github.com/kubernetes-sigs/kube-batch/pkg/client/informers/externalversions/scheduling/v1alpha1"
	podgroupslists "github.com/kubernetes-sigs/kube-batch/pkg/client/listers/scheduling/v1alpha1"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
	appsv1 "k8s.io/api/apps/v1"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
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
	"k8s.io/klog"

	common "github.com/kubeflow/common/pkg/apis/common/v1"
	kubeflow "github.com/kubeflow/mpi-operator/pkg/apis/kubeflow/v1alpha2"
	clientset "github.com/kubeflow/mpi-operator/pkg/client/clientset/versioned"
	informers "github.com/kubeflow/mpi-operator/pkg/client/informers/externalversions/kubeflow/v1alpha2"
	listers "github.com/kubeflow/mpi-operator/pkg/client/listers/kubeflow/v1alpha2"
)

const (
	controllerAgentName     = "mpi-job-controller"
	configSuffix            = "-config"
	configVolumeName        = "mpi-job-config"
	configMountPath         = "/etc/mpi"
	kubexecScriptName       = "kubexec.sh"
	hostfileName            = "hostfile"
	kubectlDeliveryName     = "kubectl-delivery"
	kubectlTargetDirEnv     = "TARGET_DIR"
	kubectlVolumeName       = "mpi-job-kubectl"
	kubectlMountPath        = "/opt/kube"
	launcher                = "launcher"
	worker                  = "worker"
	launcherSuffix          = "-launcher"
	workerSuffix            = "-worker"
	gpuResourceName         = "nvidia.com/gpu"
	labelGroupName          = "group_name"
	labelMPIJobName         = "mpi_job_name"
	labelMPIRoleType        = "mpi_role_type"
	initContainerCpu        = "100m"
	initContainerEphStorage = "5Gi"
	initContainerMem        = "512Mi"
)

const (
	// ErrResourceExists is used as part of the Event 'reason' when an MPIJob
	// fails to sync due to dependent resources of the same name already
	// existing.
	ErrResourceExists = "ErrResourceExists"

	// MessageResourceExists is the message used for Events when a resource
	// fails to sync due to dependent resources already existing.
	MessageResourceExists = "Resource %q of Kind %q already exists and is not managed by MPIJob"

	// ErrResourceDoesNotExists is used as part of the Event 'reason' when some
	// resource is missing in yaml
	ErrResourceDoesNotExists = "ErrResourceNotExists"

	// ErrResourceNotExists is used for Events when some
	// resource is missing in yaml
	MessageResourceDoesNotExists = "Resource %q is missing in yaml"

	// podTemplateRestartPolicyReason is the warning reason when the restart
	// policy is set in pod template.
	podTemplateRestartPolicyReason = "SettedPodTemplateRestartPolicy"
)

var (
	mpiJobsSuccessCount = promauto.NewCounter(prometheus.CounterOpts{
		Name: "mpi_operator_jobs_successful_total",
		Help: "Counts number of MPI jobs successful",
	})
	mpiJobsFailureCount = promauto.NewCounter(prometheus.CounterOpts{
		Name: "mpi_operator_jobs_failed_total",
		Help: "Counts number of MPI jobs failed",
	})
	mpiJobsRestartCount = promauto.NewCounter(prometheus.CounterOpts{
		Name: "mpi_operator_jobs_restarted_total",
		Help: "Counts number of MPI jobs restarted",
	})
)

// MPIJobController is the controller implementation for MPIJob resources.
type MPIJobController struct {
	// kubeClient is a standard kubernetes clientset.
	kubeClient kubernetes.Interface
	// kubeflowClient is a clientset for our own API group.
	kubeflowClient clientset.Interface
	// kubebatchClient is a clientset for kube-batch API.
	kubebatchClient kubebatchclient.Interface

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
	podgroupsLister      podgroupslists.PodGroupLister
	podgroupsSynced      cache.InformerSynced
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
	// The container image used to deliver the kubectl binary.
	kubectlDeliveryImage string
	// Gang scheduler name to use
	gangSchedulerName string

	// To allow injection of updateStatus for testing.
	updateStatusHandler func(mpijob *kubeflow.MPIJob) error
}

// NewMPIJobController returns a new MPIJob controller.
func NewMPIJobController(
	kubeClient kubernetes.Interface,
	kubeflowClient clientset.Interface,
	kubeBatchClientSet kubebatchclient.Interface,
	configMapInformer coreinformers.ConfigMapInformer,
	serviceAccountInformer coreinformers.ServiceAccountInformer,
	roleInformer rbacinformers.RoleInformer,
	roleBindingInformer rbacinformers.RoleBindingInformer,
	statefulSetInformer appsinformers.StatefulSetInformer,
	jobInformer batchinformers.JobInformer,
	podgroupsInformer podgroupsinformer.PodGroupInformer,
	mpiJobInformer informers.MPIJobInformer,
	kubectlDeliveryImage string,
	gangSchedulerName string) *MPIJobController {

	// Create event broadcaster.
	klog.V(4).Info("Creating event broadcaster")
	eventBroadcaster := record.NewBroadcaster()
	eventBroadcaster.StartLogging(klog.Infof)
	eventBroadcaster.StartRecordingToSink(&typedcorev1.EventSinkImpl{Interface: kubeClient.CoreV1().Events("")})
	recorder := eventBroadcaster.NewRecorder(scheme.Scheme, corev1.EventSource{Component: controllerAgentName})

	var podgroupsLister podgroupslists.PodGroupLister
	var podgroupsSynced cache.InformerSynced
	if gangSchedulerName != "" {
		podgroupsLister = podgroupsInformer.Lister()
		podgroupsSynced = podgroupsInformer.Informer().HasSynced
	}

	controller := &MPIJobController{
		kubeClient:           kubeClient,
		kubeflowClient:       kubeflowClient,
		kubebatchClient:      kubeBatchClientSet,
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
		podgroupsLister:      podgroupsLister,
		podgroupsSynced:      podgroupsSynced,
		mpiJobLister:         mpiJobInformer.Lister(),
		mpiJobSynced:         mpiJobInformer.Informer().HasSynced,
		queue:                workqueue.NewNamedRateLimitingQueue(workqueue.DefaultControllerRateLimiter(), "MPIJobs"),
		recorder:             recorder,
		kubectlDeliveryImage: kubectlDeliveryImage,
		gangSchedulerName:    gangSchedulerName,
	}

	controller.updateStatusHandler = controller.doUpdateJobStatus

	klog.Info("Setting up event handlers")
	// Set up an event handler for when MPIJob resources change.
	mpiJobInformer.Informer().AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc: controller.addMPIJob,
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
	if podgroupsInformer != nil {
		podgroupsInformer.Informer().AddEventHandler(cache.ResourceEventHandlerFuncs{
			AddFunc: controller.handleObject,
			UpdateFunc: func(old, new interface{}) {
				newPolicy := new.(*podgroupv1alpha1.PodGroup)
				oldPolicy := old.(*podgroupv1alpha1.PodGroup)
				if newPolicy.ResourceVersion == oldPolicy.ResourceVersion {
					// Periodic re-sync will send update events for all known PodDisruptionBudgets.
					// Two different versions of the same Job will always have
					// different RVs.
					return
				}
				controller.handleObject(new)
			},
			DeleteFunc: controller.handleObject,
		})
	}
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
	klog.Info("Starting MPIJob controller")

	// Wait for the caches to be synced before starting workers.
	klog.Info("Waiting for informer caches to sync")
	if ok := cache.WaitForCacheSync(stopCh, c.configMapSynced, c.serviceAccountSynced, c.roleSynced, c.roleBindingSynced, c.statefulSetSynced, c.jobSynced, c.mpiJobSynced); !ok {
		return fmt.Errorf("failed to wait for caches to sync")
	}
	if c.gangSchedulerName != "" {
		if ok := cache.WaitForCacheSync(stopCh, c.podgroupsSynced); !ok {
			return fmt.Errorf("failed to wait for podgroup caches to sync")
		}
	}

	klog.Info("Starting workers")
	// Launch workers to process MPIJob resources.
	for i := 0; i < threadiness; i++ {
		go wait.Until(c.runWorker, time.Second, stopCh)
	}

	klog.Info("Started workers")
	<-stopCh
	klog.Info("Shutting down workers")

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
func (c *MPIJobController) syncHandler(key string) error {
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

	// Get the MPIJob with this namespace/name.
	sharedJob, err := c.mpiJobLister.MPIJobs(namespace).Get(name)
	if err != nil {
		// The MPIJob may no longer exist, in which case we stop processing.
		if errors.IsNotFound(err) {
			klog.V(4).Infof("MPIJob has been deleted: %v", key)
			return nil
		}
		return err
	}

	// NEVER modify objects from the store. It's a read-only, local cache.
	// You can use DeepCopy() to make a deep copy of original object and modify this copy
	// Or create a copy manually for better performance
	mpiJob := sharedJob.DeepCopy()
	// Set default for the new mpiJob.
	scheme.Scheme.Default(mpiJob)

	// for mpi job that is terminating, just return.
	if mpiJob.DeletionTimestamp != nil {
		return nil
	}

	// If the MPIJob is terminated, delete its pods according to cleanPodPolicy.
	if isSucceeded(mpiJob.Status) || isFailed(mpiJob.Status) {
		if isCleanUpPods(mpiJob.Spec.CleanPodPolicy) {
			// set worker StatefulSet Replicas to 0.
			if _, err := c.getOrCreateWorkerStatefulSet(mpiJob, 0); err != nil {
				return err
			}
			initializeMPIJobStatuses(mpiJob, kubeflow.MPIReplicaTypeWorker)
			mpiJob.Status.ReplicaStatuses[common.ReplicaType(kubeflow.MPIReplicaTypeWorker)].Active = 0
		}

		if c.gangSchedulerName != "" {
			if err := c.deletePodGroups(mpiJob); err != nil {
				return err
			}
		}

		return c.updateStatusHandler(mpiJob)
	}

	// first set StartTime.
	if mpiJob.Status.StartTime == nil {
		now := metav1.Now()
		mpiJob.Status.StartTime = &now
	}

	// Get the launcher Job for this MPIJob.
	launcher, err := c.getLauncherJob(mpiJob)
	if err != nil {
		return err
	}

	var worker *appsv1.StatefulSet
	// We're done if the launcher either succeeded or failed.
	done := launcher != nil && isJobFinished(launcher)
	if !done {
		workerSpec := mpiJob.Spec.MPIReplicaSpecs[kubeflow.MPIReplicaTypeWorker]
		workerReplicas := *workerSpec.Replicas

		// Get the ConfigMap for this MPIJob.
		if config, err := c.getOrCreateConfigMap(mpiJob, workerReplicas); config == nil || err != nil {
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

		// Get the PodGroup for this MPIJob
		if c.gangSchedulerName != "" {
			if podgroup, err := c.getOrCreatePodGroups(mpiJob, workerReplicas+1); podgroup == nil || err != nil {
				return err
			}
		}

		worker, err = c.getOrCreateWorkerStatefulSet(mpiJob, workerReplicas)
		if err != nil {
			return err
		}
		if launcher == nil {
			launcher, err = c.kubeClient.BatchV1().Jobs(namespace).Create(c.newLauncher(mpiJob, c.kubectlDeliveryImage))
			if err != nil {
				return err
			}
		}
	}

	// Finally, we update the status block of the MPIJob resource to reflect the
	// current state of the world.
	err = c.updateMPIJobStatus(mpiJob, launcher, worker)
	if err != nil {
		return err
	}

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

// getOrCreatePodGroups will create a PodGroup for gang scheduling by kube-batch.
func (c *MPIJobController) getOrCreatePodGroups(mpiJob *kubeflow.MPIJob, minAvailableWorkerReplicas int32) (*podgroupv1alpha1.PodGroup, error) {
	podgroup, err := c.podgroupsLister.PodGroups(mpiJob.Namespace).Get(mpiJob.Name)
	// If the PodGroup doesn't exist, we'll create it.
	if errors.IsNotFound(err) {
		podgroup, err = c.kubebatchClient.SchedulingV1alpha1().PodGroups(mpiJob.Namespace).Create(newPodGroup(mpiJob, minAvailableWorkerReplicas))
	}
	// If an error occurs during Get/Create, we'll requeue the item so we
	// can attempt processing again later. This could have been caused by a
	// temporary network failure, or any other transient reason.
	if err != nil {
		return nil, err
	}
	// If the PodGroup is not controlled by this MPIJob resource, we
	// should log a warning to the event recorder and return.
	if !metav1.IsControlledBy(podgroup, mpiJob) {
		msg := fmt.Sprintf(MessageResourceExists, podgroup.Name, podgroup.Kind)
		c.recorder.Event(mpiJob, corev1.EventTypeWarning, ErrResourceExists, msg)
		return nil, fmt.Errorf(msg)
	}

	return podgroup, nil
}

// deletePodGroups will delete a PodGroup when MPIJob have done.
func (c *MPIJobController) deletePodGroups(mpiJob *kubeflow.MPIJob) error {
	podgroup, err := c.podgroupsLister.PodGroups(mpiJob.Namespace).Get(mpiJob.Name)
	if err != nil {
		if errors.IsNotFound(err) {
			return nil
		}
		return err
	}

	// If the PodGroup is not controlled by this MPIJob resource, we
	// should log a warning to the event recorder and return.
	if !metav1.IsControlledBy(podgroup, mpiJob) {
		msg := fmt.Sprintf(MessageResourceExists, podgroup.Name, podgroup.Kind)
		c.recorder.Event(mpiJob, corev1.EventTypeWarning, ErrResourceExists, msg)
		return fmt.Errorf(msg)
	}

	// If the PodGroup exist, we'll delete it.
	err = c.kubebatchClient.SchedulingV1alpha1().PodGroups(mpiJob.Namespace).Delete(mpiJob.Name, &metav1.DeleteOptions{})
	// If an error occurs during Delete, we'll requeue the item so we
	// can attempt processing again later. This could have been caused by a
	// temporary network failure, or any other transient reason.
	if err != nil {
		return err
	}

	return nil
}

// getOrCreateConfigMap gets the ConfigMap controlled by this MPIJob, or creates
// one if it doesn't exist.
func (c *MPIJobController) getOrCreateConfigMap(mpiJob *kubeflow.MPIJob, workerReplicas int32) (*corev1.ConfigMap, error) {
	cm, err := c.configMapLister.ConfigMaps(mpiJob.Namespace).Get(mpiJob.Name + configSuffix)
	// If the ConfigMap doesn't exist, we'll create it.
	if errors.IsNotFound(err) {
		cm, err = c.kubeClient.CoreV1().ConfigMaps(mpiJob.Namespace).Create(newConfigMap(mpiJob, workerReplicas))
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
func (c *MPIJobController) getOrCreateLauncherRole(mpiJob *kubeflow.MPIJob, workerReplicas int32) (*rbacv1.Role, error) {
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
func (c *MPIJobController) getOrCreateWorkerStatefulSet(mpiJob *kubeflow.MPIJob, workerReplicas int32) (*appsv1.StatefulSet, error) {
	worker, err := c.statefulSetLister.StatefulSets(mpiJob.Namespace).Get(mpiJob.Name + workerSuffix)
	// If the StatefulSet doesn't exist, we'll create it.
	if errors.IsNotFound(err) && workerReplicas > 0 {
		worker := newWorker(mpiJob, workerReplicas, c.gangSchedulerName)
		if worker == nil {
			msg := fmt.Sprintf(MessageResourceDoesNotExists, "Worker")
			c.recorder.Event(mpiJob, corev1.EventTypeWarning, ErrResourceDoesNotExists, msg)
			err = fmt.Errorf(msg)
			return nil, err
		}
		worker, err = c.kubeClient.AppsV1().StatefulSets(mpiJob.Namespace).Create(worker)
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
	if worker != nil && *worker.Spec.Replicas != workerReplicas {
		newWorkerObj := newWorker(mpiJob, workerReplicas, c.gangSchedulerName)
		if newWorkerObj == nil {
			msg := fmt.Sprintf(MessageResourceDoesNotExists, "Worker")
			c.recorder.Event(mpiJob, corev1.EventTypeWarning, ErrResourceDoesNotExists, msg)
			err = fmt.Errorf(msg)
			return nil, err
		}
		worker, err = c.kubeClient.AppsV1().StatefulSets(mpiJob.Namespace).Update(newWorkerObj)
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
	oldStatus := mpiJob.Status.DeepCopy()
	if launcher != nil {
		initializeMPIJobStatuses(mpiJob, kubeflow.MPIReplicaTypeLauncher)
		mpiJob.Status.ReplicaStatuses[common.ReplicaType(kubeflow.MPIReplicaTypeLauncher)].Succeeded = launcher.Status.Succeeded
		mpiJob.Status.ReplicaStatuses[common.ReplicaType(kubeflow.MPIReplicaTypeLauncher)].Failed = launcher.Status.Failed
		mpiJob.Status.ReplicaStatuses[common.ReplicaType(kubeflow.MPIReplicaTypeLauncher)].Active = launcher.Status.Active
		if isJobComplete(launcher) {
			msg := fmt.Sprintf("MPIJob %s/%s successfully completed.", mpiJob.Namespace, mpiJob.Name)
			c.recorder.Event(mpiJob, corev1.EventTypeNormal, mpiJobSucceededReason, msg)
			if mpiJob.Status.CompletionTime == nil {
				now := metav1.Now()
				mpiJob.Status.CompletionTime = &now
			}
			err := updateMPIJobConditions(mpiJob, common.JobSucceeded, mpiJobSucceededReason, msg)
			if err != nil {
				klog.Infof("Append mpiJob(%s/%s) condition error: %v", mpiJob.Namespace, mpiJob.Name, err)
				return err
			}
			mpiJobsSuccessCount.Inc()
		} else if isJobFailed(launcher) {
			msg := fmt.Sprintf("MPIJob %s/%s has failed", mpiJob.Namespace, mpiJob.Name)
			c.recorder.Event(mpiJob, corev1.EventTypeWarning, mpiJobFailedReason, msg)
			if mpiJob.Status.CompletionTime == nil {
				now := metav1.Now()
				mpiJob.Status.CompletionTime = &now
			}
			err := updateMPIJobConditions(mpiJob, common.JobFailed, mpiJobFailedReason, msg)
			if err != nil {
				klog.Infof("Append mpiJob(%s/%s) condition error: %v", mpiJob.Namespace, mpiJob.Name, err)
				return err
			}
			mpiJobsFailureCount.Inc()
		} else if launcher.Status.Failed > 0 {
			msg := fmt.Sprintf("MPIJob %s/%s is restarting.", mpiJob.Namespace, mpiJob.Name)
			err := updateMPIJobConditions(mpiJob, common.JobRestarting, mpiJobRestartingReason, msg)
			if err != nil {
				klog.Infof("Append mpiJob(%s/%s) condition error: %v", mpiJob.Namespace, mpiJob.Name, err)
				return err
			}
			mpiJobsRestartCount.Inc()
		}
	}

	if worker != nil {
		initializeMPIJobStatuses(mpiJob, kubeflow.MPIReplicaTypeWorker)
		if worker.Status.ReadyReplicas > 0 {
			mpiJob.Status.ReplicaStatuses[common.ReplicaType(kubeflow.MPIReplicaTypeWorker)].Active = worker.Status.ReadyReplicas
		}
	}

	if launcher != nil && worker != nil && launcher.Status.Active > 0 && worker.Status.ReadyReplicas == *worker.Spec.Replicas {
		msg := fmt.Sprintf("MPIJob %s/%s is running.", mpiJob.Namespace, mpiJob.Name)
		err := updateMPIJobConditions(mpiJob, common.JobRunning, mpiJobRunningReason, msg)
		if err != nil {
			klog.Infof("Append mpiJob(%s/%s) condition error: %v", mpiJob.Namespace, mpiJob.Name, err)
			return err
		}
	}

	// no need to update the mpijob if the status hasn't changed since last time.
	if !reflect.DeepEqual(*oldStatus, mpiJob.Status) {
		return c.updateStatusHandler(mpiJob)
	}
	return nil
}

// When a mpiJob is added, set the defaults and enqueue the current mpiJob.
func (c *MPIJobController) addMPIJob(obj interface{}) {
	mpiJob := obj.(*kubeflow.MPIJob)

	// Set default for the new mpiJob.
	scheme.Scheme.Default(mpiJob)
	msg := fmt.Sprintf("MPIJob %s/%s is created.", mpiJob.Namespace, mpiJob.Name)
	// Add a created condition.
	err := updateMPIJobConditions(mpiJob, common.JobCreated, mpiJobCreatedReason, msg)
	if err != nil {
		klog.Errorf("Append mpiJob condition error: %v", err)
		return
	}

	c.enqueueMPIJob(mpiJob)
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
		klog.V(4).Infof("Recovered deleted object '%s' from tombstone", object.GetName())
	}
	klog.V(4).Infof("Processing object: %s", object.GetName())
	if ownerRef := metav1.GetControllerOf(object); ownerRef != nil {
		// Parse the Group out of the OwnerReference to compare it to what was parsed out of the requested OwnerType
		refGV, err := schema.ParseGroupVersion(ownerRef.APIVersion)
		if err != nil {
			runtime.HandleError(fmt.Errorf("Could not parse OwnerReference APIVersion: %v", err))
			return
		}

		// Compare the OwnerReference Group and Kind against the OwnerType Group and Kind.
		// Since we do not support conversion webhook now, we do not deal with v1alpha1 resources in this operator.
		if ownerRef.Kind != kubeflow.Kind || refGV.Group != kubeflow.GroupName || refGV.Version != kubeflow.GroupVersion {
			return
		}

		mpiJob, err := c.mpiJobLister.MPIJobs(object.GetNamespace()).Get(ownerRef.Name)
		if err != nil {
			klog.V(4).Infof("ignoring orphaned object '%s' of mpi job '%s'", object.GetSelfLink(), ownerRef.Name)
			return
		}

		c.enqueueMPIJob(mpiJob)
		return
	}
}

// doUpdateJobStatus updates the status of the given MPIJob by call apiServer.
func (c *MPIJobController) doUpdateJobStatus(mpiJob *kubeflow.MPIJob) error {
	_, err := c.kubeflowClient.KubeflowV1alpha2().MPIJobs(mpiJob.Namespace).UpdateStatus(mpiJob)
	return err
}

// newConfigMap creates a new ConfigMap containing configurations for an MPIJob
// resource. It also sets the appropriate OwnerReferences on the resource so
// handleObject can discover the MPIJob resource that 'owns' it.
func newConfigMap(mpiJob *kubeflow.MPIJob, workerReplicas int32) *corev1.ConfigMap {
	kubexec := fmt.Sprintf(`#!/bin/sh
set -x
POD_NAME=$1
shift
%s/kubectl exec ${POD_NAME}`, kubectlMountPath)
	if len(mpiJob.Spec.MainContainer) > 0 {
		kubexec = fmt.Sprintf("%s --container %s", kubexec, mpiJob.Spec.MainContainer)
	}
	kubexec = fmt.Sprintf("%s -- /bin/sh -c \"$*\"", kubexec)

	// If no processing unit is specified, default to 1 slot.
	slots := 1
	if mpiJob.Spec.SlotsPerWorker != nil {
		slots = int(*mpiJob.Spec.SlotsPerWorker)
	}
	var buffer bytes.Buffer
	for i := 0; i < int(workerReplicas); i++ {
		buffer.WriteString(fmt.Sprintf("%s%s-%d slots=%d\n", mpiJob.Name, workerSuffix, i, slots))
	}

	return &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      mpiJob.Name + configSuffix,
			Namespace: mpiJob.Namespace,
			Labels: map[string]string{
				"app": mpiJob.Name,
			},
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
func newLauncherRole(mpiJob *kubeflow.MPIJob, workerReplicas int32) *rbacv1.Role {
	var podNames []string
	for i := 0; i < int(workerReplicas); i++ {
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
				Verbs:     []string{"get", "list", "watch"},
				APIGroups: []string{""},
				Resources: []string{"pods"},
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

// newPodGroup creates a new PodGroup for an MPIJob
// resource. It also sets the appropriate OwnerReferences on the resource so
// handleObject can discover the MPIJob resource that 'owns' it.
func newPodGroup(mpiJob *kubeflow.MPIJob, minAvailableReplicas int32) *podgroupv1alpha1.PodGroup {
	var pName string
	if l := mpiJob.Spec.MPIReplicaSpecs[kubeflow.MPIReplicaTypeLauncher]; l != nil {
		pName = l.Template.Spec.PriorityClassName
		if w := mpiJob.Spec.MPIReplicaSpecs[kubeflow.MPIReplicaTypeWorker]; pName == "" && w != nil {
			pName = w.Template.Spec.PriorityClassName
		}
	}
	return &podgroupv1alpha1.PodGroup{
		ObjectMeta: metav1.ObjectMeta{
			Name:      mpiJob.Name,
			Namespace: mpiJob.Namespace,
			OwnerReferences: []metav1.OwnerReference{
				*metav1.NewControllerRef(mpiJob, kubeflow.SchemeGroupVersionKind),
			},
		},
		Spec: podgroupv1alpha1.PodGroupSpec{
			MinMember:         minAvailableReplicas,
			PriorityClassName: pName,
		},
	}
}

// newWorker creates a new worker StatefulSet for an MPIJob resource. It also
// sets the appropriate OwnerReferences on the resource so handleObject can
// discover the MPIJob resource that 'owns' it.
func newWorker(mpiJob *kubeflow.MPIJob, desiredReplicas int32, gangSchedulerName string) *appsv1.StatefulSet {
	labels := map[string]string{
		labelGroupName:   "kubeflow.org",
		labelMPIJobName:  mpiJob.Name,
		labelMPIRoleType: worker,
	}

	podSpec := mpiJob.Spec.MPIReplicaSpecs[kubeflow.MPIReplicaTypeWorker].Template.DeepCopy()

	// keep the labels which are set in PodTemplate
	if len(podSpec.Labels) == 0 {
		podSpec.Labels = make(map[string]string)
	}

	for key, value := range labels {
		podSpec.Labels[key] = value
	}
	// always set restartPolicy to restartAlways for statefulset
	podSpec.Spec.RestartPolicy = corev1.RestartPolicyAlways

	if len(podSpec.Spec.Containers) == 0 {
		klog.Errorln("Worker pod does not have any containers in its spec")
		return nil
	}
	container := podSpec.Spec.Containers[0]
	if len(container.Command) == 0 {
		container.Command = []string{"sleep"}
		container.Args = []string{"365d"}
	}

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

	// add SchedulerName to podSpec
	if gangSchedulerName != "" {
		if podSpec.Spec.SchedulerName != "" && podSpec.Spec.SchedulerName != gangSchedulerName {
			klog.Warningf("%s scheduler is specified when gang-scheduling is enabled and it will be overwritten", podSpec.Spec.SchedulerName)
		}
		podSpec.Spec.SchedulerName = gangSchedulerName

		if podSpec.Annotations == nil {
			podSpec.Annotations = map[string]string{}
		}
		// we create the podGroup with the same name as the mpijob
		podSpec.Annotations[podgroupv1alpha1.GroupNameAnnotationKey] = mpiJob.Name
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
func (c *MPIJobController) newLauncher(mpiJob *kubeflow.MPIJob, kubectlDeliveryImage string) *batchv1.Job {
	launcherName := mpiJob.Name + launcherSuffix
	labels := map[string]string{
		labelGroupName:   "kubeflow.org",
		labelMPIJobName:  mpiJob.Name,
		labelMPIRoleType: launcher,
	}

	podSpec := mpiJob.Spec.MPIReplicaSpecs[kubeflow.MPIReplicaTypeLauncher].Template.DeepCopy()
	// copy the labels and annotations to pod from PodTemplate
	if len(podSpec.Labels) == 0 {
		podSpec.Labels = make(map[string]string)
	}
	for key, value := range labels {
		podSpec.Labels[key] = value
	}
	// add SchedulerName to podSpec
	if c.gangSchedulerName != "" {
		if podSpec.Spec.SchedulerName != "" && podSpec.Spec.SchedulerName != c.gangSchedulerName {
			klog.Warningf("%s scheduler is specified when gang-scheduling is enabled and it will be overwritten", podSpec.Spec.SchedulerName)
		}
		podSpec.Spec.SchedulerName = c.gangSchedulerName

		if podSpec.Annotations == nil {
			podSpec.Annotations = map[string]string{}
		}
		// we create the podGroup with the same name as the mpijob
		podSpec.Annotations[podgroupv1alpha1.GroupNameAnnotationKey] = mpiJob.Name
	}
	podSpec.Spec.ServiceAccountName = launcherName
	podSpec.Spec.InitContainers = append(podSpec.Spec.InitContainers, corev1.Container{
		Name:            kubectlDeliveryName,
		Image:           kubectlDeliveryImage,
		ImagePullPolicy: corev1.PullIfNotPresent,
		Env: []corev1.EnvVar{
			{
				Name:  kubectlTargetDirEnv,
				Value: kubectlMountPath,
			},
			{
				Name:  "NAMESPACE",
				Value: mpiJob.Namespace,
			},
		},
		VolumeMounts: []corev1.VolumeMount{
			{
				Name:      kubectlVolumeName,
				MountPath: kubectlMountPath,
			},
			{
				Name:      configVolumeName,
				MountPath: configMountPath,
			},
		},
		Resources: corev1.ResourceRequirements{
			Limits: corev1.ResourceList{
				corev1.ResourceCPU:              resource.MustParse(initContainerCpu),
				corev1.ResourceMemory:           resource.MustParse(initContainerMem),
				corev1.ResourceEphemeralStorage: resource.MustParse(initContainerEphStorage),
			},
			Requests: corev1.ResourceList{
				corev1.ResourceCPU:              resource.MustParse(initContainerCpu),
				corev1.ResourceMemory:           resource.MustParse(initContainerMem),
				corev1.ResourceEphemeralStorage: resource.MustParse(initContainerEphStorage),
			},
		},
	})
	if len(podSpec.Spec.Containers) == 0 {
		klog.Errorln("Launcher pod does not have any containers in its spec")
		msg := fmt.Sprintf(MessageResourceDoesNotExists, "Launcher")
		c.recorder.Event(mpiJob, corev1.EventTypeWarning, ErrResourceDoesNotExists, msg)
		return nil
	}
	container := podSpec.Spec.Containers[0]
	container.Env = append(container.Env,
		corev1.EnvVar{
			Name:  "OMPI_MCA_plm_rsh_agent",
			Value: fmt.Sprintf("%s/%s", configMountPath, kubexecScriptName),
		},
		corev1.EnvVar{
			Name:  "OMPI_MCA_orte_default_hostfile",
			Value: fmt.Sprintf("%s/%s", configMountPath, hostfileName),
		},
		// We overwrite these environment variables so that users will not
		// be mistakenly using GPU resources for launcher due to potential
		// issues with scheduler/container technologies.
		corev1.EnvVar{
			Name:  "NVIDIA_VISIBLE_DEVICES",
			Value: "",
		},
		corev1.EnvVar{
			Name:  "NVIDIA_DRIVER_CAPABILITIES",
			Value: "",
		})

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

	// Submit a warning event if the user specifies restart policy for
	// the pod template. We recommend to set it from the replica level.
	if podSpec.Spec.RestartPolicy != corev1.RestartPolicy("") {
		errMsg := "Restart policy in pod template will be overwritten by restart policy in replica spec"
		klog.Warning(errMsg)
		c.recorder.Event(mpiJob, corev1.EventTypeWarning, podTemplateRestartPolicyReason, errMsg)
	}
	setRestartPolicy(podSpec, mpiJob.Spec.MPIReplicaSpecs[kubeflow.MPIReplicaTypeLauncher])

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
	backOffLimit := mpiJob.Spec.BackoffLimit
	activeDeadlineSeconds := mpiJob.Spec.ActiveDeadlineSeconds
	if mpiJob.Spec.RunPolicy != nil {
		warnMsg := fmt.Sprintf(
			"runPolicy is specified in MPIJobSpec so backOffLimit/activeDeadlineSeconds in MPIJobSpec will be overwritten")
		klog.Warning(warnMsg)
		if mpiJob.Spec.RunPolicy.BackoffLimit != nil {
			backOffLimit = mpiJob.Spec.RunPolicy.BackoffLimit
		}
		if mpiJob.Spec.RunPolicy.ActiveDeadlineSeconds != nil {
			activeDeadlineSeconds = mpiJob.Spec.RunPolicy.ActiveDeadlineSeconds
		}
	}
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
			BackoffLimit:          backOffLimit,
			ActiveDeadlineSeconds: activeDeadlineSeconds,
			Template:              *podSpec,
		},
	}
}

func setRestartPolicy(podTemplateSpec *corev1.PodTemplateSpec, spec *common.ReplicaSpec) {
	podTemplateSpec.Spec.RestartPolicy = corev1.RestartPolicy(spec.RestartPolicy)
}

func isJobFinished(j *batchv1.Job) bool {
	return isJobComplete(j) || isJobFailed(j)
}

func isJobFailed(j *batchv1.Job) bool {
	return hasJobCondition(j, batchv1.JobFailed)
}

func isJobComplete(j *batchv1.Job) bool {
	return hasJobCondition(j, batchv1.JobComplete)
}

func hasJobCondition(j *batchv1.Job, condType batchv1.JobConditionType) bool {
	for _, condition := range j.Status.Conditions {
		if condition.Type == condType && condition.Status == corev1.ConditionTrue {
			return true
		}
	}
	return false
}

func isCleanUpPods(cleanPodPolicy *common.CleanPodPolicy) bool {
	if *cleanPodPolicy == common.CleanPodPolicyAll || *cleanPodPolicy == common.CleanPodPolicyRunning {
		return true
	}
	return false
}
