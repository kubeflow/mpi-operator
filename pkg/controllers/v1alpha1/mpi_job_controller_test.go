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
	"reflect"
	"testing"
	"time"

	appsv1 "k8s.io/api/apps/v1"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	policyv1beta1 "k8s.io/api/policy/v1beta1"
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

	kubeflow "github.com/kubeflow/mpi-operator/pkg/apis/kubeflow/v1alpha1"
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

	client     *fake.Clientset
	kubeClient *k8sfake.Clientset

	// Objects to put in the store.
	configMapLister      []*corev1.ConfigMap
	serviceAccountLister []*corev1.ServiceAccount
	roleLister           []*rbacv1.Role
	roleBindingLister    []*rbacv1.RoleBinding
	statefulSetLister    []*appsv1.StatefulSet
	jobLister            []*batchv1.Job
	pdbLister            []*policyv1beta1.PodDisruptionBudget
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
	mpiJob := &kubeflow.MPIJob{
		TypeMeta: metav1.TypeMeta{APIVersion: kubeflow.SchemeGroupVersion.String()},
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: metav1.NamespaceDefault,
		},
		Spec: kubeflow.MPIJobSpec{
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
		Status: kubeflow.MPIJobStatus{},
	}

	if startTime != nil {
		mpiJob.Status.StartTime = startTime
	}

	if completionTime != nil {
		mpiJob.Status.CompletionTime = completionTime
	}

	return mpiJob
}

func newMPIJob(name string, gpus *int32, startTime, completionTime *metav1.Time) *kubeflow.MPIJob {
	mpiJob := newMPIJobCommon(name, startTime, completionTime)
	mpiJob.Spec.GPUs = gpus

	return mpiJob
}

func newMPIJobWithCPUs(name string, cpus *int32, startTime, completionTime *metav1.Time) *kubeflow.MPIJob {
	mpiJob := newMPIJobCommon(name, startTime, completionTime)
	mpiJob.Spec.ProcessingUnits = cpus

	return mpiJob
}

func newMPIJobWithCustomResources(name string, replicas *int32, pusPerReplica int64, processingResourceType string, startTime, completionTime *metav1.Time) *kubeflow.MPIJob {
	mpiJob := newMPIJobCommon(name, startTime, completionTime)
	mpiJob.Spec.Replicas = replicas

	for i := range mpiJob.Spec.Template.Spec.Containers {
		container := &mpiJob.Spec.Template.Spec.Containers[i]
		container.Resources = corev1.ResourceRequirements{
			Limits: corev1.ResourceList{
				convertProcessingResourceType(processingResourceType): *resource.NewQuantity(pusPerReplica, resource.DecimalExponent),
			},
		}
	}

	return mpiJob
}

func (f *fixture) newController(processingResourceType string) (*MPIJobController, informers.SharedInformerFactory, kubeinformers.SharedInformerFactory) {
	f.client = fake.NewSimpleClientset(f.objects...)
	f.kubeClient = k8sfake.NewSimpleClientset(f.kubeObjects...)

	i := informers.NewSharedInformerFactory(f.client, noResyncPeriodFunc())
	k8sI := kubeinformers.NewSharedInformerFactory(f.kubeClient, noResyncPeriodFunc())

	c := NewMPIJobController(
		f.kubeClient,
		f.client,
		k8sI.Core().V1().ConfigMaps(),
		k8sI.Core().V1().ServiceAccounts(),
		k8sI.Rbac().V1().Roles(),
		k8sI.Rbac().V1().RoleBindings(),
		k8sI.Apps().V1().StatefulSets(),
		k8sI.Batch().V1().Jobs(),
		k8sI.Policy().V1beta1().PodDisruptionBudgets(),
		i.Kubeflow().V1alpha1().MPIJobs(),
		8,
		8,
		processingResourceType,
		"kubectl-delivery",
		false,
	)

	c.configMapSynced = alwaysReady
	c.serviceAccountSynced = alwaysReady
	c.roleSynced = alwaysReady
	c.roleBindingSynced = alwaysReady
	c.statefulSetSynced = alwaysReady
	c.jobSynced = alwaysReady
	c.pdbSynced = alwaysReady
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

	for _, pdb := range f.pdbLister {
		k8sI.Policy().V1beta1().PodDisruptionBudgets().Informer().GetIndexer().Add(pdb)
	}

	for _, mpiJob := range f.mpiJobLister {
		i.Kubeflow().V1alpha1().MPIJobs().Informer().GetIndexer().Add(mpiJob)
	}

	return c, i, k8sI
}

func (f *fixture) run(mpiJobName string, processingResourceType string) {
	f.runController(mpiJobName, true, false, processingResourceType)
}

func (f *fixture) runExpectError(mpiJobName string, processingResourceType string) {
	f.runController(mpiJobName, true, true, processingResourceType)
}

func (f *fixture) runController(mpiJobName string, startInformers bool, expectError bool, processingResourceType string) {
	c, i, k8sI := f.newController(processingResourceType)
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
	case core.CreateAction:
		e, _ := expected.(core.CreateAction)
		expObject := e.GetObject()
		object := a.GetObject()

		if !reflect.DeepEqual(expObject, object) {
			t.Errorf("Action %s %s has wrong object\nDiff:\n %s",
				a.GetVerb(), a.GetResource().Resource, diff.ObjectGoPrintDiff(expObject, object))
		}
	case core.UpdateAction:
		e, _ := expected.(core.UpdateAction)
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
				action.Matches("list", "poddisruptionbudgets") ||
				action.Matches("watch", "poddisruptionbudgets") ||
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
	// TODO: Until #38113 is merged, we can't use Subresource
	//action.Subresource = "status"
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

func (f *fixture) setUpRbac(mpiJob *kubeflow.MPIJob, workerReplicas int) {
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
	f.run("foo/bar/baz", gpuResourceName)
}

func TestDoNothingWithNonexistentMPIJob(t *testing.T) {
	f := newFixture(t)
	startTime := metav1.Now()
	completionTime := metav1.Now()
	mpiJob := newMPIJob("test", int32Ptr(64), &startTime, &completionTime)
	f.run(getKey(mpiJob, t), gpuResourceName)
}

func TestLauncherNotControlledByUs(t *testing.T) {
	f := newFixture(t)
	startTime := metav1.Now()
	completionTime := metav1.Now()

	mpiJob := newMPIJob("test", int32Ptr(64), &startTime, &completionTime)
	f.setUpMPIJob(mpiJob)

	launcher := newLauncher(mpiJob, "kubectl-delivery")
	launcher.OwnerReferences = nil
	f.setUpLauncher(launcher)

	f.runExpectError(getKey(mpiJob, t), gpuResourceName)
}

func TestLauncherSucceeded(t *testing.T) {
	f := newFixture(t)

	startTime := metav1.Now()
	completionTime := metav1.Now()
	mpiJob := newMPIJob("test", int32Ptr(64), &startTime, &completionTime)
	f.setUpMPIJob(mpiJob)

	launcher := newLauncher(mpiJob, "kubectl-delivery")
	launcher.Status.Succeeded = 1
	f.setUpLauncher(launcher)

	mpiJobCopy := mpiJob.DeepCopy()
	mpiJobCopy.Status.LauncherStatus = kubeflow.LauncherSucceeded
	setUpMPIJobTimestamp(mpiJobCopy, &startTime, &completionTime)
	f.expectUpdateMPIJobStatusAction(mpiJobCopy)

	f.run(getKey(mpiJob, t), gpuResourceName)
}

func TestLauncherFailed(t *testing.T) {
	f := newFixture(t)
	startTime := metav1.Now()

	mpiJob := newMPIJob("test", int32Ptr(64), &startTime, nil)
	f.setUpMPIJob(mpiJob)

	launcher := newLauncher(mpiJob, "kubectl-delivery")
	launcher.Status.Failed = 1
	f.setUpLauncher(launcher)

	mpiJobCopy := mpiJob.DeepCopy()
	mpiJobCopy.Status.LauncherStatus = kubeflow.LauncherFailed
	setUpMPIJobTimestamp(mpiJobCopy, &startTime, nil)
	f.expectUpdateMPIJobStatusAction(mpiJobCopy)

	f.run(getKey(mpiJob, t), gpuResourceName)
}

func TestLauncherDoesNotExist(t *testing.T) {
	f := newFixture(t)
	startTime := metav1.Now()
	completionTime := metav1.Now()

	mpiJob := newMPIJob("test", int32Ptr(64), &startTime, &completionTime)
	f.setUpMPIJob(mpiJob)

	expConfigMap := newConfigMap(mpiJob, 8, 8)
	f.expectCreateConfigMapAction(expConfigMap)

	expServiceAccount := newLauncherServiceAccount(mpiJob)
	f.expectCreateServiceAccountAction(expServiceAccount)

	expRole := newLauncherRole(mpiJob, 8)
	f.expectCreateRoleAction(expRole)

	expRoleBinding := newLauncherRoleBinding(mpiJob)
	f.expectCreateRoleBindingAction(expRoleBinding)

	expWorker := newWorker(mpiJob, 8, 8, gpuResourceName)
	f.expectCreateStatefulSetAction(expWorker)

	mpiJobCopy := mpiJob.DeepCopy()
	mpiJobCopy.Status.WorkerReplicas = 0
	setUpMPIJobTimestamp(mpiJobCopy, &startTime, &completionTime)
	f.expectUpdateMPIJobStatusAction(mpiJobCopy)

	f.run(getKey(mpiJob, t), gpuResourceName)
}

func TestLauncherDoesNotExistWithCustomResources(t *testing.T) {
	resourceNames := []string{cpuResourceName, gpuResourceName}
	for _, resourceName := range resourceNames {
		f := newFixture(t)
		startTime := metav1.Now()
		completionTime := metav1.Now()

		mpiJob := newMPIJobWithCustomResources("test", int32Ptr(4), 4, resourceName, &startTime, &completionTime)
		f.setUpMPIJob(mpiJob)

		expConfigMap := newConfigMap(mpiJob, 4, 4)
		f.expectCreateConfigMapAction(expConfigMap)

		expServiceAccount := newLauncherServiceAccount(mpiJob)
		f.expectCreateServiceAccountAction(expServiceAccount)

		expRole := newLauncherRole(mpiJob, 4)
		f.expectCreateRoleAction(expRole)

		expRoleBinding := newLauncherRoleBinding(mpiJob)
		f.expectCreateRoleBindingAction(expRoleBinding)

		expWorker := newWorker(mpiJob, 4, 4, resourceName)
		f.expectCreateStatefulSetAction(expWorker)

		mpiJobCopy := mpiJob.DeepCopy()
		mpiJobCopy.Status.WorkerReplicas = 0
		setUpMPIJobTimestamp(mpiJobCopy, &startTime, &completionTime)
		f.expectUpdateMPIJobStatusAction(mpiJobCopy)

		f.run(getKey(mpiJob, t), resourceName)
	}
}

func TestConfigMapNotControlledByUs(t *testing.T) {
	f := newFixture(t)
	startTime := metav1.Now()
	completionTime := metav1.Now()

	mpiJob := newMPIJob("test", int32Ptr(64), &startTime, &completionTime)
	f.setUpMPIJob(mpiJob)

	configMap := newConfigMap(mpiJob, 8, 8)
	configMap.OwnerReferences = nil
	f.setUpConfigMap(configMap)

	f.runExpectError(getKey(mpiJob, t), gpuResourceName)
}

func TestServiceAccountNotControlledByUs(t *testing.T) {
	f := newFixture(t)
	startTime := metav1.Now()
	completionTime := metav1.Now()

	mpiJob := newMPIJob("test", int32Ptr(64), &startTime, &completionTime)
	f.setUpMPIJob(mpiJob)

	f.setUpConfigMap(newConfigMap(mpiJob, 8, 8))

	serviceAccount := newLauncherServiceAccount(mpiJob)
	serviceAccount.OwnerReferences = nil
	f.setUpServiceAccount(serviceAccount)

	f.runExpectError(getKey(mpiJob, t), gpuResourceName)
}

func TestRoleNotControlledByUs(t *testing.T) {
	f := newFixture(t)
	startTime := metav1.Now()
	completionTime := metav1.Now()

	mpiJob := newMPIJob("test", int32Ptr(64), &startTime, &completionTime)
	f.setUpMPIJob(mpiJob)

	f.setUpConfigMap(newConfigMap(mpiJob, 8, 8))
	f.setUpServiceAccount(newLauncherServiceAccount(mpiJob))

	role := newLauncherRole(mpiJob, 8)
	role.OwnerReferences = nil
	f.setUpRole(role)

	f.runExpectError(getKey(mpiJob, t), gpuResourceName)
}

func TestRoleBindingNotControlledByUs(t *testing.T) {
	f := newFixture(t)
	startTime := metav1.Now()
	completionTime := metav1.Now()

	mpiJob := newMPIJob("test", int32Ptr(64), &startTime, &completionTime)
	f.setUpMPIJob(mpiJob)

	f.setUpConfigMap(newConfigMap(mpiJob, 8, 8))
	f.setUpServiceAccount(newLauncherServiceAccount(mpiJob))
	f.setUpRole(newLauncherRole(mpiJob, 8))

	roleBinding := newLauncherRoleBinding(mpiJob)
	roleBinding.OwnerReferences = nil
	f.setUpRoleBinding(roleBinding)

	f.runExpectError(getKey(mpiJob, t), gpuResourceName)
}

func TestShutdownWorker(t *testing.T) {
	f := newFixture(t)
	startTime := metav1.Now()
	completionTime := metav1.Now()

	mpiJob := newMPIJob("test", int32Ptr(64), &startTime, &completionTime)
	f.setUpMPIJob(mpiJob)

	launcher := newLauncher(mpiJob, "kubectl-delivery")
	launcher.Status.Succeeded = 1
	f.setUpLauncher(launcher)

	worker := newWorker(mpiJob, 8, 8, gpuResourceName)
	f.setUpWorker(worker)

	expWorker := newWorker(mpiJob, 0, 8, gpuResourceName)
	f.expectUpdateStatefulSetAction(expWorker)

	mpiJobCopy := mpiJob.DeepCopy()
	mpiJobCopy.Status.WorkerReplicas = 0
	mpiJobCopy.Status.LauncherStatus = kubeflow.LauncherSucceeded
	setUpMPIJobTimestamp(mpiJobCopy, &startTime, &completionTime)
	f.expectUpdateMPIJobStatusAction(mpiJobCopy)

	f.run(getKey(mpiJob, t), gpuResourceName)
}

func TestWorkerNotControlledByUs(t *testing.T) {
	f := newFixture(t)
	startTime := metav1.Now()
	completionTime := metav1.Now()

	mpiJob := newMPIJob("test", int32Ptr(64), &startTime, &completionTime)
	f.setUpMPIJob(mpiJob)

	f.setUpConfigMap(newConfigMap(mpiJob, 8, 8))
	f.setUpRbac(mpiJob, 8)

	worker := newWorker(mpiJob, 8, 8, gpuResourceName)
	worker.OwnerReferences = nil
	f.setUpWorker(worker)

	f.runExpectError(getKey(mpiJob, t), gpuResourceName)
}

func TestLauncherActive(t *testing.T) {
	f := newFixture(t)
	startTime := metav1.Now()
	completionTime := metav1.Now()

	mpiJob := newMPIJob("test", int32Ptr(8), &startTime, &completionTime)

	f.setUpMPIJob(mpiJob)

	f.setUpConfigMap(newConfigMap(mpiJob, 1, 8))
	f.setUpRbac(mpiJob, 1)

	launcher := newLauncher(mpiJob, "kubectl-delivery")
	launcher.Status.Active = 1
	f.setUpLauncher(launcher)

	worker := newWorker(mpiJob, 1, 8, gpuResourceName)
	f.setUpWorker(worker)

	mpiJobCopy := mpiJob.DeepCopy()
	mpiJobCopy.Status.LauncherStatus = kubeflow.LauncherActive
	setUpMPIJobTimestamp(mpiJobCopy, &startTime, &completionTime)
	f.expectUpdateMPIJobStatusAction(mpiJobCopy)

	f.run(getKey(mpiJob, t), gpuResourceName)
}

func TestWorkerReady(t *testing.T) {
	f := newFixture(t)
	startTime := metav1.Now()
	completionTime := metav1.Now()

	mpiJob := newMPIJob("test", int32Ptr(16), &startTime, &completionTime)
	f.setUpMPIJob(mpiJob)

	f.setUpConfigMap(newConfigMap(mpiJob, 2, 8))
	f.setUpRbac(mpiJob, 2)

	worker := newWorker(mpiJob, 2, 8, gpuResourceName)
	worker.Status.ReadyReplicas = 2
	f.setUpWorker(worker)

	expLauncher := newLauncher(mpiJob, "kubectl-delivery")
	f.expectCreateJobAction(expLauncher)

	mpiJobCopy := mpiJob.DeepCopy()
	mpiJobCopy.Status.WorkerReplicas = 2
	setUpMPIJobTimestamp(mpiJobCopy, &startTime, &completionTime)
	f.expectUpdateMPIJobStatusAction(mpiJobCopy)

	f.run(getKey(mpiJob, t), gpuResourceName)
}

func TestWorkerReadyWithCPUs(t *testing.T) {
	f := newFixture(t)
	startTime := metav1.Now()
	completionTime := metav1.Now()

	mpiJob := newMPIJobWithCPUs("test", int32Ptr(16), &startTime, &completionTime)
	f.setUpMPIJob(mpiJob)

	f.setUpConfigMap(newConfigMap(mpiJob, 2, 8))
	f.setUpRbac(mpiJob, 2)

	worker := newWorker(mpiJob, 2, 8, cpuResourceName)
	worker.Status.ReadyReplicas = 2
	f.setUpWorker(worker)

	expLauncher := newLauncher(mpiJob, "kubectl-delivery")
	f.expectCreateJobAction(expLauncher)

	mpiJobCopy := mpiJob.DeepCopy()
	mpiJobCopy.Status.WorkerReplicas = 2
	setUpMPIJobTimestamp(mpiJobCopy, &startTime, &completionTime)
	f.expectUpdateMPIJobStatusAction(mpiJobCopy)

	f.run(getKey(mpiJob, t), cpuResourceName)
}

func int32Ptr(i int32) *int32 { return &i }
