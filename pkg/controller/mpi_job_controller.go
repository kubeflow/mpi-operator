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

package controller

import (
	"bytes"
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/x509"
	"encoding/pem"
	"fmt"
	"reflect"
	"sort"
	"strconv"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
	"golang.org/x/crypto/ssh"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/equality"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/apimachinery/pkg/util/wait"
	batchinformers "k8s.io/client-go/informers/batch/v1"
	coreinformers "k8s.io/client-go/informers/core/v1"
	"k8s.io/client-go/kubernetes"
	typedcorev1 "k8s.io/client-go/kubernetes/typed/core/v1"
	batchlisters "k8s.io/client-go/listers/batch/v1"
	corelisters "k8s.io/client-go/listers/core/v1"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/tools/record"
	"k8s.io/client-go/util/workqueue"
	"k8s.io/klog"
	podgroupv1beta1 "volcano.sh/apis/pkg/apis/scheduling/v1beta1"
	volcanoclient "volcano.sh/apis/pkg/client/clientset/versioned"
	podgroupsinformer "volcano.sh/apis/pkg/client/informers/externalversions/scheduling/v1beta1"
	podgroupslists "volcano.sh/apis/pkg/client/listers/scheduling/v1beta1"

	common "github.com/kubeflow/common/pkg/apis/common/v1"
	kubeflow "github.com/kubeflow/mpi-operator/v2/pkg/apis/kubeflow/v2beta1"
	"github.com/kubeflow/mpi-operator/v2/pkg/apis/kubeflow/validation"
	clientset "github.com/kubeflow/mpi-operator/v2/pkg/client/clientset/versioned"
	"github.com/kubeflow/mpi-operator/v2/pkg/client/clientset/versioned/scheme"
	informers "github.com/kubeflow/mpi-operator/v2/pkg/client/informers/externalversions/kubeflow/v2beta1"
	listers "github.com/kubeflow/mpi-operator/v2/pkg/client/listers/kubeflow/v2beta1"
)

const (
	controllerAgentName     = "mpi-job-controller"
	configSuffix            = "-config"
	configVolumeName        = "mpi-job-config"
	configMountPath         = "/etc/mpi"
	hostfileName            = "hostfile"
	discoverHostsScriptName = "discover_hosts.sh"
	sshAuthSecretSuffix     = "-ssh"
	sshAuthVolume           = "ssh-auth"
	rootSSHPath             = "/root/.ssh"
	launcher                = "launcher"
	worker                  = "worker"
	launcherSuffix          = "-launcher"
	workerSuffix            = "-worker"
	labelGroupName          = "group-name"
	labelMPIJobName         = "mpi-job-name"
	labelMPIRoleType        = "mpi-job-role"
	sshPublicKey            = "ssh-publickey"
	sshPrivateKeyFile       = "id_rsa"
	sshPublicKeyFile        = sshPrivateKeyFile + ".pub"
	sshAuthorizedKeysFile   = "authorized_keys"
)

const (
	// ErrResourceExists is used as part of the Event 'reason' when an MPIJob
	// fails to sync due to dependent resources of the same name already
	// existing.
	ErrResourceExists = "ErrResourceExists"

	// MessageResourceExists is the message used for Events when a resource
	// fails to sync due to dependent resources already existing.
	MessageResourceExists = "Resource %q of Kind %q already exists and is not managed by MPIJob"

	// ValidationError is used as part of the Event 'reason' when failed to
	// validate an MPIJob.
	ValidationError = "ValidationError"

	// podTemplateRestartPolicyReason is the warning reason when the restart
	// policy is set in pod template.
	podTemplateRestartPolicyReason = "SetPodTemplateRestartPolicy"

	// eventMessageLimit is the maximum size of an Event's message.
	// From: k8s.io/kubernetes/pkg/apis/core/validation/events.go
	eventMessageLimit = 1024

	// jobBackoffLimitExceededReason is the reason that the k8s job controller
	// uses when the backoff limit is exceeded.
	jobBackoffLimitExceededReason = "BackoffLimitExceeded"

	openMPISlotsEnv  = "OMPI_MCA_orte_set_default_slots"
	intelMPISlotsEnv = "I_MPI_PERHOST"
)

var (
	mpiJobsCreatedCount = promauto.NewCounter(prometheus.CounterOpts{
		Name: "mpi_operator_jobs_created_total",
		Help: "Counts number of MPI jobs created",
	})
	mpiJobsSuccessCount = promauto.NewCounter(prometheus.CounterOpts{
		Name: "mpi_operator_jobs_successful_total",
		Help: "Counts number of MPI jobs successful",
	})
	mpiJobsFailureCount = promauto.NewCounter(prometheus.CounterOpts{
		Name: "mpi_operator_jobs_failed_total",
		Help: "Counts number of MPI jobs failed",
	})
	mpiJobInfoGauge = promauto.NewGaugeVec(prometheus.GaugeOpts{
		Name: "mpi_operator_job_info",
		Help: "Information about MPIJob",
	}, []string{"launcher", "namespace"})

	sshVolumeItems = []corev1.KeyToPath{
		{
			Key:  corev1.SSHAuthPrivateKey,
			Path: sshPrivateKeyFile,
		},
		{
			Key:  sshPublicKey,
			Path: sshPublicKeyFile,
		},
		{
			Key:  sshPublicKey,
			Path: sshAuthorizedKeysFile,
		},
	}
	configVolumeItems = []corev1.KeyToPath{
		{
			Key:  hostfileName,
			Path: hostfileName,
			Mode: newInt32(0444),
		},
		{
			Key:  discoverHostsScriptName,
			Path: discoverHostsScriptName,
			Mode: newInt32(0555),
		},
	}

	launcherEnvVars = []corev1.EnvVar{
		{
			Name:  "K_MPI_JOB_ROLE",
			Value: launcher,
		},
	}
	workerEnvVars = []corev1.EnvVar{
		{
			Name:  "K_MPI_JOB_ROLE",
			Value: worker,
		},
	}
	ompiEnvVars = []corev1.EnvVar{
		// Allows driver to reach workers through the Service.
		{
			Name:  "OMPI_MCA_orte_keep_fqdn_hostnames",
			Value: "true",
		},
		{
			Name:  "OMPI_MCA_orte_default_hostfile",
			Value: fmt.Sprintf("%s/%s", configMountPath, hostfileName),
		},
		{
			Name:  "OMPI_MCA_plm_rsh_args",
			Value: "-o ConnectionAttempts=10",
		},
	}
	intelEnvVars = []corev1.EnvVar{
		{
			Name:  "I_MPI_HYDRA_HOST_FILE",
			Value: fmt.Sprintf("%s/%s", configMountPath, hostfileName),
		},
		{
			Name:  "I_MPI_HYDRA_BOOTSTRAP_EXEC_EXTRA_ARGS",
			Value: "-o ConnectionAttempts=10",
		},
	}
	nvidiaDisableEnvVars = []corev1.EnvVar{
		{Name: "NVIDIA_VISIBLE_DEVICES"},
		{Name: "NVIDIA_DRIVER_CAPABILITIES"},
	}
)

// MPIJobController is the controller implementation for MPIJob resources.
type MPIJobController struct {
	// kubeClient is a standard kubernetes clientset.
	kubeClient kubernetes.Interface
	// kubeflowClient is a clientset for our own API group.
	kubeflowClient clientset.Interface
	// volcanoClient is a clientset for volcano.sh API.
	volcanoClient volcanoclient.Interface

	configMapLister corelisters.ConfigMapLister
	configMapSynced cache.InformerSynced
	secretLister    corelisters.SecretLister
	secretSynced    cache.InformerSynced
	serviceLister   corelisters.ServiceLister
	serviceSynced   cache.InformerSynced
	jobLister       batchlisters.JobLister
	jobSynced       cache.InformerSynced
	podLister       corelisters.PodLister
	podSynced       cache.InformerSynced
	podgroupsLister podgroupslists.PodGroupLister
	podgroupsSynced cache.InformerSynced
	mpiJobLister    listers.MPIJobLister
	mpiJobSynced    cache.InformerSynced

	// queue is a rate limited work queue. This is used to queue work to be
	// processed instead of performing it as soon as a change happens. This
	// means we can ensure we only process a fixed amount of resources at a
	// time, and makes it easy to ensure we are never processing the same item
	// simultaneously in two different workers.
	queue workqueue.RateLimitingInterface
	// recorder is an event recorder for recording Event resources to the
	// Kubernetes API.
	recorder record.EventRecorder
	// Gang scheduler name to use
	gangSchedulerName string

	// To allow injection of updateStatus for testing.
	updateStatusHandler func(mpijob *kubeflow.MPIJob) error
}

// NewMPIJobController returns a new MPIJob controller.
func NewMPIJobController(
	kubeClient kubernetes.Interface,
	kubeflowClient clientset.Interface,
	volcanoClientSet volcanoclient.Interface,
	configMapInformer coreinformers.ConfigMapInformer,
	secretInformer coreinformers.SecretInformer,
	serviceInformer coreinformers.ServiceInformer,
	jobInformer batchinformers.JobInformer,
	podInformer coreinformers.PodInformer,
	podgroupsInformer podgroupsinformer.PodGroupInformer,
	mpiJobInformer informers.MPIJobInformer,
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
		kubeClient:        kubeClient,
		kubeflowClient:    kubeflowClient,
		volcanoClient:     volcanoClientSet,
		configMapLister:   configMapInformer.Lister(),
		configMapSynced:   configMapInformer.Informer().HasSynced,
		secretLister:      secretInformer.Lister(),
		secretSynced:      secretInformer.Informer().HasSynced,
		serviceLister:     serviceInformer.Lister(),
		serviceSynced:     serviceInformer.Informer().HasSynced,
		jobLister:         jobInformer.Lister(),
		jobSynced:         jobInformer.Informer().HasSynced,
		podLister:         podInformer.Lister(),
		podSynced:         podInformer.Informer().HasSynced,
		podgroupsLister:   podgroupsLister,
		podgroupsSynced:   podgroupsSynced,
		mpiJobLister:      mpiJobInformer.Lister(),
		mpiJobSynced:      mpiJobInformer.Informer().HasSynced,
		queue:             workqueue.NewNamedRateLimitingQueue(workqueue.DefaultControllerRateLimiter(), "MPIJobs"),
		recorder:          recorder,
		gangSchedulerName: gangSchedulerName,
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
		AddFunc:    controller.handleObject,
		UpdateFunc: controller.handleObjectUpdate,
		DeleteFunc: controller.handleObject,
	})
	secretInformer.Informer().AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc:    controller.handleObject,
		UpdateFunc: controller.handleObjectUpdate,
		DeleteFunc: controller.handleObject,
	})
	serviceInformer.Informer().AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc:    controller.handleObject,
		UpdateFunc: controller.handleObjectUpdate,
		DeleteFunc: controller.handleObject,
	})
	jobInformer.Informer().AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc:    controller.handleObject,
		UpdateFunc: controller.handleObjectUpdate,
		DeleteFunc: controller.handleObject,
	})
	podInformer.Informer().AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc:    controller.handleObject,
		UpdateFunc: controller.handleObjectUpdate,
		DeleteFunc: controller.handleObject,
	})
	if podgroupsInformer != nil {
		podgroupsInformer.Informer().AddEventHandler(cache.ResourceEventHandlerFuncs{
			AddFunc:    controller.handleObject,
			UpdateFunc: controller.handleObjectUpdate,
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
	if ok := cache.WaitForCacheSync(stopCh, c.configMapSynced, c.secretSynced, c.serviceSynced, c.jobSynced, c.podSynced, c.mpiJobSynced); !ok {
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

	// Get the MPIJob with this namespace/name.
	sharedJob, err := c.mpiJobLister.MPIJobs(namespace).Get(name)
	if err != nil {
		// The MPIJob may no longer exist, in which case we stop processing.
		if errors.IsNotFound(err) {
			klog.V(4).Infof("MPIJob has been deleted: %v", key)
			return nil
		}
		return fmt.Errorf("obtaining job: %w", err)
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

	if errs := validation.ValidateMPIJob(mpiJob); len(errs) != 0 {
		msg := truncateMessage(fmt.Sprintf("Found validation errors: %v", errs.ToAggregate()))
		c.recorder.Event(mpiJob, corev1.EventTypeWarning, ValidationError, msg)
		// Do not requeue
		return nil
	}

	if len(mpiJob.Status.Conditions) == 0 {
		msg := fmt.Sprintf("MPIJob %s/%s is created.", mpiJob.Namespace, mpiJob.Name)
		updateMPIJobConditions(mpiJob, common.JobCreated, mpiJobCreatedReason, msg)
		c.recorder.Event(mpiJob, corev1.EventTypeNormal, "MPIJobCreated", msg)
		mpiJobsCreatedCount.Inc()
	}

	// CompletionTime is only filled when the launcher Job succeeded or stopped
	// retrying (it reached .spec.backoffLimit). If it's filled, we want to
	// cleanup and stop retrying the MPIJob.
	if isFinished(mpiJob.Status) && mpiJob.Status.CompletionTime != nil {
		if isCleanUpPods(mpiJob.Spec.RunPolicy.CleanPodPolicy) {
			// set worker StatefulSet Replicas to 0.
			if err := c.deleteWorkerPods(mpiJob); err != nil {
				return err
			}
			initializeMPIJobStatuses(mpiJob, kubeflow.MPIReplicaTypeWorker)
			if c.gangSchedulerName != "" {
				if err := c.deletePodGroups(mpiJob); err != nil {
					return err
				}
			}
			mpiJob.Status.ReplicaStatuses[common.ReplicaType(kubeflow.MPIReplicaTypeWorker)].Active = 0
			return c.updateStatusHandler(mpiJob)
		}
		return nil
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

	var worker []*corev1.Pod
	// We're done if the launcher either succeeded or failed.
	done := launcher != nil && isJobFinished(launcher)
	if !done {
		_, err := c.getOrCreateService(mpiJob, newWorkersService(mpiJob))
		if err != nil {
			return fmt.Errorf("getting or creating Service to front workers: %w", err)
		}

		if config, err := c.getOrCreateConfigMap(mpiJob); config == nil || err != nil {
			return fmt.Errorf("getting or creating ConfigMap: %w", err)
		}

		_, err = c.getOrCreateSSHAuthSecret(mpiJob)
		if err != nil {
			return fmt.Errorf("creating SSH auth secret: %w", err)
		}

		// Get the PodGroup for this MPIJob
		if c.gangSchedulerName != "" {
			if podgroup, err := c.getOrCreatePodGroups(mpiJob, workerReplicas(mpiJob)+1); podgroup == nil || err != nil {
				return err
			}
		}

		worker, err = c.getOrCreateWorker(mpiJob)
		if err != nil {
			return err
		}
		if mpiJob.Spec.MPIImplementation == kubeflow.MPIImplementationIntel {
			// The Intel implementation requires workers to communicate with the
			// launcher through its hostname. For that, we create a Service which
			// has the same name as the launcher's hostname.
			_, err := c.getOrCreateService(mpiJob, newLauncherService(mpiJob))
			if err != nil {
				return fmt.Errorf("getting or creating Service to front launcher: %w", err)
			}
		}
		if launcher == nil {
			launcher, err = c.kubeClient.BatchV1().Jobs(namespace).Create(context.TODO(), c.newLauncherJob(mpiJob), metav1.CreateOptions{})
			if err != nil {
				c.recorder.Eventf(mpiJob, corev1.EventTypeWarning, mpiJobFailedReason, "launcher pod created failed: %v", err)
				return fmt.Errorf("creating launcher Pod: %w", err)
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

// getOrCreatePodGroups will create a PodGroup for gang scheduling by volcano.
func (c *MPIJobController) getOrCreatePodGroups(mpiJob *kubeflow.MPIJob, minAvailableWorkerReplicas int32) (*podgroupv1beta1.PodGroup, error) {
	podgroup, err := c.podgroupsLister.PodGroups(mpiJob.Namespace).Get(mpiJob.Name)
	// If the PodGroup doesn't exist, we'll create it.
	if errors.IsNotFound(err) {
		podgroup, err = c.volcanoClient.SchedulingV1beta1().PodGroups(mpiJob.Namespace).Create(context.TODO(), newPodGroup(mpiJob, minAvailableWorkerReplicas), metav1.CreateOptions{})
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
	err = c.volcanoClient.SchedulingV1beta1().PodGroups(mpiJob.Namespace).Delete(context.TODO(), mpiJob.Name, metav1.DeleteOptions{})
	// If an error occurs during Delete, we'll requeue the item so we
	// can attempt processing again later. This could have been caused by a
	// temporary network failure, or any other transient reason.
	if err != nil {
		return err
	}

	return nil
}

// getRunningWorkerPods get all worker Pods with Running phase controlled by this MPIJob.
func (c *MPIJobController) getRunningWorkerPods(mpiJob *kubeflow.MPIJob) ([]*corev1.Pod, error) {
	selector, err := workerSelector(mpiJob.Name)
	if err != nil {
		return nil, err
	}
	podFullList, err := c.podLister.List(selector)
	if err != nil {
		return nil, err
	}
	// Only running Pods should be included within the `discover_hosts.sh` script.
	var podList []*corev1.Pod
	for idx, pod := range podFullList {
		if pod.Status.Phase == corev1.PodRunning {
			podList = append(podList, podFullList[idx])
		}
	}

	return podList, nil
}

// getOrCreateConfigMap gets the ConfigMap controlled by this MPIJob, or creates
// one if it doesn't exist.
func (c *MPIJobController) getOrCreateConfigMap(mpiJob *kubeflow.MPIJob) (*corev1.ConfigMap, error) {
	newCM := newConfigMap(mpiJob, workerReplicas(mpiJob))
	podList, err := c.getRunningWorkerPods(mpiJob)
	if err != nil {
		return nil, err
	}
	updateDiscoverHostsInConfigMap(newCM, mpiJob, podList)

	cm, err := c.configMapLister.ConfigMaps(mpiJob.Namespace).Get(mpiJob.Name + configSuffix)
	// If the ConfigMap doesn't exist, we'll create it.
	if errors.IsNotFound(err) {
		return c.kubeClient.CoreV1().ConfigMaps(mpiJob.Namespace).Create(context.TODO(), newCM, metav1.CreateOptions{})
	}
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

	// If the ConfigMap is changed, update it
	if !equality.Semantic.DeepEqual(cm.Data, newCM.Data) {
		cm = cm.DeepCopy()
		cm.Data = newCM.Data
		cm, err = c.kubeClient.CoreV1().ConfigMaps(mpiJob.Namespace).Update(context.TODO(), cm, metav1.UpdateOptions{})
		if err != nil {
			return nil, err
		}
	}

	return cm, nil
}

func (c *MPIJobController) getOrCreateService(job *kubeflow.MPIJob, newSvc *corev1.Service) (*corev1.Service, error) {
	svc, err := c.serviceLister.Services(job.Namespace).Get(newSvc.Name)
	if errors.IsNotFound(err) {
		return c.kubeClient.CoreV1().Services(job.Namespace).Create(context.TODO(), newSvc, metav1.CreateOptions{})
	}
	if err != nil {
		return nil, err
	}
	if !metav1.IsControlledBy(svc, job) {
		msg := fmt.Sprintf(MessageResourceExists, svc.Name, svc.Kind)
		c.recorder.Event(job, corev1.EventTypeWarning, ErrResourceExists, msg)
		return nil, fmt.Errorf(msg)
	}

	// If the Service selector is changed, update it.
	if !equality.Semantic.DeepEqual(svc.Spec.Selector, newSvc.Spec.Selector) {
		svc = svc.DeepCopy()
		svc.Spec.Selector = newSvc.Spec.Selector
		return c.kubeClient.CoreV1().Services(svc.Namespace).Update(context.TODO(), svc, metav1.UpdateOptions{})
	}

	return svc, nil
}

// getOrCreateSSHAuthSecret gets the Secret holding the SSH auth for this job,
// or create one if it doesn't exist.
func (c *MPIJobController) getOrCreateSSHAuthSecret(job *kubeflow.MPIJob) (*corev1.Secret, error) {
	secret, err := c.secretLister.Secrets(job.Namespace).Get(job.Name + sshAuthSecretSuffix)
	if errors.IsNotFound(err) {
		secret, err := newSSHAuthSecret(job)
		if err != nil {
			return nil, err
		}
		return c.kubeClient.CoreV1().Secrets(job.Namespace).Create(context.TODO(), secret, metav1.CreateOptions{})
	}
	if err != nil {
		return nil, err
	}
	if !metav1.IsControlledBy(secret, job) {
		msg := fmt.Sprintf(MessageResourceExists, secret.Name, secret.Kind)
		c.recorder.Event(job, corev1.EventTypeWarning, ErrResourceExists, msg)
		return nil, fmt.Errorf(msg)
	}
	newSecret, err := newSSHAuthSecret(job)
	if err != nil {
		return nil, fmt.Errorf("generating new secret: %w", err)
	}
	hasKeys := keysFromData(secret.Data)
	wantKeys := keysFromData(newSecret.Data)
	if !equality.Semantic.DeepEqual(hasKeys, wantKeys) {
		secret := secret.DeepCopy()
		secret.Data = newSecret.Data
		return c.kubeClient.CoreV1().Secrets(secret.Namespace).Update(context.TODO(), secret, metav1.UpdateOptions{})
	}
	return secret, nil
}

func keysFromData(data map[string][]byte) []string {
	keys := make([]string, 0, len(data))
	for k := range data {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}

// getOrCreateWorkerStatefulSet gets the worker StatefulSet controlled by this
// MPIJob, or creates one if it doesn't exist.
func (c *MPIJobController) getOrCreateWorker(mpiJob *kubeflow.MPIJob) ([]*corev1.Pod, error) {
	var workerPods []*corev1.Pod
	worker := mpiJob.Spec.MPIReplicaSpecs[kubeflow.MPIReplicaTypeWorker]
	if worker == nil {
		return workerPods, nil
	}

	// Remove Pods when replicas are scaled down
	selector, err := workerSelector(mpiJob.Name)
	if err != nil {
		return nil, err
	}
	podFullList, err := c.podLister.List(selector)
	if err != nil {
		return nil, err
	}
	if len(podFullList) > int(*worker.Replicas) {
		for _, pod := range podFullList {
			indexStr, ok := pod.Labels[common.ReplicaIndexLabel]
			if !ok {
				return nil, err
			}
			index, err := strconv.Atoi(indexStr)
			if err == nil {
				if index >= int(*worker.Replicas) {
					err = c.kubeClient.CoreV1().Pods(pod.Namespace).Delete(context.TODO(), pod.Name, metav1.DeleteOptions{})
					if err != nil {
						return nil, err
					}
				}
			}
		}
	}

	for i := 0; i < int(*worker.Replicas); i++ {
		pod, err := c.podLister.Pods(mpiJob.Namespace).Get(workerName(mpiJob, i))

		// If the worker Pod doesn't exist, we'll create it.
		if errors.IsNotFound(err) {
			worker := c.newWorker(mpiJob, i)
			pod, err = c.kubeClient.CoreV1().Pods(mpiJob.Namespace).Create(context.TODO(), worker, metav1.CreateOptions{})
		}
		// If an error occurs during Get/Create, we'll requeue the item so we
		// can attempt processing again later. This could have been caused by a
		// temporary network failure, or any other transient reason.
		if err != nil {
			c.recorder.Eventf(mpiJob, corev1.EventTypeWarning, mpiJobFailedReason, "worker pod created failed: %v", err)
			return nil, err
		}
		// If the worker is not controlled by this MPIJob resource, we should log
		// a warning to the event recorder and return.
		if pod != nil && !metav1.IsControlledBy(pod, mpiJob) {
			msg := fmt.Sprintf(MessageResourceExists, pod.Name, pod.Kind)
			c.recorder.Event(mpiJob, corev1.EventTypeWarning, ErrResourceExists, msg)
			return nil, fmt.Errorf(msg)
		}
		workerPods = append(workerPods, pod)
	}

	return workerPods, nil
}

func (c *MPIJobController) deleteWorkerPods(mpiJob *kubeflow.MPIJob) error {
	var (
		workerPrefix       = mpiJob.Name + workerSuffix
		i            int32 = 0
	)
	worker := mpiJob.Spec.MPIReplicaSpecs[kubeflow.MPIReplicaTypeWorker]
	if worker == nil {
		return nil
	}

	for ; i < *worker.Replicas; i++ {
		name := fmt.Sprintf("%s-%d", workerPrefix, i)
		pod, err := c.podLister.Pods(mpiJob.Namespace).Get(name)

		// If the worker Pod doesn't exist, no need to remove it.
		if errors.IsNotFound(err) {
			continue
		}
		// If the worker is not controlled by this MPIJob resource, we should log
		// a warning to the event recorder and return.
		if pod != nil && !metav1.IsControlledBy(pod, mpiJob) {
			msg := fmt.Sprintf(MessageResourceExists, pod.Name, pod.Kind)
			c.recorder.Event(mpiJob, corev1.EventTypeWarning, ErrResourceExists, msg)
			return fmt.Errorf(msg)
		}
		// If the worker pod is not running and cleanupPolicy is
		// set to CleanPodPolicyRunning, keep the pod.
		// Note that pending pod should still be removed under this
		// situation, since it may turn to running in the future.
		if *mpiJob.Spec.RunPolicy.CleanPodPolicy == common.CleanPodPolicyRunning && !isPodRunning(pod) && !isPodPending(pod) {
			// Keep the worker pod
			continue
		}
		err = c.kubeClient.CoreV1().Pods(mpiJob.Namespace).Delete(context.TODO(), name, metav1.DeleteOptions{})
		if err != nil && !errors.IsNotFound(err) {
			klog.Errorf("Failed to delete pod[%s/%s]: %v", mpiJob.Namespace, name, err)
			return err
		}
	}
	return nil
}

func (c *MPIJobController) updateMPIJobStatus(mpiJob *kubeflow.MPIJob, launcher *batchv1.Job, worker []*corev1.Pod) error {
	oldStatus := mpiJob.Status.DeepCopy()
	launcherPods, err := c.jobPods(launcher)
	if err != nil {
		return fmt.Errorf("checking launcher pods running: %w", err)
	}
	// Job.status.Active accounts for Pending and Running pods. Count running pods
	// from the lister instead.
	launcherPodsCnt := countRunningPods(launcherPods)
	if launcher != nil {
		initializeMPIJobStatuses(mpiJob, kubeflow.MPIReplicaTypeLauncher)
		launcherStatus := mpiJob.Status.ReplicaStatuses[common.ReplicaType(kubeflow.MPIReplicaTypeLauncher)]
		launcherStatus.Failed = launcher.Status.Failed
		if isJobSucceeded(launcher) {
			launcherStatus.Succeeded = 1
			msg := fmt.Sprintf("MPIJob %s/%s successfully completed.", mpiJob.Namespace, mpiJob.Name)
			c.recorder.Event(mpiJob, corev1.EventTypeNormal, mpiJobSucceededReason, msg)
			if mpiJob.Status.CompletionTime == nil {
				mpiJob.Status.CompletionTime = launcher.Status.CompletionTime
			}
			updateMPIJobConditions(mpiJob, common.JobSucceeded, mpiJobSucceededReason, msg)
			mpiJobsSuccessCount.Inc()
		} else if isJobFailed(launcher) {
			c.updateMPIJobFailedStatus(mpiJob, launcher, launcherPods)
		} else {
			mpiJob.Status.ReplicaStatuses[common.ReplicaType(kubeflow.MPIReplicaTypeLauncher)].Active = int32(launcherPodsCnt)
		}
		mpiJobInfoGauge.WithLabelValues(launcher.Name, mpiJob.Namespace).Set(1)
	}

	var (
		running = 0
		evict   = 0
	)

	initializeMPIJobStatuses(mpiJob, kubeflow.MPIReplicaTypeWorker)
	//spec := mpiJob.Spec.MPIReplicaSpecs[kubeflow.MPIReplicaTypeWorker]
	for i := 0; i < len(worker); i++ {
		switch worker[i].Status.Phase {
		case corev1.PodFailed:
			mpiJob.Status.ReplicaStatuses[common.ReplicaType(kubeflow.MPIReplicaTypeWorker)].Failed += 1
			if worker[i].Status.Reason == "Evicted" {
				evict += 1
			}
		case corev1.PodSucceeded:
			mpiJob.Status.ReplicaStatuses[common.ReplicaType(kubeflow.MPIReplicaTypeWorker)].Succeeded += 1
		case corev1.PodRunning:
			running += 1
			mpiJob.Status.ReplicaStatuses[common.ReplicaType(kubeflow.MPIReplicaTypeWorker)].Active += 1
		}
	}
	if evict > 0 {
		msg := fmt.Sprintf("%d/%d workers are evicted", evict, len(worker))
		klog.Infof("MPIJob <%s/%s>: %v", mpiJob.Namespace, mpiJob.Name, msg)
		updateMPIJobConditions(mpiJob, common.JobFailed, mpiJobEvict, msg)
		c.recorder.Event(mpiJob, corev1.EventTypeWarning, mpiJobEvict, msg)
	}

	if launcher != nil && launcherPodsCnt >= 1 && running == len(worker) {
		msg := fmt.Sprintf("MPIJob %s/%s is running.", mpiJob.Namespace, mpiJob.Name)
		updateMPIJobConditions(mpiJob, common.JobRunning, mpiJobRunningReason, msg)
		c.recorder.Eventf(mpiJob, corev1.EventTypeNormal, "MPIJobRunning", "MPIJob %s/%s is running", mpiJob.Namespace, mpiJob.Name)
	}

	// no need to update the mpijob if the status hasn't changed since last time.
	if !reflect.DeepEqual(*oldStatus, mpiJob.Status) {
		return c.updateStatusHandler(mpiJob)
	}
	return nil
}

func (c *MPIJobController) updateMPIJobFailedStatus(mpiJob *kubeflow.MPIJob, launcher *batchv1.Job, launcherPods []*corev1.Pod) {
	jobFailedCond := getJobCondition(launcher, batchv1.JobFailed)
	reason := jobFailedCond.Reason
	if reason == "" {
		reason = mpiJobFailedReason
	}
	msg := jobFailedCond.Message
	if msg == "" {
		msg = fmt.Sprintf("MPIJob %s/%s has failed", mpiJob.Namespace, mpiJob.Name)
	}
	if reason == jobBackoffLimitExceededReason {
		// Concatenate the reason and message from the last failed Pod.
		var lastFailedPod *corev1.Pod
		for _, p := range launcherPods {
			if isPodFailed(p) && (lastFailedPod == nil || lastFailedPod.CreationTimestamp.Before(&p.CreationTimestamp)) {
				lastFailedPod = p
			}
		}
		if lastFailedPod != nil {
			reason += "/" + lastFailedPod.Status.Reason
			msg += ": " + lastFailedPod.Status.Message
			msg = truncateMessage(msg)
		}
	}
	c.recorder.Event(mpiJob, corev1.EventTypeWarning, reason, msg)
	if mpiJob.Status.CompletionTime == nil {
		now := metav1.Now()
		mpiJob.Status.CompletionTime = &now
	}
	updateMPIJobConditions(mpiJob, common.JobFailed, reason, msg)
	mpiJobsFailureCount.Inc()
}

// When a mpiJob is added, set the defaults and enqueue the current mpiJob.
func (c *MPIJobController) addMPIJob(obj interface{}) {
	mpiJob := obj.(*kubeflow.MPIJob)

	// Set default for the new mpiJob.
	scheme.Scheme.Default(mpiJob)
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
	ownerRef, ownerGVK, err := ownerReferenceAndGVK(object)
	if err != nil {
		runtime.HandleError(err)
		return
	}

	// If the Pod is controlled by a Job, get the Job's ownerReference.
	if ownerGVK.Group == batchv1.GroupName && ownerGVK.Kind == "Job" {
		j, err := c.jobLister.Jobs(object.GetNamespace()).Get(ownerRef.Name)
		if err != nil {
			runtime.HandleError(fmt.Errorf("obtaining owning k8s Job: %w", err))
			return
		}
		ownerRef, ownerGVK, err = ownerReferenceAndGVK(j)
		if err != nil {
			runtime.HandleError(fmt.Errorf("obtaining k8s Job's owner: %w", err))
			return
		}
	}

	// Compare the OwnerReference Group and Kind against the OwnerType Group and Kind.
	// Since we do not support conversion webhook now, we do not deal with v1alpha1/v1alpha2/v1 resources in this operator.
	if ownerGVK.Kind != kubeflow.Kind || ownerGVK.Group != kubeflow.GroupName || ownerGVK.Version != kubeflow.GroupVersion {
		return
	}

	mpiJob, err := c.mpiJobLister.MPIJobs(object.GetNamespace()).Get(ownerRef.Name)
	if err != nil {
		klog.V(4).Infof("ignoring orphaned object '%s' of mpi job '%s'", object.GetSelfLink(), ownerRef.Name)
		return
	}

	c.enqueueMPIJob(mpiJob)
}

func (c *MPIJobController) handleObjectUpdate(old, new interface{}) {
	oldObj := old.(metav1.Object)
	newObj := new.(metav1.Object)
	if newObj.GetResourceVersion() == oldObj.GetResourceVersion() {
		// Periodic re-sync will send update events for all known
		// ConfigMaps. Two different versions of the same ConfigMap
		// will always have different RVs.
		return
	}
	c.handleObject(new)
}

// doUpdateJobStatus updates the status of the given MPIJob by call apiServer.
func (c *MPIJobController) doUpdateJobStatus(mpiJob *kubeflow.MPIJob) error {
	_, err := c.kubeflowClient.KubeflowV2beta1().MPIJobs(mpiJob.Namespace).UpdateStatus(context.TODO(), mpiJob, metav1.UpdateOptions{})
	return err
}

// newConfigMap creates a new ConfigMap containing configurations for an MPIJob
// resource. It also sets the appropriate OwnerReferences on the resource so
// handleObject can discover the MPIJob resource that 'owns' it.
func newConfigMap(mpiJob *kubeflow.MPIJob, workerReplicas int32) *corev1.ConfigMap {
	var buffer bytes.Buffer
	workersService := mpiJob.Name + workerSuffix
	for i := 0; i < int(workerReplicas); i++ {
		buffer.WriteString(fmt.Sprintf("%s%s-%d.%s.%s.svc\n", mpiJob.Name, workerSuffix, i, workersService, mpiJob.Namespace))
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
			hostfileName: buffer.String(),
		},
	}
}

// updateDiscoverHostsInConfigMap updates the ConfigMap if the content of `discover_hosts.sh` changes.
func updateDiscoverHostsInConfigMap(configMap *corev1.ConfigMap, mpiJob *kubeflow.MPIJob, runningPods []*corev1.Pod) {
	// Sort the slice of Pods to make sure the order of entries in `discover_hosts.sh` is maintained.
	sort.Slice(runningPods, func(i, j int) bool {
		return runningPods[i].Name < runningPods[j].Name
	})

	var buffer bytes.Buffer
	buffer.WriteString("#!/bin/sh\n")
	workersService := mpiJob.Name + workerSuffix
	for _, p := range runningPods {
		buffer.WriteString(fmt.Sprintf("echo %s.%s.%s.svc\n", p.Name, workersService, p.Namespace))
	}

	configMap.Data[discoverHostsScriptName] = buffer.String()
}

// newWorkersService creates a new workers' Service for an MPIJob resource.
func newWorkersService(job *kubeflow.MPIJob) *corev1.Service {
	return newService(job, job.Name+workerSuffix, defaultLabels(job.Name, worker))
}

// newLauncherService creates a new launcher's Service for an MPIJob resource.
func newLauncherService(job *kubeflow.MPIJob) *corev1.Service {
	return newService(job, job.Name+launcherSuffix, defaultLabels(job.Name, launcher))
}

func newService(job *kubeflow.MPIJob, name string, selector map[string]string) *corev1.Service {
	return &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: job.Namespace,
			Labels: map[string]string{
				"app": job.Name,
			},
			OwnerReferences: []metav1.OwnerReference{
				*metav1.NewControllerRef(job, kubeflow.SchemeGroupVersionKind),
			},
		},
		Spec: corev1.ServiceSpec{
			ClusterIP: corev1.ClusterIPNone,
			Selector:  selector,
		},
	}
}

// newSSHAuthSecret creates a new Secret that holds SSH auth: a private Key
// and its public key version.
func newSSHAuthSecret(job *kubeflow.MPIJob) (*corev1.Secret, error) {
	privateKey, err := ecdsa.GenerateKey(elliptic.P521(), rand.Reader)
	if err != nil {
		return nil, fmt.Errorf("generating private SSH key: %w", err)
	}
	privateDER, err := x509.MarshalECPrivateKey(privateKey)
	if err != nil {
		return nil, fmt.Errorf("converting private SSH key to DER format: %w", err)
	}
	privatePEM := pem.EncodeToMemory(&pem.Block{
		Type:  "EC PRIVATE KEY",
		Bytes: privateDER,
	})

	publicKey, err := ssh.NewPublicKey(&privateKey.PublicKey)
	if err != nil {
		return nil, fmt.Errorf("generating public SSH key: %w", err)
	}
	return &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      job.Name + sshAuthSecretSuffix,
			Namespace: job.Namespace,
			Labels: map[string]string{
				"app": job.Name,
			},
			OwnerReferences: []metav1.OwnerReference{
				*metav1.NewControllerRef(job, kubeflow.SchemeGroupVersionKind),
			},
		},
		Type: corev1.SecretTypeSSHAuth,
		Data: map[string][]byte{
			corev1.SSHAuthPrivateKey: privatePEM,
			sshPublicKey:             ssh.MarshalAuthorizedKey(publicKey),
		},
	}, nil
}

// newPodGroup creates a new PodGroup for an MPIJob
// resource. It also sets the appropriate OwnerReferences on the resource so
// handleObject can discover the MPIJob resource that 'owns' it.
func newPodGroup(mpiJob *kubeflow.MPIJob, minAvailableReplicas int32) *podgroupv1beta1.PodGroup {
	var pName string
	if l := mpiJob.Spec.MPIReplicaSpecs[kubeflow.MPIReplicaTypeLauncher]; l != nil {
		pName = l.Template.Spec.PriorityClassName
		if w := mpiJob.Spec.MPIReplicaSpecs[kubeflow.MPIReplicaTypeWorker]; pName == "" && w != nil {
			pName = w.Template.Spec.PriorityClassName
		}
	}
	return &podgroupv1beta1.PodGroup{
		ObjectMeta: metav1.ObjectMeta{
			Name:      mpiJob.Name,
			Namespace: mpiJob.Namespace,
			OwnerReferences: []metav1.OwnerReference{
				*metav1.NewControllerRef(mpiJob, kubeflow.SchemeGroupVersionKind),
			},
		},
		Spec: podgroupv1beta1.PodGroupSpec{
			MinMember:         minAvailableReplicas,
			Queue:             mpiJob.Annotations[podgroupv1beta1.QueueNameAnnotationKey],
			PriorityClassName: pName,
		},
	}
}

func workerName(mpiJob *kubeflow.MPIJob, index int) string {
	return fmt.Sprintf("%s%s-%d", mpiJob.Name, workerSuffix, index)
}

// newWorker creates a new worker StatefulSet for an MPIJob resource. It also
// sets the appropriate OwnerReferences on the resource so handleObject can
// discover the MPIJob resource that 'owns' it.
func (c *MPIJobController) newWorker(mpiJob *kubeflow.MPIJob, index int) *corev1.Pod {
	name := workerName(mpiJob, index)

	podTemplate := mpiJob.Spec.MPIReplicaSpecs[kubeflow.MPIReplicaTypeWorker].Template.DeepCopy()

	// keep the labels which are set in PodTemplate
	if len(podTemplate.Labels) == 0 {
		podTemplate.Labels = make(map[string]string)
	}
	for key, value := range defaultLabels(mpiJob.Name, worker) {
		podTemplate.Labels[key] = value
	}
	podTemplate.Labels[common.ReplicaIndexLabel] = strconv.Itoa(index)
	podTemplate.Spec.Hostname = name
	podTemplate.Spec.Subdomain = mpiJob.Name + workerSuffix // Matches workers' Service name.
	if podTemplate.Spec.HostNetwork {
		// Allows resolution of worker hostnames without needing to include the
		// namespace or cluster domain.
		podTemplate.Spec.DNSPolicy = corev1.DNSClusterFirstWithHostNet
	}
	setRestartPolicy(podTemplate, mpiJob.Spec.MPIReplicaSpecs[kubeflow.MPIReplicaTypeWorker])

	container := &podTemplate.Spec.Containers[0]
	if len(container.Command) == 0 && len(container.Args) == 0 {
		container.Command = []string{"/usr/sbin/sshd", "-De"}
	}
	container.Env = append(container.Env, workerEnvVars...)
	c.setupSSHOnPod(&podTemplate.Spec, mpiJob)

	// add SchedulerName to podSpec
	if c.gangSchedulerName != "" {
		if podTemplate.Spec.SchedulerName != "" && podTemplate.Spec.SchedulerName != c.gangSchedulerName {
			klog.Warningf("%s scheduler is specified when gang-scheduling is enabled and it will be overwritten", podTemplate.Spec.SchedulerName)
		}
		podTemplate.Spec.SchedulerName = c.gangSchedulerName

		if podTemplate.Annotations == nil {
			podTemplate.Annotations = map[string]string{}
		}
		// we create the podGroup with the same name as the mpijob
		podTemplate.Annotations[podgroupv1beta1.KubeGroupNameAnnotationKey] = mpiJob.Name
	}

	return &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:        name,
			Namespace:   mpiJob.Namespace,
			Labels:      podTemplate.Labels,
			Annotations: podTemplate.Annotations,
			OwnerReferences: []metav1.OwnerReference{
				*metav1.NewControllerRef(mpiJob, kubeflow.SchemeGroupVersionKind),
			},
		},
		Spec: podTemplate.Spec,
	}
}

func (c *MPIJobController) newLauncherJob(mpiJob *kubeflow.MPIJob) *batchv1.Job {
	return &batchv1.Job{
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
		Spec: batchv1.JobSpec{
			TTLSecondsAfterFinished: mpiJob.Spec.RunPolicy.TTLSecondsAfterFinished,
			ActiveDeadlineSeconds:   mpiJob.Spec.RunPolicy.ActiveDeadlineSeconds,
			BackoffLimit:            mpiJob.Spec.RunPolicy.BackoffLimit,
			Template:                c.newLauncherPodTemplate(mpiJob),
		},
	}
}

// newLauncherPodTemplate creates a new launcher Job for an MPIJob resource. It also sets
// the appropriate OwnerReferences on the resource so handleObject can discover
// the MPIJob resource that 'owns' it.
func (c *MPIJobController) newLauncherPodTemplate(mpiJob *kubeflow.MPIJob) corev1.PodTemplateSpec {
	launcherName := mpiJob.Name + launcherSuffix

	podTemplate := mpiJob.Spec.MPIReplicaSpecs[kubeflow.MPIReplicaTypeLauncher].Template.DeepCopy()
	// copy the labels and annotations to pod from PodTemplate
	if len(podTemplate.Labels) == 0 {
		podTemplate.Labels = make(map[string]string)
	}
	for key, value := range defaultLabels(mpiJob.Name, launcher) {
		podTemplate.Labels[key] = value
	}
	// add SchedulerName to podSpec
	if c.gangSchedulerName != "" {
		if podTemplate.Spec.SchedulerName != "" && podTemplate.Spec.SchedulerName != c.gangSchedulerName {
			klog.Warningf("%s scheduler is specified when gang-scheduling is enabled and it will be overwritten", podTemplate.Spec.SchedulerName)
		}
		podTemplate.Spec.SchedulerName = c.gangSchedulerName

		if podTemplate.Annotations == nil {
			podTemplate.Annotations = map[string]string{}
		}
		// we create the podGroup with the same name as the mpijob
		podTemplate.Annotations[podgroupv1beta1.KubeGroupNameAnnotationKey] = mpiJob.Name
	}
	podTemplate.Spec.Hostname = launcherName
	podTemplate.Spec.Subdomain = mpiJob.Name + workerSuffix // Matches workers' Service name.
	if podTemplate.Spec.HostNetwork {
		// Allows resolution of worker hostnames without needing to include the
		// namespace or cluster domain.
		podTemplate.Spec.DNSPolicy = corev1.DNSClusterFirstWithHostNet
	}
	container := &podTemplate.Spec.Containers[0]
	container.Env = append(container.Env, launcherEnvVars...)
	slotsStr := strconv.Itoa(int(*mpiJob.Spec.SlotsPerWorker))
	switch mpiJob.Spec.MPIImplementation {
	case kubeflow.MPIImplementationOpenMPI:
		container.Env = append(container.Env, ompiEnvVars...)
		container.Env = append(container.Env, corev1.EnvVar{
			Name:  openMPISlotsEnv,
			Value: slotsStr,
		})
	case kubeflow.MPIImplementationIntel:
		container.Env = append(container.Env, intelEnvVars...)
		container.Env = append(container.Env, corev1.EnvVar{
			Name:  intelMPISlotsEnv,
			Value: slotsStr,
		})
	}

	container.Env = append(container.Env,
		// We overwrite these environment variables so that users will not
		// be mistakenly using GPU resources for launcher due to potential
		// issues with scheduler/container technologies.
		nvidiaDisableEnvVars...)
	c.setupSSHOnPod(&podTemplate.Spec, mpiJob)

	// Submit a warning event if the user specifies restart policy for
	// the pod template. We recommend to set it from the replica level.
	if podTemplate.Spec.RestartPolicy != "" {
		errMsg := "Restart policy in pod template overridden by restart policy in replica spec"
		klog.Warning(errMsg)
		c.recorder.Event(mpiJob, corev1.EventTypeWarning, podTemplateRestartPolicyReason, errMsg)
	}
	setRestartPolicy(podTemplate, mpiJob.Spec.MPIReplicaSpecs[kubeflow.MPIReplicaTypeLauncher])

	podTemplate.Spec.Volumes = append(podTemplate.Spec.Volumes,
		corev1.Volume{
			Name: configVolumeName,
			VolumeSource: corev1.VolumeSource{
				ConfigMap: &corev1.ConfigMapVolumeSource{
					LocalObjectReference: corev1.LocalObjectReference{
						Name: mpiJob.Name + configSuffix,
					},
					Items: configVolumeItems,
				},
			},
		})
	container.VolumeMounts = append(container.VolumeMounts, corev1.VolumeMount{
		Name:      configVolumeName,
		MountPath: configMountPath,
	})

	return corev1.PodTemplateSpec{
		ObjectMeta: metav1.ObjectMeta{
			Labels:      podTemplate.Labels,
			Annotations: podTemplate.Annotations,
			OwnerReferences: []metav1.OwnerReference{
				*metav1.NewControllerRef(mpiJob, kubeflow.SchemeGroupVersionKind),
			},
		},
		Spec: podTemplate.Spec,
	}
}

func (c *MPIJobController) jobPods(j *batchv1.Job) ([]*corev1.Pod, error) {
	selector, err := metav1.LabelSelectorAsSelector(j.Spec.Selector)
	if err != nil {
		return nil, fmt.Errorf("parsing Pod selector: %w", err)
	}
	pods, err := c.podLister.Pods(j.Namespace).List(selector)
	if err != nil {
		return nil, fmt.Errorf("obtaining pods: %w", err)
	}
	var result = make([]*corev1.Pod, 0, len(pods))
	for _, p := range pods {
		if metav1.IsControlledBy(p, j) {
			result = append(result, p)
		}
	}
	return result, nil
}

func countRunningPods(pods []*corev1.Pod) int {
	running := 0
	for _, p := range pods {
		if isPodRunning(p) {
			running++
		}
	}
	return running
}

func setRestartPolicy(podTemplateSpec *corev1.PodTemplateSpec, spec *common.ReplicaSpec) {
	if spec.RestartPolicy == common.RestartPolicyExitCode {
		podTemplateSpec.Spec.RestartPolicy = corev1.RestartPolicyNever
	} else {
		podTemplateSpec.Spec.RestartPolicy = corev1.RestartPolicy(spec.RestartPolicy)
	}
}

func isJobFinished(j *batchv1.Job) bool {
	return isJobSucceeded(j) || isJobFailed(j)
}

func isJobFailed(j *batchv1.Job) bool {
	c := getJobCondition(j, batchv1.JobFailed)
	return c != nil && c.Status == corev1.ConditionTrue
}

func isJobSucceeded(j *batchv1.Job) bool {
	c := getJobCondition(j, batchv1.JobComplete)
	return c != nil && c.Status == corev1.ConditionTrue
}

func getJobCondition(j *batchv1.Job, condition batchv1.JobConditionType) *batchv1.JobCondition {
	for _, c := range j.Status.Conditions {
		if c.Type == condition {
			return &c
		}
	}
	return nil
}

func isPodRunning(p *corev1.Pod) bool {
	return p.Status.Phase == corev1.PodRunning
}

func isPodPending(p *corev1.Pod) bool {
	return p.Status.Phase == corev1.PodPending
}

func isPodFailed(p *corev1.Pod) bool {
	return p.Status.Phase == corev1.PodFailed
}

func isCleanUpPods(cleanPodPolicy *common.CleanPodPolicy) bool {
	if *cleanPodPolicy == common.CleanPodPolicyAll || *cleanPodPolicy == common.CleanPodPolicyRunning {
		return true
	}
	return false
}

func defaultLabels(jobName, role string) map[string]string {
	return map[string]string{
		common.OperatorNameLabel: kubeflow.OperatorName,
		common.JobNameLabel:      jobName,
		common.JobRoleLabel:      role,
	}
}

func workerSelector(mpiJobName string) (labels.Selector, error) {
	set := defaultLabels(mpiJobName, worker)
	return labels.ValidatedSelectorFromSet(set)
}

func workerReplicas(job *kubeflow.MPIJob) int32 {
	workerSpec := job.Spec.MPIReplicaSpecs[kubeflow.MPIReplicaTypeWorker]
	if workerSpec != nil && workerSpec.Replicas != nil {
		return *workerSpec.Replicas
	}
	return 0
}

func (c *MPIJobController) setupSSHOnPod(podSpec *corev1.PodSpec, job *kubeflow.MPIJob) {
	var mode *int32
	if job.Spec.SSHAuthMountPath == rootSSHPath {
		mode = newInt32(0600)
	}
	mainContainer := &podSpec.Containers[0]
	podSpec.Volumes = append(podSpec.Volumes,
		corev1.Volume{
			Name: sshAuthVolume,
			VolumeSource: corev1.VolumeSource{
				Secret: &corev1.SecretVolumeSource{
					DefaultMode: mode,
					SecretName:  job.Name + sshAuthSecretSuffix,
					Items:       sshVolumeItems,
				},
			},
		})

	mainContainer.VolumeMounts = append(mainContainer.VolumeMounts,
		corev1.VolumeMount{
			Name:      sshAuthVolume,
			MountPath: job.Spec.SSHAuthMountPath,
		})
}

func ownerReferenceAndGVK(object metav1.Object) (*metav1.OwnerReference, schema.GroupVersionKind, error) {
	ownerRef := metav1.GetControllerOf(object)
	if ownerRef == nil {
		return nil, schema.GroupVersionKind{}, nil
	}
	gv, err := schema.ParseGroupVersion(ownerRef.APIVersion)
	if err != nil {
		return nil, schema.GroupVersionKind{}, fmt.Errorf("parsing owner's API version: %w", err)
	}
	return ownerRef, gv.WithKind(ownerRef.Kind), nil
}

func newInt32(v int32) *int32 {
	return &v
}

// truncateMessage truncates a message if it hits the NoteLengthLimit.
func truncateMessage(message string) string {
	if len(message) <= eventMessageLimit {
		return message
	}
	suffix := "..."
	return message[:eventMessageLimit-len(suffix)] + suffix
}
