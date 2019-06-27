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
	"fmt"
	"reflect"
	"testing"
	"time"

	appsv1 "k8s.io/api/apps/v1"
	batchv1 "k8s.io/api/batch/v1"
	"k8s.io/api/core/v1"
	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/util/diff"
	kubeinformers "k8s.io/client-go/informers"
	k8sfake "k8s.io/client-go/kubernetes/fake"
	core "k8s.io/client-go/testing"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/tools/record"

	podgroupv1alpha1 "github.com/kubernetes-sigs/kube-batch/pkg/apis/scheduling/v1alpha1"
	kubebatchfake "github.com/kubernetes-sigs/kube-batch/pkg/client/clientset/versioned/fake"
	kubebatchinformers "github.com/kubernetes-sigs/kube-batch/pkg/client/informers/externalversions"

	kubeflow "github.com/kubeflow/mpi-operator/pkg/apis/kubeflow/v1alpha2"
	"github.com/kubeflow/mpi-operator/pkg/client/clientset/versioned/fake"
	informers "github.com/kubeflow/mpi-operator/pkg/client/informers/externalversions"
	"k8s.io/apimachinery/pkg/api/resource"
)

var (
	alwaysReady        = func() bool { return true }
	noResyncPeriodFunc = func() time.Duration { return 0 }
)

type fixture struct {
	t *testing.T

	client          *fake.Clientset
	kubeClient      *k8sfake.Clientset
	kubebatchClient *kubebatchfake.Clientset

	// Objects to put in the store.
	configMapLister      []*corev1.ConfigMap
	serviceAccountLister []*corev1.ServiceAccount
	roleLister           []*rbacv1.Role
	roleBindingLister    []*rbacv1.RoleBinding
	statefulSetLister    []*appsv1.StatefulSet
	jobLister            []*batchv1.Job
	podGroupLister       []*podgroupv1alpha1.PodGroup
	mpiJobLister         []*kubeflow.MPIJob

	// Actions expected to happen on the client.
	kubeActions []core.Action
	actions     []core.Action

	// Objects from here are pre-loaded into NewSimpleFake.
	kubeObjects []runtime.Object
	objects     []runtime.Object
}

func newFixture(t *testing.T) *fixture {
	f := &fixture{}
	f.t = t
	f.objects = []runtime.Object{}
	f.kubeObjects = []runtime.Object{}
	return f
}

func newMPIJobCommon(name string, startTime, completionTime *metav1.Time) *kubeflow.MPIJob {
	cleanPodPolicyAll := kubeflow.CleanPodPolicyAll
	mpiJob := &kubeflow.MPIJob{
		TypeMeta: metav1.TypeMeta{APIVersion: kubeflow.SchemeGroupVersion.String()},
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: metav1.NamespaceDefault,
		},
		Spec: kubeflow.MPIJobSpec{
			CleanPodPolicy: &cleanPodPolicyAll,
			MPIReplicaSpecs: map[kubeflow.MPIReplicaType]*kubeflow.ReplicaSpec{
				kubeflow.MPIReplicaTypeWorker: {
					Template: corev1.PodTemplateSpec{
						Spec: corev1.PodSpec{
							Containers: []corev1.Container{
								{
									Name:  "foo",
									Image: "bar",
								},
							},
						},
					},
				},
				kubeflow.MPIReplicaTypeLauncher: {
					Template: corev1.PodTemplateSpec{
						Spec: corev1.PodSpec{
							Containers: []corev1.Container{
								{
									Name:  "foo",
									Image: "bar",
								},
							},
						},
					},
				},
			},
		},
		Status: kubeflow.JobStatus{},
	}

	if startTime != nil {
		mpiJob.Status.StartTime = startTime
	}
	if completionTime != nil {
		mpiJob.Status.CompletionTime = completionTime
	}

	return mpiJob
}

func newMPIJob(name string, replicas *int32, pusPerReplica int64, resourceName string, startTime, completionTime *metav1.Time) *kubeflow.MPIJob {
	mpiJob := newMPIJobCommon(name, startTime, completionTime)

	mpiJob.Spec.MPIReplicaSpecs[kubeflow.MPIReplicaTypeWorker].Replicas = replicas

	workerContainers := mpiJob.Spec.MPIReplicaSpecs[kubeflow.MPIReplicaTypeWorker].Template.Spec.Containers
	for i := range workerContainers {
		container := &workerContainers[i]
		container.Resources = corev1.ResourceRequirements{
			Limits: corev1.ResourceList{
				corev1.ResourceName(resourceName): *resource.NewQuantity(pusPerReplica, resource.DecimalExponent),
			},
		}
	}

	return mpiJob
}

func (f *fixture) newController(enableGangScheduling bool) (*MPIJobController, informers.SharedInformerFactory, kubeinformers.SharedInformerFactory) {
	f.client = fake.NewSimpleClientset(f.objects...)
	f.kubeClient = k8sfake.NewSimpleClientset(f.kubeObjects...)

	i := informers.NewSharedInformerFactory(f.client, noResyncPeriodFunc())
	k8sI := kubeinformers.NewSharedInformerFactory(f.kubeClient, noResyncPeriodFunc())

	kubebatchInformerFactory := kubebatchinformers.NewSharedInformerFactory(f.kubebatchClient, 0)
	podgroupsInformer := kubebatchInformerFactory.Scheduling().V1alpha1().PodGroups()

	c := NewMPIJobController(
		f.kubeClient,
		f.client,
		f.kubebatchClient,
		k8sI.Core().V1().ConfigMaps(),
		k8sI.Core().V1().ServiceAccounts(),
		k8sI.Rbac().V1().Roles(),
		k8sI.Rbac().V1().RoleBindings(),
		k8sI.Apps().V1().StatefulSets(),
		k8sI.Batch().V1().Jobs(),
		podgroupsInformer,
		i.Kubeflow().V1alpha2().MPIJobs(),
		"kubectl-delivery",
		enableGangScheduling,
	)

	c.configMapSynced = alwaysReady
	c.serviceAccountSynced = alwaysReady
	c.roleSynced = alwaysReady
	c.roleBindingSynced = alwaysReady
	c.statefulSetSynced = alwaysReady
	c.jobSynced = alwaysReady
	c.podgroupsSynced = alwaysReady
	c.mpiJobSynced = alwaysReady
	c.recorder = &record.FakeRecorder{}

	for _, configMap := range f.configMapLister {
		k8sI.Core().V1().ConfigMaps().Informer().GetIndexer().Add(configMap)
	}

	for _, serviceAccount := range f.serviceAccountLister {
		k8sI.Core().V1().ServiceAccounts().Informer().GetIndexer().Add(serviceAccount)
	}

	for _, role := range f.roleLister {
		k8sI.Rbac().V1().Roles().Informer().GetIndexer().Add(role)
	}

	for _, roleBinding := range f.roleBindingLister {
		k8sI.Rbac().V1().RoleBindings().Informer().GetIndexer().Add(roleBinding)
	}

	for _, statefulSet := range f.statefulSetLister {
		k8sI.Apps().V1().StatefulSets().Informer().GetIndexer().Add(statefulSet)
	}

	for _, job := range f.jobLister {
		k8sI.Batch().V1().Jobs().Informer().GetIndexer().Add(job)
	}

	for _, podGroup := range f.podGroupLister {
		podgroupsInformer.Informer().GetIndexer().Add(podGroup)
	}

	for _, mpiJob := range f.mpiJobLister {
		i.Kubeflow().V1alpha2().MPIJobs().Informer().GetIndexer().Add(mpiJob)
	}

	return c, i, k8sI
}

func (f *fixture) run(mpiJobName string) {
	f.runController(mpiJobName, true, false, false)
}

func (f *fixture) runExpectError(mpiJobName string) {
	f.runController(mpiJobName, true, true, false)
}

func (f *fixture) runController(mpiJobName string, startInformers bool, expectError, enableGangScheduling bool) {
	c, i, k8sI := f.newController(enableGangScheduling)
	if startInformers {
		stopCh := make(chan struct{})
		defer close(stopCh)
		i.Start(stopCh)
		k8sI.Start(stopCh)
	}

	err := c.syncHandler(mpiJobName)
	if !expectError && err != nil {
		f.t.Errorf("error syncing mpi job: %v", err)
	} else if expectError && err == nil {
		f.t.Error("expected error syncing mpi job, got nil")
	}

	actions := filterInformerActions(f.client.Actions())
	for i, action := range actions {
		if len(f.actions) < i+1 {
			f.t.Errorf("%d unexpected actions: %+v", len(actions)-len(f.actions), actions[i:])
			break
		}

		expectedAction := f.actions[i]
		checkAction(expectedAction, action, f.t)
	}

	if len(f.actions) > len(actions) {
		f.t.Errorf("%d additional expected actions:%+v", len(f.actions)-len(actions), f.actions[len(actions):])
	}

	k8sActions := filterInformerActions(f.kubeClient.Actions())
	for i, action := range k8sActions {
		if len(f.kubeActions) < i+1 {
			f.t.Errorf("%d unexpected actions: %+v", len(k8sActions)-len(f.kubeActions), k8sActions[i:])
			break
		}

		expectedAction := f.kubeActions[i]
		checkAction(expectedAction, action, f.t)
	}

	if len(f.kubeActions) > len(k8sActions) {
		f.t.Errorf("%d additional expected actions:%+v", len(f.kubeActions)-len(k8sActions), f.kubeActions[len(k8sActions):])
	}
}

// checkAction verifies that expected and actual actions are equal and both have
// same attached resources
func checkAction(expected, actual core.Action, t *testing.T) {
	if !(expected.Matches(actual.GetVerb(), actual.GetResource().Resource) && actual.GetSubresource() == expected.GetSubresource()) {
		t.Errorf("Expected\n\t%#v\ngot\n\t%#v", expected, actual)
		return
	}

	if reflect.TypeOf(actual) != reflect.TypeOf(expected) {
		t.Errorf("Action has wrong type. Expected: %t. Got: %t", expected, actual)
		return
	}

	switch a := actual.(type) {
	case core.UpdateAction:
		e, _ := expected.(core.UpdateAction)
		expObject := e.GetObject()
		object := a.GetObject()

		expMPIJob, ok1 := expObject.(*kubeflow.MPIJob)
		gotMPIJob, ok2 := object.(*kubeflow.MPIJob)
		if ok1 && ok2 {
			clearConditionTime(expMPIJob)
			clearConditionTime(gotMPIJob)

			if !reflect.DeepEqual(expMPIJob, gotMPIJob) {
				t.Errorf("Action %s %s has wrong object\nDiff:\n %s",
					a.GetVerb(), a.GetResource().Resource, diff.ObjectGoPrintDiff(expObject, object))
			}
			return
		}

		if !reflect.DeepEqual(expObject, object) {
			t.Errorf("Action %s %s has wrong object\nDiff:\n %s",
				a.GetVerb(), a.GetResource().Resource, diff.ObjectGoPrintDiff(expObject, object))
		}
	case core.CreateAction:
		e, _ := expected.(core.CreateAction)
		expObject := e.GetObject()
		object := a.GetObject()

		if !reflect.DeepEqual(expObject, object) {
			t.Errorf("Action %s %s has wrong object\nDiff:\n %s",
				a.GetVerb(), a.GetResource().Resource, diff.ObjectGoPrintDiff(expObject, object))
		}
	case core.PatchAction:
		e, _ := expected.(core.PatchAction)
		expPatch := e.GetPatch()
		patch := a.GetPatch()

		if !reflect.DeepEqual(expPatch, expPatch) {
			t.Errorf("Action %s %s has wrong patch\nDiff:\n %s",
				a.GetVerb(), a.GetResource().Resource, diff.ObjectGoPrintDiff(expPatch, patch))
		}
	}
}

// filterInformerActions filters list and watch actions for testing resources.
// Since list and watch don't change resource state we can filter it to lower
// nose level in our tests.
func filterInformerActions(actions []core.Action) []core.Action {
	var ret []core.Action
	for _, action := range actions {
		if len(action.GetNamespace()) == 0 &&
			(action.Matches("list", "configmaps") ||
				action.Matches("watch", "configmaps") ||
				action.Matches("list", "serviceaccounts") ||
				action.Matches("watch", "serviceaccounts") ||
				action.Matches("list", "roles") ||
				action.Matches("watch", "roles") ||
				action.Matches("list", "rolebindings") ||
				action.Matches("watch", "rolebindings") ||
				action.Matches("list", "statefulsets") ||
				action.Matches("watch", "statefulsets") ||
				action.Matches("list", "pods") ||
				action.Matches("watch", "pods") ||
				action.Matches("list", "jobs") ||
				action.Matches("watch", "jobs") ||
				action.Matches("list", "podgroups") ||
				action.Matches("watch", "podgroups") ||
				action.Matches("list", "mpijobs") ||
				action.Matches("watch", "mpijobs")) {
			continue
		}
		ret = append(ret, action)
	}

	return ret
}

func (f *fixture) expectCreateConfigMapAction(d *corev1.ConfigMap) {
	f.kubeActions = append(f.kubeActions, core.NewCreateAction(schema.GroupVersionResource{Resource: "configmaps"}, d.Namespace, d))
}

func (f *fixture) expectUpdateConfigMapAction(d *corev1.ConfigMap) {
	f.kubeActions = append(f.kubeActions, core.NewUpdateAction(schema.GroupVersionResource{Resource: "configmaps"}, d.Namespace, d))
}

func (f *fixture) expectCreateServiceAccountAction(d *corev1.ServiceAccount) {
	f.kubeActions = append(f.kubeActions, core.NewCreateAction(schema.GroupVersionResource{Resource: "serviceaccounts"}, d.Namespace, d))
}

func (f *fixture) expectUpdateServiceAccountAction(d *corev1.ServiceAccount) {
	f.kubeActions = append(f.kubeActions, core.NewUpdateAction(schema.GroupVersionResource{Resource: "serviceaccounts"}, d.Namespace, d))
}

func (f *fixture) expectCreateRoleAction(d *rbacv1.Role) {
	f.kubeActions = append(f.kubeActions, core.NewCreateAction(schema.GroupVersionResource{Resource: "roles"}, d.Namespace, d))
}

func (f *fixture) expectUpdateRoleAction(d *rbacv1.Role) {
	f.kubeActions = append(f.kubeActions, core.NewUpdateAction(schema.GroupVersionResource{Resource: "roles"}, d.Namespace, d))
}

func (f *fixture) expectCreateRoleBindingAction(d *rbacv1.RoleBinding) {
	f.kubeActions = append(f.kubeActions, core.NewCreateAction(schema.GroupVersionResource{Resource: "rolebindings"}, d.Namespace, d))
}

func (f *fixture) expectUpdateRoleBindingAction(d *rbacv1.RoleBinding) {
	f.kubeActions = append(f.kubeActions, core.NewUpdateAction(schema.GroupVersionResource{Resource: "rolebindings"}, d.Namespace, d))
}

func (f *fixture) expectCreateStatefulSetAction(d *appsv1.StatefulSet) {
	f.kubeActions = append(f.kubeActions, core.NewCreateAction(schema.GroupVersionResource{Resource: "statefulsets"}, d.Namespace, d))
}

func (f *fixture) expectUpdateStatefulSetAction(d *appsv1.StatefulSet) {
	f.kubeActions = append(f.kubeActions, core.NewUpdateAction(schema.GroupVersionResource{Resource: "statefulsets"}, d.Namespace, d))
}

func (f *fixture) expectCreateJobAction(d *batchv1.Job) {
	f.kubeActions = append(f.kubeActions, core.NewCreateAction(schema.GroupVersionResource{Resource: "jobs"}, d.Namespace, d))
}

func (f *fixture) expectUpdateJobAction(d *batchv1.Job) {
	f.kubeActions = append(f.kubeActions, core.NewUpdateAction(schema.GroupVersionResource{Resource: "jobs"}, d.Namespace, d))
}

func (f *fixture) expectUpdateMPIJobStatusAction(mpiJob *kubeflow.MPIJob) {
	action := core.NewUpdateAction(schema.GroupVersionResource{Resource: "mpijobs"}, mpiJob.Namespace, mpiJob)
	action.Subresource = "status"
	f.actions = append(f.actions, action)
}

func (f *fixture) setUpMPIJob(mpiJob *kubeflow.MPIJob) {
	f.mpiJobLister = append(f.mpiJobLister, mpiJob)
	f.objects = append(f.objects, mpiJob)
}

func (f *fixture) setUpLauncher(launcher *batchv1.Job) {
	f.jobLister = append(f.jobLister, launcher)
	f.kubeObjects = append(f.kubeObjects, launcher)
}

func (f *fixture) setUpWorker(worker *appsv1.StatefulSet) {
	f.statefulSetLister = append(f.statefulSetLister, worker)
	f.kubeObjects = append(f.kubeObjects, worker)
}

func (f *fixture) setUpConfigMap(configMap *corev1.ConfigMap) {
	f.configMapLister = append(f.configMapLister, configMap)
	f.kubeObjects = append(f.kubeObjects, configMap)
}

func (f *fixture) setUpServiceAccount(serviceAccount *corev1.ServiceAccount) {
	f.serviceAccountLister = append(f.serviceAccountLister, serviceAccount)
	f.kubeObjects = append(f.kubeObjects, serviceAccount)
}

func (f *fixture) setUpRole(role *rbacv1.Role) {
	f.roleLister = append(f.roleLister, role)
	f.kubeObjects = append(f.kubeObjects, role)
}

func (f *fixture) setUpRoleBinding(roleBinding *rbacv1.RoleBinding) {
	f.roleBindingLister = append(f.roleBindingLister, roleBinding)
	f.kubeObjects = append(f.kubeObjects, roleBinding)
}

func (f *fixture) setUpRbac(mpiJob *kubeflow.MPIJob, workerReplicas int32) {
	serviceAccount := newLauncherServiceAccount(mpiJob)
	f.setUpServiceAccount(serviceAccount)

	role := newLauncherRole(mpiJob, workerReplicas)
	f.setUpRole(role)

	roleBinding := newLauncherRoleBinding(mpiJob)
	f.setUpRoleBinding(roleBinding)
}

func setUpMPIJobTimestamp(mpiJob *kubeflow.MPIJob, startTime, completionTime *metav1.Time) {
	if startTime != nil {
		mpiJob.Status.StartTime = startTime
	}

	if completionTime != nil {
		mpiJob.Status.CompletionTime = completionTime
	}
}

func clearConditionTime(mpiJob *kubeflow.MPIJob) {
	var clearConditions []kubeflow.JobCondition
	for _, condition := range mpiJob.Status.Conditions {
		condition.LastTransitionTime = metav1.Time{}
		condition.LastUpdateTime = metav1.Time{}
		clearConditions = append(clearConditions, condition)
	}
	mpiJob.Status.Conditions = clearConditions
}

func getKey(mpiJob *kubeflow.MPIJob, t *testing.T) string {
	key, err := cache.DeletionHandlingMetaNamespaceKeyFunc(mpiJob)
	if err != nil {
		t.Errorf("Unexpected error getting key for mpi job %v: %v", mpiJob.Name, err)
		return ""
	}
	return key
}

func TestDoNothingWithInvalidKey(t *testing.T) {
	f := newFixture(t)
	f.run("foo/bar/baz")
}

func TestDoNothingWithNonexistentMPIJob(t *testing.T) {
	f := newFixture(t)
	startTime := metav1.Now()
	completionTime := metav1.Now()
	mpiJob := newMPIJob("test", int32Ptr(64), 1, gpuResourceName, &startTime, &completionTime)
	f.run(getKey(mpiJob, t))
}

func TestLauncherNotControlledByUs(t *testing.T) {
	f := newFixture(t)
	startTime := metav1.Now()
	completionTime := metav1.Now()

	mpiJob := newMPIJob("test", int32Ptr(64), 1, gpuResourceName, &startTime, &completionTime)
	f.setUpMPIJob(mpiJob)

	fmjc := newFakeMPIJobController()
	launcher := fmjc.newLauncher(mpiJob, "kubectl-delivery")
	launcher.OwnerReferences = nil
	f.setUpLauncher(launcher)

	f.runExpectError(getKey(mpiJob, t))
}

func TestLauncherSucceeded(t *testing.T) {
	f := newFixture(t)

	startTime := metav1.Now()
	completionTime := metav1.Now()

	mpiJob := newMPIJob("test", int32Ptr(64), 1, gpuResourceName, &startTime, &completionTime)
	f.setUpMPIJob(mpiJob)

	fmjc := newFakeMPIJobController()
	launcher := fmjc.newLauncher(mpiJob, "kubectl-delivery")
	launcher.Status.Succeeded = 1
	launcher.Status.Conditions = append(launcher.Status.Conditions,
		batchv1.JobCondition{
			Type:               batchv1.JobComplete,
			Status:             v1.ConditionTrue,
			LastProbeTime:      metav1.Now(),
			LastTransitionTime: metav1.Now(),
		},
	)
	f.setUpLauncher(launcher)

	mpiJobCopy := mpiJob.DeepCopy()
	mpiJobCopy.Status.ReplicaStatuses = map[kubeflow.ReplicaType]*kubeflow.ReplicaStatus{
		kubeflow.ReplicaType(kubeflow.MPIReplicaTypeLauncher): {
			Active:    0,
			Succeeded: 1,
			Failed:    0,
		},
	}

	setUpMPIJobTimestamp(mpiJobCopy, &startTime, &completionTime)

	msg := fmt.Sprintf("MPIJob %s/%s successfully completed.", mpiJob.Namespace, mpiJob.Name)
	updateMPIJobConditions(mpiJobCopy, kubeflow.JobSucceeded, mpiJobSucceededReason, msg)

	f.expectUpdateMPIJobStatusAction(mpiJobCopy)

	f.run(getKey(mpiJob, t))
}

func TestLauncherFailed(t *testing.T) {
	f := newFixture(t)
	startTime := metav1.Now()
	completionTime := metav1.Now()

	mpiJob := newMPIJob("test", int32Ptr(64), 1, gpuResourceName, &startTime, &completionTime)
	f.setUpMPIJob(mpiJob)

	fmjc := newFakeMPIJobController()
	launcher := fmjc.newLauncher(mpiJob, "kubectl-delivery")
	launcher.Status.Failed = 1
	launcher.Status.Conditions = append(launcher.Status.Conditions,
		batchv1.JobCondition{
			Type:               batchv1.JobFailed,
			Status:             v1.ConditionTrue,
			LastProbeTime:      metav1.Now(),
			LastTransitionTime: metav1.Now(),
		},
	)
	f.setUpLauncher(launcher)

	mpiJobCopy := mpiJob.DeepCopy()
	mpiJobCopy.Status.ReplicaStatuses = map[kubeflow.ReplicaType]*kubeflow.ReplicaStatus{
		kubeflow.ReplicaType(kubeflow.MPIReplicaTypeLauncher): {
			Active:    0,
			Succeeded: 0,
			Failed:    1,
		},
	}
	setUpMPIJobTimestamp(mpiJobCopy, &startTime, &completionTime)

	msg := fmt.Sprintf("MPIJob %s/%s has failed", mpiJob.Namespace, mpiJob.Name)
	updateMPIJobConditions(mpiJobCopy, kubeflow.JobFailed, mpiJobFailedReason, msg)

	f.expectUpdateMPIJobStatusAction(mpiJobCopy)

	f.run(getKey(mpiJob, t))
}

func TestLauncherDoesNotExist(t *testing.T) {
	f := newFixture(t)
	startTime := metav1.Now()
	completionTime := metav1.Now()

	mpiJob := newMPIJob("test", int32Ptr(4), 4, gpuResourceName, &startTime, &completionTime)
	f.setUpMPIJob(mpiJob)

	expConfigMap := newConfigMap(mpiJob, 4)
	f.expectCreateConfigMapAction(expConfigMap)

	expServiceAccount := newLauncherServiceAccount(mpiJob)
	f.expectCreateServiceAccountAction(expServiceAccount)

	expRole := newLauncherRole(mpiJob, 4)
	f.expectCreateRoleAction(expRole)

	expRoleBinding := newLauncherRoleBinding(mpiJob)
	f.expectCreateRoleBindingAction(expRoleBinding)

	expWorker := newWorker(mpiJob, 4)
	f.expectCreateStatefulSetAction(expWorker)

	mpiJobCopy := mpiJob.DeepCopy()
	mpiJobCopy.Status.ReplicaStatuses = map[kubeflow.ReplicaType]*kubeflow.ReplicaStatus{
		kubeflow.ReplicaType(kubeflow.MPIReplicaTypeWorker): {
			Active:    0,
			Succeeded: 0,
			Failed:    0,
		},
	}
	setUpMPIJobTimestamp(mpiJobCopy, &startTime, &completionTime)
	f.expectUpdateMPIJobStatusAction(mpiJobCopy)

	f.run(getKey(mpiJob, t))
}

func TestConfigMapNotControlledByUs(t *testing.T) {
	f := newFixture(t)
	startTime := metav1.Now()
	completionTime := metav1.Now()

	mpiJob := newMPIJob("test", int32Ptr(64), 1, gpuResourceName, &startTime, &completionTime)
	f.setUpMPIJob(mpiJob)

	configMap := newConfigMap(mpiJob, 8)
	configMap.OwnerReferences = nil
	f.setUpConfigMap(configMap)

	f.runExpectError(getKey(mpiJob, t))
}

func TestServiceAccountNotControlledByUs(t *testing.T) {
	f := newFixture(t)
	startTime := metav1.Now()
	completionTime := metav1.Now()

	mpiJob := newMPIJob("test", int32Ptr(64), 1, gpuResourceName, &startTime, &completionTime)
	f.setUpMPIJob(mpiJob)

	f.setUpConfigMap(newConfigMap(mpiJob, 8))

	serviceAccount := newLauncherServiceAccount(mpiJob)
	serviceAccount.OwnerReferences = nil
	f.setUpServiceAccount(serviceAccount)

	f.runExpectError(getKey(mpiJob, t))
}

func TestRoleNotControlledByUs(t *testing.T) {
	f := newFixture(t)
	startTime := metav1.Now()
	completionTime := metav1.Now()

	mpiJob := newMPIJob("test", int32Ptr(64), 1, gpuResourceName, &startTime, &completionTime)
	f.setUpMPIJob(mpiJob)

	f.setUpConfigMap(newConfigMap(mpiJob, 8))
	f.setUpServiceAccount(newLauncherServiceAccount(mpiJob))

	role := newLauncherRole(mpiJob, 8)
	role.OwnerReferences = nil
	f.setUpRole(role)

	f.runExpectError(getKey(mpiJob, t))
}

func TestRoleBindingNotControlledByUs(t *testing.T) {
	f := newFixture(t)
	startTime := metav1.Now()
	completionTime := metav1.Now()

	mpiJob := newMPIJob("test", int32Ptr(64), 1, gpuResourceName, &startTime, &completionTime)
	f.setUpMPIJob(mpiJob)

	f.setUpConfigMap(newConfigMap(mpiJob, 8))
	f.setUpServiceAccount(newLauncherServiceAccount(mpiJob))
	f.setUpRole(newLauncherRole(mpiJob, 8))

	roleBinding := newLauncherRoleBinding(mpiJob)
	roleBinding.OwnerReferences = nil
	f.setUpRoleBinding(roleBinding)

	f.runExpectError(getKey(mpiJob, t))
}

func TestShutdownWorker(t *testing.T) {
	f := newFixture(t)
	startTime := metav1.Now()
	completionTime := metav1.Now()

	mpiJob := newMPIJob("test", int32Ptr(64), 1, gpuResourceName, &startTime, &completionTime)
	msg := fmt.Sprintf("MPIJob %s/%s successfully completed.", mpiJob.Namespace, mpiJob.Name)
	updateMPIJobConditions(mpiJob, kubeflow.JobSucceeded, mpiJobSucceededReason, msg)
	f.setUpMPIJob(mpiJob)

	fmjc := newFakeMPIJobController()
	launcher := fmjc.newLauncher(mpiJob, "kubectl-delivery")
	launcher.Status.Succeeded = 1
	launcher.Status.Conditions = append(launcher.Status.Conditions,
		batchv1.JobCondition{
			Type:               batchv1.JobComplete,
			Status:             v1.ConditionTrue,
			LastProbeTime:      metav1.Now(),
			LastTransitionTime: metav1.Now(),
		},
	)
	f.setUpLauncher(launcher)

	worker := newWorker(mpiJob, 8)
	f.setUpWorker(worker)

	expWorker := newWorker(mpiJob, 0)
	f.expectUpdateStatefulSetAction(expWorker)

	mpiJobCopy := mpiJob.DeepCopy()
	mpiJobCopy.Status.ReplicaStatuses = map[kubeflow.ReplicaType]*kubeflow.ReplicaStatus{
		kubeflow.ReplicaType(kubeflow.MPIReplicaTypeWorker): {
			Active:    0,
			Succeeded: 0,
			Failed:    0,
		},
	}
	setUpMPIJobTimestamp(mpiJobCopy, &startTime, &completionTime)
	f.expectUpdateMPIJobStatusAction(mpiJobCopy)

	f.run(getKey(mpiJob, t))
}

func TestWorkerNotControlledByUs(t *testing.T) {
	f := newFixture(t)
	startTime := metav1.Now()
	completionTime := metav1.Now()

	mpiJob := newMPIJob("test", int32Ptr(64), 1, gpuResourceName, &startTime, &completionTime)
	f.setUpMPIJob(mpiJob)

	f.setUpConfigMap(newConfigMap(mpiJob, 8))
	f.setUpRbac(mpiJob, 8)

	worker := newWorker(mpiJob, 8)
	worker.OwnerReferences = nil
	f.setUpWorker(worker)

	f.runExpectError(getKey(mpiJob, t))
}

func TestLauncherActive(t *testing.T) {
	f := newFixture(t)
	startTime := metav1.Now()
	completionTime := metav1.Now()

	mpiJob := newMPIJob("test", int32Ptr(8), 1, gpuResourceName, &startTime, &completionTime)
	f.setUpMPIJob(mpiJob)

	f.setUpConfigMap(newConfigMap(mpiJob, 1))
	f.setUpRbac(mpiJob, 1)

	fmjc := newFakeMPIJobController()
	launcher := fmjc.newLauncher(mpiJob, "kubectl-delivery")
	launcher.Status.Active = 1
	f.setUpLauncher(launcher)

	worker := newWorker(mpiJob, 8)
	f.setUpWorker(worker)

	mpiJobCopy := mpiJob.DeepCopy()
	mpiJobCopy.Status.ReplicaStatuses = map[kubeflow.ReplicaType]*kubeflow.ReplicaStatus{
		kubeflow.ReplicaType(kubeflow.MPIReplicaTypeLauncher): {
			Active:    1,
			Succeeded: 0,
			Failed:    0,
		},
		kubeflow.ReplicaType(kubeflow.MPIReplicaTypeWorker): {
			Active:    0,
			Succeeded: 0,
			Failed:    0,
		},
	}
	setUpMPIJobTimestamp(mpiJobCopy, &startTime, &completionTime)
	msg := fmt.Sprintf("MPIJob %s/%s is running.", mpiJob.Namespace, mpiJob.Name)
	updateMPIJobConditions(mpiJobCopy, kubeflow.JobRunning, mpiJobRunningReason, msg)
	f.expectUpdateMPIJobStatusAction(mpiJobCopy)

	f.run(getKey(mpiJob, t))
}

func TestLauncherRestarting(t *testing.T) {
	f := newFixture(t)
	startTime := metav1.Now()
	completionTime := metav1.Now()

	mpiJob := newMPIJob("test", int32Ptr(8), 1, gpuResourceName, &startTime, &completionTime)
	f.setUpMPIJob(mpiJob)

	f.setUpConfigMap(newConfigMap(mpiJob, 1))
	f.setUpRbac(mpiJob, 1)

	fmjc := newFakeMPIJobController()
	launcher := fmjc.newLauncher(mpiJob, "kubectl-delivery")
	launcher.Status.Failed = 1
	launcher.Status.Active = 1
	f.setUpLauncher(launcher)

	worker := newWorker(mpiJob, 8)
	f.setUpWorker(worker)

	mpiJobCopy := mpiJob.DeepCopy()
	mpiJobCopy.Status.ReplicaStatuses = map[kubeflow.ReplicaType]*kubeflow.ReplicaStatus{
		kubeflow.ReplicaType(kubeflow.MPIReplicaTypeLauncher): {
			Active:    1,
			Succeeded: 0,
			Failed:    1,
		},
		kubeflow.ReplicaType(kubeflow.MPIReplicaTypeWorker): {
			Active:    0,
			Succeeded: 0,
			Failed:    0,
		},
	}
	setUpMPIJobTimestamp(mpiJobCopy, &startTime, &completionTime)
	msg := fmt.Sprintf("MPIJob %s/%s is restarting.", mpiJob.Namespace, mpiJob.Name)
	updateMPIJobConditions(mpiJobCopy, kubeflow.JobRestarting, mpiJobRestartingReason, msg)
	f.expectUpdateMPIJobStatusAction(mpiJobCopy)

	f.run(getKey(mpiJob, t))
}

func TestWorkerReady(t *testing.T) {
	f := newFixture(t)
	startTime := metav1.Now()
	completionTime := metav1.Now()

	mpiJob := newMPIJob("test", int32Ptr(16), 1, gpuResourceName, &startTime, &completionTime)
	f.setUpMPIJob(mpiJob)

	f.setUpConfigMap(newConfigMap(mpiJob, 16))
	f.setUpRbac(mpiJob, 16)

	worker := newWorker(mpiJob, 16)
	worker.Status.ReadyReplicas = 16
	f.setUpWorker(worker)

	fmjc := newFakeMPIJobController()
	expLauncher := fmjc.newLauncher(mpiJob, "kubectl-delivery")
	f.expectCreateJobAction(expLauncher)

	mpiJobCopy := mpiJob.DeepCopy()
	mpiJobCopy.Status.ReplicaStatuses = map[kubeflow.ReplicaType]*kubeflow.ReplicaStatus{
		kubeflow.ReplicaType(kubeflow.MPIReplicaTypeLauncher): {
			Active:    0,
			Succeeded: 0,
			Failed:    0,
		},
		kubeflow.ReplicaType(kubeflow.MPIReplicaTypeWorker): {
			Active:    16,
			Succeeded: 0,
			Failed:    0,
		},
	}
	setUpMPIJobTimestamp(mpiJobCopy, &startTime, &completionTime)
	f.expectUpdateMPIJobStatusAction(mpiJobCopy)

	f.run(getKey(mpiJob, t))
}

func int32Ptr(i int32) *int32 { return &i }

func newFakeMPIJobController() *MPIJobController {
	return &MPIJobController{
		recorder: &record.FakeRecorder{},
	}
}
