// Copyright 2023 The Kubeflow Authors.
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
	"context"
	"sort"

	"github.com/google/go-cmp/cmp"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/equality"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	schedulinglisters "k8s.io/client-go/listers/scheduling/v1"
	"k8s.io/client-go/tools/cache"
	"k8s.io/klog"
	"k8s.io/utils/ptr"
	schedv1alpha1 "sigs.k8s.io/scheduler-plugins/apis/scheduling/v1alpha1"
	schedclientset "sigs.k8s.io/scheduler-plugins/pkg/generated/clientset/versioned"
	schedinformers "sigs.k8s.io/scheduler-plugins/pkg/generated/informers/externalversions"
	schedinformer "sigs.k8s.io/scheduler-plugins/pkg/generated/informers/externalversions/scheduling/v1alpha1"
	volcanov1beta1 "volcano.sh/apis/pkg/apis/scheduling/v1beta1"
	volcanoclient "volcano.sh/apis/pkg/client/clientset/versioned"
	volcanoinformers "volcano.sh/apis/pkg/client/informers/externalversions"
	volcanopodgroupinformer "volcano.sh/apis/pkg/client/informers/externalversions/scheduling/v1beta1"

	"github.com/kubeflow/mpi-operator/cmd/mpi-operator/app/options"
	kubeflow "github.com/kubeflow/mpi-operator/pkg/apis/kubeflow/v2beta1"
)

type PodGroupControl interface {
	// StartInformerFactory will start the podGroup informer.
	StartInformerFactory(stopCh <-chan struct{})
	// PodGroupSharedIndexInformer will return Indexer based on SharedInformer for the podGroup.
	PodGroupSharedIndexInformer() cache.SharedIndexInformer
	// newPodGroup will generate a new podGroup for an MPIJob resource.
	// It also sets the appropriate OwnerReferences on the resource so
	// handleObject can discover the MPIJob resource that 'owns' it.
	newPodGroup(mpiJob *kubeflow.MPIJob) metav1.Object
	// getPodGroup will return a podGroup.
	getPodGroup(namespace, name string) (metav1.Object, error)
	// createPodGroup will create  a podGroup.
	createPodGroup(ctx context.Context, pg metav1.Object) (metav1.Object, error)
	// updatePodGroup will update a podGroup.
	updatePodGroup(ctx context.Context, old, new metav1.Object) (metav1.Object, error)
	// deletePodGroup will delete a podGroup.
	deletePodGroup(ctx context.Context, namespace, name string) error
	// decoratePodTemplateSpec will decorate the podTemplate before it's used to generate a pod with information for gang-scheduling.
	decoratePodTemplateSpec(pts *corev1.PodTemplateSpec, mpiJobName string)
	// calculatePGMinResources will calculate minResources for podGroup.
	calculatePGMinResources(minMember *int32, mpiJob *kubeflow.MPIJob) *corev1.ResourceList
	// pgSpecsAreEqual will return true if the spec fields of two podGroup are equals.
	pgSpecsAreEqual(a, b metav1.Object) bool
}

// VolcanoCtrl is the implementation fo PodGroupControl with volcano.
type VolcanoCtrl struct {
	Client              volcanoclient.Interface
	InformerFactory     volcanoinformers.SharedInformerFactory
	PodGroupInformer    volcanopodgroupinformer.PodGroupInformer
	PriorityClassLister schedulinglisters.PriorityClassLister
	schedulerName       string
}

func NewVolcanoCtrl(c volcanoclient.Interface, watchNamespace string, pcLister schedulinglisters.PriorityClassLister) *VolcanoCtrl {
	var informerFactoryOpts []volcanoinformers.SharedInformerOption
	if watchNamespace != metav1.NamespaceAll {
		informerFactoryOpts = append(informerFactoryOpts, volcanoinformers.WithNamespace(watchNamespace))
	}
	informerFactory := volcanoinformers.NewSharedInformerFactoryWithOptions(c, 0, informerFactoryOpts...)
	return &VolcanoCtrl{
		Client:              c,
		InformerFactory:     informerFactory,
		PodGroupInformer:    informerFactory.Scheduling().V1beta1().PodGroups(),
		PriorityClassLister: pcLister,
		schedulerName:       options.GangSchedulerVolcano,
	}
}

func (v *VolcanoCtrl) PodGroupSharedIndexInformer() cache.SharedIndexInformer {
	return v.PodGroupInformer.Informer()
}

func (v *VolcanoCtrl) StartInformerFactory(stopCh <-chan struct{}) {
	go v.InformerFactory.Start(stopCh)
}

// newPodGroup will generate a new PodGroup for an MPIJob resource.
// If the parameters set in the schedulingPolicy aren't empty, it will pass them to a new PodGroup;
// if they are empty, it will set the default values in the following:
//
//	minMember: NUM(workers) + 1
//	queue: A "scheduling.volcano.sh/queue-name" annotation value.
//	priorityClass: A value returned from the calcPriorityClassName function.
//	minResources: nil
//
// However, it doesn't pass the ".schedulingPolicy.scheduleTimeoutSeconds" to the podGroup resource.
func (v *VolcanoCtrl) newPodGroup(mpiJob *kubeflow.MPIJob) metav1.Object {
	if mpiJob == nil {
		return nil
	}
	minMember := calculateMinAvailable(mpiJob)
	queueName := mpiJob.Annotations[volcanov1beta1.QueueNameAnnotationKey]
	if schedulingPolicy := mpiJob.Spec.RunPolicy.SchedulingPolicy; schedulingPolicy != nil && len(schedulingPolicy.Queue) != 0 {
		queueName = schedulingPolicy.Queue
	}
	return &volcanov1beta1.PodGroup{
		TypeMeta: metav1.TypeMeta{
			APIVersion: volcanov1beta1.SchemeGroupVersion.String(),
			Kind:       "PodGroup",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      mpiJob.Name,
			Namespace: mpiJob.Namespace,
			OwnerReferences: []metav1.OwnerReference{
				*metav1.NewControllerRef(mpiJob, kubeflow.SchemeGroupVersionKind),
			},
		},
		Spec: volcanov1beta1.PodGroupSpec{
			MinMember:         *minMember,
			Queue:             queueName,
			PriorityClassName: calculatePriorityClassName(mpiJob.Spec.MPIReplicaSpecs, mpiJob.Spec.RunPolicy.SchedulingPolicy),
			MinResources:      v.calculatePGMinResources(minMember, mpiJob),
		},
	}
}

func (v *VolcanoCtrl) getPodGroup(namespace, name string) (metav1.Object, error) {
	return v.PodGroupInformer.Lister().PodGroups(namespace).Get(name)
}

func (v *VolcanoCtrl) createPodGroup(ctx context.Context, pg metav1.Object) (metav1.Object, error) {
	podGroup := pg.(*volcanov1beta1.PodGroup)
	return v.Client.SchedulingV1beta1().PodGroups(pg.GetNamespace()).Create(ctx, podGroup, metav1.CreateOptions{})
}

func (v *VolcanoCtrl) updatePodGroup(ctx context.Context, old, new metav1.Object) (metav1.Object, error) {
	oldPG := old.(*volcanov1beta1.PodGroup)
	newPG := new.(*volcanov1beta1.PodGroup)
	oldPG.Spec = newPG.Spec
	return v.Client.SchedulingV1beta1().PodGroups(oldPG.GetNamespace()).Update(ctx, oldPG, metav1.UpdateOptions{})
}

func (v *VolcanoCtrl) deletePodGroup(ctx context.Context, namespace, name string) error {
	return v.Client.SchedulingV1beta1().PodGroups(namespace).Delete(ctx, name, metav1.DeleteOptions{})
}

func (v *VolcanoCtrl) decoratePodTemplateSpec(pts *corev1.PodTemplateSpec, mpiJobName string) {
	if pts.Spec.SchedulerName != v.schedulerName {
		klog.Warningf("%s scheduler is specified when gang-scheduling is enabled and it will be overwritten", pts.Spec.SchedulerName)
	}
	pts.Spec.SchedulerName = v.schedulerName
	if pts.Annotations == nil {
		pts.Annotations = make(map[string]string)
	}
	// We create the podGroup with the same name as the mpiJob.
	pts.Annotations[volcanov1beta1.KubeGroupNameAnnotationKey] = mpiJobName
}

// calculatePGMinResources calculates minResources for volcano podGroup.
// The minMember is task's total MinAvailable or replicas if task's minAvailable is not set in vcJob.
// PodGroup's MinResources leaves empty now if it is not set. So we calculate the minResources among those first minMember replicas with higher priority.
// ret: https://github.com/volcano-sh/volcano/blob/1933d46bdc4434772518ebb74c4281671ddeffa1/pkg/webhooks/admission/jobs/mutate/mutate_job.go#L168
// ref: https://github.com/volcano-sh/volcano/blob/1933d46bdc4434772518ebb74c4281671ddeffa1/pkg/controllers/job/job_controller_actions.go#L761
func (v *VolcanoCtrl) calculatePGMinResources(minMember *int32, mpiJob *kubeflow.MPIJob) *corev1.ResourceList {
	if schedPolicy := mpiJob.Spec.RunPolicy.SchedulingPolicy; schedPolicy != nil && schedPolicy.MinResources != nil {
		return schedPolicy.MinResources
	}
	if minMember != nil && *minMember == 0 {
		return nil
	}

	// sort task by priorityClasses
	return calPGMinResource(minMember, mpiJob, v.PriorityClassLister)
}

func (v *VolcanoCtrl) pgSpecsAreEqual(a, b metav1.Object) bool {
	PGa := a.(*volcanov1beta1.PodGroup)
	PGb := b.(*volcanov1beta1.PodGroup)
	return equality.Semantic.DeepEqual(PGa.Spec, PGb.Spec)
}

var _ PodGroupControl = &VolcanoCtrl{}

// SchedulerPluginsCtrl is the implementation fo PodGroupControl with scheduler-plugins.
type SchedulerPluginsCtrl struct {
	Client              schedclientset.Interface
	InformerFactory     schedinformers.SharedInformerFactory
	PodGroupInformer    schedinformer.PodGroupInformer
	PriorityClassLister schedulinglisters.PriorityClassLister
	schedulerName       string
}

func NewSchedulerPluginsCtrl(
	c schedclientset.Interface,
	watchNamespace, schedulerName string,
	pcLister schedulinglisters.PriorityClassLister,
) *SchedulerPluginsCtrl {
	var informerFactoryOpts []schedinformers.SharedInformerOption
	if watchNamespace != metav1.NamespaceAll {
		informerFactoryOpts = append(informerFactoryOpts, schedinformers.WithNamespace(watchNamespace))
	}
	pgInformerFactory := schedinformers.NewSharedInformerFactoryWithOptions(c, 0, informerFactoryOpts...)
	return &SchedulerPluginsCtrl{
		Client:              c,
		InformerFactory:     pgInformerFactory,
		PodGroupInformer:    pgInformerFactory.Scheduling().V1alpha1().PodGroups(),
		PriorityClassLister: pcLister,
		schedulerName:       schedulerName,
	}
}

func (s *SchedulerPluginsCtrl) PodGroupSharedIndexInformer() cache.SharedIndexInformer {
	return s.PodGroupInformer.Informer()
}

func (s *SchedulerPluginsCtrl) StartInformerFactory(stopCh <-chan struct{}) {
	go s.InformerFactory.Start(stopCh)
}

// newPodGroup will generate a new PodGroup for an MPIJob resource.
// If the parameters set in the schedulingPolicy aren't empty, it will pass them to a new PodGroup;
// if they are empty, it will set the default values in the following:
//
//	minMember: NUM(workers) + 1
//	scheduleTimeoutSeconds: 0
//	minResources: Follows the result of calculatePGMinResources.
//
// However, it doesn't pass the ".schedulingPolicy.priorityClass" and "schedulingPolicy.queue" to the podGroup resource.
func (s *SchedulerPluginsCtrl) newPodGroup(mpiJob *kubeflow.MPIJob) metav1.Object {
	if mpiJob == nil {
		return nil
	}
	scheduleTimeoutSec := ptr.To[int32](0)
	if schedPolicy := mpiJob.Spec.RunPolicy.SchedulingPolicy; schedPolicy != nil && schedPolicy.ScheduleTimeoutSeconds != nil {
		scheduleTimeoutSec = schedPolicy.ScheduleTimeoutSeconds
	}
	minMember := calculateMinAvailable(mpiJob)
	var minResources corev1.ResourceList
	if origin := s.calculatePGMinResources(minMember, mpiJob); origin != nil {
		minResources = *origin
	}
	return &schedv1alpha1.PodGroup{
		TypeMeta: metav1.TypeMeta{
			APIVersion: schedv1alpha1.SchemeGroupVersion.String(),
			Kind:       "PodGroup",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      mpiJob.Name,
			Namespace: mpiJob.Namespace,
			OwnerReferences: []metav1.OwnerReference{
				*metav1.NewControllerRef(mpiJob, kubeflow.SchemeGroupVersionKind),
			},
		},
		Spec: schedv1alpha1.PodGroupSpec{
			MinMember:              *minMember,
			ScheduleTimeoutSeconds: scheduleTimeoutSec,
			MinResources:           minResources,
		},
	}
}

func (s *SchedulerPluginsCtrl) getPodGroup(namespace, name string) (metav1.Object, error) {
	return s.PodGroupInformer.Lister().PodGroups(namespace).Get(name)
}

func (s *SchedulerPluginsCtrl) createPodGroup(ctx context.Context, pg metav1.Object) (metav1.Object, error) {
	podGroup := pg.(*schedv1alpha1.PodGroup)
	return s.Client.SchedulingV1alpha1().PodGroups(pg.GetNamespace()).Create(ctx, podGroup, metav1.CreateOptions{})
}

func (s *SchedulerPluginsCtrl) updatePodGroup(ctx context.Context, old, new metav1.Object) (metav1.Object, error) {
	oldPG := old.(*schedv1alpha1.PodGroup)
	newPG := new.(*schedv1alpha1.PodGroup)
	oldPG.Spec = newPG.Spec
	return s.Client.SchedulingV1alpha1().PodGroups(oldPG.GetNamespace()).Update(ctx, oldPG, metav1.UpdateOptions{})
}

func (s *SchedulerPluginsCtrl) deletePodGroup(ctx context.Context, namespace, name string) error {
	return s.Client.SchedulingV1alpha1().PodGroups(namespace).Delete(ctx, name, metav1.DeleteOptions{})
}

func (s *SchedulerPluginsCtrl) decoratePodTemplateSpec(pts *corev1.PodTemplateSpec, mpiJobName string) {
	if pts.Spec.SchedulerName != s.schedulerName {
		klog.Warningf("%s scheduler is specified when gang-scheduling is enabled and it will be overwritten", pts.Spec.SchedulerName)
	}
	pts.Spec.SchedulerName = s.schedulerName
	if pts.Labels == nil {
		pts.Labels = make(map[string]string)
	}
	pts.Labels[schedv1alpha1.PodGroupLabel] = mpiJobName
}

// calculatePGMinResources will calculate minResources for podGroup.
// It calculates the sum of resources defined in all containers and will return the result.
// If the number of replicas (.spec.mpiReplicaSpecs[Launcher].replicas + .spec.mpiReplicaSpecs[Worker].replicas)
// is more of minMember, it reorders replicas according to each priorityClass setting in `podSpec.priorityClassName`
// and then resources with a priority less than minMember will not be added to minResources.
// Note that it doesn't account for the priorityClass specified in podSpec.priorityClassName
// if the priorityClass doesn't exist in the cluster when it reorders replicas.
//
// By adding appropriate required resources to a podGroup resource,
// the coscheduling plugin can filter out the pods that belong to the podGroup in PreFilter
// if the cluster doesn't have enough resources.
// ref: https://github.com/kubernetes-sigs/scheduler-plugins/blob/93d7c92851c4a17f110907f3b5be873176628441/pkg/coscheduling/core/core.go#L159-L182
func (s *SchedulerPluginsCtrl) calculatePGMinResources(minMember *int32, mpiJob *kubeflow.MPIJob) *corev1.ResourceList {
	if schedPolicy := mpiJob.Spec.RunPolicy.SchedulingPolicy; schedPolicy != nil && schedPolicy.MinResources != nil {
		return schedPolicy.MinResources
	}
	if minMember != nil && *minMember == 0 {
		return nil
	}

	return calPGMinResource(minMember, mpiJob, s.PriorityClassLister)
}

func (s *SchedulerPluginsCtrl) pgSpecsAreEqual(a, b metav1.Object) bool {
	PGa := a.(*schedv1alpha1.PodGroup)
	PGb := b.(*schedv1alpha1.PodGroup)
	return equality.Semantic.DeepEqual(PGa.Spec, PGb.Spec)
}

var _ PodGroupControl = &SchedulerPluginsCtrl{}

// calPGMinResource returns the minimum resource for mpiJob with minMembers
func calPGMinResource(minMember *int32, mpiJob *kubeflow.MPIJob, pcLister schedulinglisters.PriorityClassLister) *corev1.ResourceList {
	var order replicasOrder
	for rt, replica := range mpiJob.Spec.MPIReplicaSpecs {
		rp := replicaPriority{
			priority:    0,
			replicaType: rt,
			ReplicaSpec: *replica,
		}

		pcName := replica.Template.Spec.PriorityClassName
		if len(pcName) != 0 && pcLister != nil {
			if priorityClass, err := pcLister.Get(pcName); err != nil {
				klog.Warningf("Ignore replica %q priority class %q: %v", rt, pcName, err)
			} else {
				rp.priority = priorityClass.Value
			}
		}
		order = append(order, rp)
	}

	sort.Sort(sort.Reverse(order))
	// Launcher + Worker > minMember
	replicas := *order[0].Replicas
	if len(order) > 1 {
		// When using runLauncherAsWorker, there may be no worker.
		replicas += *order[1].Replicas
	}
	if minMember != nil && replicas > *minMember {
		// If the launcher and workers have the same priority, it treats workers as a lower priority.
		if order[0].priority == order[1].priority {
			wIndex := order.getWorkerIndex()
			if wIndex == -1 {
				klog.Warningf("Couldn't find the worker replicas")
				return nil
			}
			order[wIndex].Replicas = ptr.To(*minMember - 1)
		} else {
			order[1].Replicas = ptr.To(*minMember - 1)
		}
	}

	minResources := corev1.ResourceList{}
	for _, rp := range order {
		if rp.Replicas == nil {
			continue
		}
		for _, c := range rp.Template.Spec.Containers {
			addResources(minResources, c.Resources, int64(*rp.Replicas))
		}
	}
	return &minResources
}

// calculateMinAvailable calculates minAvailable for the PodGroup.
// If the schedulingPolicy.minAvailable is nil, it returns returns `NUM(workers) + 1`; otherwise returns `schedulingPolicy.minAvailable`.
func calculateMinAvailable(mpiJob *kubeflow.MPIJob) *int32 {
	if schedulingPolicy := mpiJob.Spec.RunPolicy.SchedulingPolicy; schedulingPolicy != nil && schedulingPolicy.MinAvailable != nil {
		return schedulingPolicy.MinAvailable
	}
	return ptr.To(workerReplicas(mpiJob) + 1)
}

// calculatePriorityClassName calculates the priorityClass name needed for podGroup according to the following priorities:
//  1. .spec.runPolicy.schedulingPolicy.priorityClass
//  2. .spec.mpiReplicaSecs[Launcher].template.spec.priorityClassName
//  3. .spec.mpiReplicaSecs[Worker].template.spec.priorityClassName
func calculatePriorityClassName(
	replicas map[kubeflow.MPIReplicaType]*kubeflow.ReplicaSpec,
	schedulingPolicy *kubeflow.SchedulingPolicy,
) string {
	if schedulingPolicy != nil && len(schedulingPolicy.PriorityClass) != 0 {
		return schedulingPolicy.PriorityClass
	} else if l := replicas[kubeflow.MPIReplicaTypeLauncher]; l != nil && len(l.Template.Spec.PriorityClassName) != 0 {
		return l.Template.Spec.PriorityClassName
	} else if w := replicas[kubeflow.MPIReplicaTypeWorker]; w != nil && len(w.Template.Spec.PriorityClassName) != 0 {
		return w.Template.Spec.PriorityClassName
	} else {
		return ""
	}
}

// addResources adds resources to minResources.
// If resources don't have requests, it defaults limit if that is explicitly specified.
func addResources(minResources corev1.ResourceList, resources corev1.ResourceRequirements, replicas int64) {
	if minResources == nil || cmp.Equal(resources, corev1.ResourceRequirements{}) {
		return
	}

	merged := corev1.ResourceList{}
	for name, req := range resources.Requests {
		merged[name] = req
	}
	for name, lim := range resources.Limits {
		if _, ok := merged[name]; !ok {
			merged[name] = lim
		}
	}
	for name, quantity := range merged {
		quantity.Mul(replicas)
		if q, ok := minResources[name]; !ok {
			minResources[name] = quantity.DeepCopy()
		} else {
			q.Add(quantity)
			minResources[name] = q
		}
	}
}

type replicaPriority struct {
	priority    int32
	replicaType kubeflow.MPIReplicaType

	kubeflow.ReplicaSpec
}

type replicasOrder []replicaPriority

func (p replicasOrder) Len() int {
	return len(p)
}

func (p replicasOrder) Less(i, j int) bool {
	return p[i].priority < p[j].priority
}

func (p replicasOrder) Swap(i, j int) {
	p[i], p[j] = p[j], p[i]
}

// getWorkerIndex will return an index holding the replicaSpec for the worker.
// If the worker doesn't exist, it returns -1.
func (p replicasOrder) getWorkerIndex() int {
	for i := range p {
		if p[i].replicaType == kubeflow.MPIReplicaTypeWorker {
			return i
		}
	}
	return -1
}
