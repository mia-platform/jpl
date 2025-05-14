// Copyright Mia srl
// SPDX-License-Identifier: Apache-2.0
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

package poller

import (
	"fmt"
	"reflect"
	"strings"
	"time"

	appsv1 "k8s.io/api/apps/v1"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	apiextv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
)

const (
	generationMessageFormat = "%q current generation is %d, observed generation is %d"
	deletionMessage         = "Resource is scheduled for deletion"
	currentMessage          = "Resource is current"

	crdInProgressMessage = "CRD installation in progress"
	crdCurrentMessage    = "CRD is established"

	serviceInProgressMessage = "Load Balancer assignment in progress"

	pvcCurrentMessage          = "PVC is Bound"
	pvcInProgressMessageFormat = "PVC current phase: %q"

	jobCurrentMessageFormat = "Job Completed. succeeded: %d/%d"
	jobSuspendedMessage     = "Job is suspended"
	jobInProgressMessage    = "Job is not started yet"
	jobRunningMessage       = "Job in progress. success:%d, active: %d, failed: %d"

	podSuccededMessage                  = "Pod has been successfully completed"
	podFailedMessage                    = "Pod has failed to complete with success"
	podUnscheduledMessage               = "Pod is waiting to be scheduled"
	podPendingMessage                   = "Pod is pending"
	podInProgressMessage                = "Pod is in progress"
	crashloopingContainersMessageFormat = "Containers in CrashLoopBackOff: %q"
	podReadyMessage                     = "Pod is Ready"
	podRunningMessage                   = "Pod is running but is not ready yet"

	deploymentDeadlineMessage               = "Deployment failed because the deadline has exceeed"
	deploymentNotProgressingMessage         = "Deployment is not progressing because ReplicaSet is not available"
	deploymentNotAvailableMessage           = "Deployment is not available"
	deploymentCurrentMessageFormat          = "Deployment is available with %d replicas"
	deploymentFewReplicasMessageFormat      = "Deployment creating replicas: %d/%d"
	deploymentUpdatingReplicasMessageFormat = "Deployment updating replicas: %d/%d"
	deploymentDeletingReplicasMessageFormat = "Deployment terminating replicas: %d"
	deploymentWaitingReplicasMessageFormat  = "Deployment waiting replicas: %d/%d"

	daemonsetInProgressMessage             = "DaemonSet is in progress"
	daemonsetCurrentMessageFormat          = "DaemonSet is available with %d replicas"
	daemonsetFewReplicasMessageFormat      = "DaemonSet creating replicas: %d/%d"
	daemonsetUpdatingReplicasMessageFormat = "DaemonSet updating replicas: %d/%d"
	daemonsetWaitingReplicasMessageFormat  = "DaemonSet waiting replicas: %d/%d"

	statefulSetOnDeleteStrategyMessage        = "StatefulSet is set to use OnDelete strategy"
	statefulSetFewReplicasMessageFormat       = "StatefulSet creating replicas: %d/%d"
	statefulSetWaitingReplicasMessageFormat   = "StatefulSet waiting replicas: %d/%d"
	statefulSetUpdatingReplicasMessageFormat  = "StatefulSet updating replicas: %d/%d"
	statefulSetDeletingReplicasMessageFormat  = "StatefulSet terminating replicas: %d"
	statefulSetPartitionRolloutMessageFormat  = "StatefulSet partition rolling out: %d/%d"
	statefulSetPartitionCompleteMessageFormat = "StatefulSet partition completed with %d replicas"
	statefulSetRevisionMismatchMessage        = "StatefulSet revision mismatch"
	statefulSetCurrentMessageFormat           = "StatefulSet is available with %d replicas"
)

var (
	unscheduledWindow = -30 * time.Second

	crdGK = apiextv1.SchemeGroupVersion.WithKind(reflect.TypeOf(apiextv1.CustomResourceDefinition{}).Name()).GroupKind()
	svcGK = corev1.SchemeGroupVersion.WithKind(reflect.TypeOf(corev1.Service{}).Name()).GroupKind()
	pvcGK = corev1.SchemeGroupVersion.WithKind(reflect.TypeOf(corev1.PersistentVolumeClaim{}).Name()).GroupKind()
	podGK = corev1.SchemeGroupVersion.WithKind(reflect.TypeOf(corev1.Pod{}).Name()).GroupKind()

	jobGK    = batchv1.SchemeGroupVersion.WithKind(reflect.TypeOf(batchv1.Job{}).Name()).GroupKind()
	deployGK = appsv1.SchemeGroupVersion.WithKind(reflect.TypeOf(appsv1.Deployment{}).Name()).GroupKind()
	dsGK     = appsv1.SchemeGroupVersion.WithKind(reflect.TypeOf(appsv1.DaemonSet{}).Name()).GroupKind()
	stsGK    = appsv1.SchemeGroupVersion.WithKind(reflect.TypeOf(appsv1.StatefulSet{}).Name()).GroupKind()
)

// statusCheck will perform a series of checks to find if objects has some properties set on its status
// that can be extrapolated to find what its current status in the cluster is.
// The checks are:
//   - presence of deletion timestamp
//   - comparing current and observed generations
//   - specific checks for core native k8s kinds
//   - custom checks provided by the user
//   - presence of particular conditions types
func statusCheck(object *unstructured.Unstructured, customCheckers CustomStatusCheckers) (*Result, error) {
	// 1. control if a deletion timestamp is present on the resource,
	// if is not nil the object has been marked for deletion
	deletionTimestamp := object.GetDeletionTimestamp()
	if !deletionTimestamp.IsZero() {
		return terminatingResult(deletionMessage), nil
	}

	// 2. control if the resource has the observedGeneration property and its equal to the current generation
	if result, err := checkGenerations(object); result != nil || err != nil {
		return result, err
	}

	// 3. do a series of check for well known core kubernetes apis for checking their status properties
	if result, err := checkCoreResources(object); result != nil || err != nil {
		return result, err
	}

	// 4. call custom checks provided by end users
	if fn, found := customCheckers[object.GroupVersionKind().GroupKind()]; found {
		return fn(object)
	}

	// 5. control resource status conditions array and check if there are the standard one or the widely used "ready"
	if result, err := checkStatusConditions(object); result != nil || err != nil {
		return result, err
	}

	// 6. if any other control is not found, we cannot check anything else, presume that the resource is current
	return currentResult(currentMessage), nil
}

// checkGenerations will search if object has the observedGeneration property in its status and compare it with
// the current generation reported in its metadata. If they aren't the same we can assume that the resource is
// in progress
func checkGenerations(object *unstructured.Unstructured) (*Result, error) {
	observedGeneration, found, err := unstructured.NestedInt64(object.Object, "status", "observedGeneration")
	if !found || err != nil {
		return nil, err
	}

	currentGeneration := object.GetGeneration()
	if observedGeneration != currentGeneration {
		msg := fmt.Sprintf(generationMessageFormat, object.GroupVersionKind().Kind, currentGeneration, observedGeneration)
		return inProgressResult(msg), nil
	}

	return nil, nil
}

// checkStatusConditions will search if in the conditions property of the resource status are presents default
// status or the widely used "Ready" one.
// The check on the ready condition is not a deterministic because is not a design recommendation of kubernetes,
// so its implementation varies between different controller and resources. Is not certain that the condition is
// set to false at the start of the reconciling cycle, we can check its presence before the controller has a chance
// to set it, and we don't know if the status "false" will change in the future or its reconciliation is stuck
func checkStatusConditions(object *unstructured.Unstructured) (*Result, error) {
	conditionsData, found, err := unstructured.NestedSlice(object.Object, "status", "conditions")
	if err != nil {
		return nil, err
	}

	if !found {
		return nil, nil
	}

	for _, conditionData := range conditionsData {
		condition := new(metav1.Condition)
		err := runtime.DefaultUnstructuredConverter.FromUnstructured(conditionData.(map[string]interface{}), condition)
		if err != nil {
			continue
		}

		switch condition.Type {
		case "Ready":
			switch condition.Status {
			case metav1.ConditionTrue:
				return currentResult(currentMessage), nil
			case metav1.ConditionFalse:
				return inProgressResult(condition.Message), nil
			case metav1.ConditionUnknown:
				return inProgressResult(condition.Message), nil
			}
		case "Reconciling":
			if condition.Status == metav1.ConditionTrue {
				return inProgressResult(condition.Message), nil
			}
		case "Stalled":
			if condition.Status == metav1.ConditionTrue {
				return failedResult(condition.Message), nil
			}
		}
	}

	return nil, nil
}

// checkCoreResources will associate specific control to some core kubernetes resources matched using ResourceKind
func checkCoreResources(object *unstructured.Unstructured) (*Result, error) {
	gk := object.GroupVersionKind().GroupKind()

	switch gk {
	case crdGK:
		return crdStatusCheck(object)
	case svcGK:
		return serviceStatusCheck(object)
	case pvcGK:
		return pvcStatusCheck(object)
	case jobGK:
		return jobStatusCheck(object)
	case podGK:
		return podStatusCheck(object)
	case deployGK:
		return deployStatusCheck(object)
	case dsGK:
		return daemonSetStatusCheck(object)
	case stsGK:
		return statefulSetStatusCheck(object)
	}

	return nil, nil
}

// crdStatusCheck will extract information from the CRD to check:
//   - NameAccepted condition current status
//   - Established condition current status
//   - return in progress if no matches
func crdStatusCheck(object *unstructured.Unstructured) (*Result, error) {
	crd := new(apiextv1.CustomResourceDefinition)
	if err := runtime.DefaultUnstructuredConverter.FromUnstructured(object.Object, crd); err != nil {
		return nil, err
	}

	for _, condition := range crd.Status.Conditions {
		switch {
		case condition.Type == apiextv1.NamesAccepted && condition.Status == apiextv1.ConditionFalse:
			return failedResult(condition.Message), nil
		case condition.Type == apiextv1.Established && condition.Status == apiextv1.ConditionTrue:
			return currentResult(crdCurrentMessage), nil
		case condition.Type == apiextv1.NamesAccepted && condition.Status == apiextv1.ConditionFalse && condition.Reason != "Installing":
			return failedResult(condition.Message), nil
		}
	}

	return inProgressResult("CRD installation in progress"), nil
}

// serviceStatusCheck will extract information from the Service to check:
//   - service Type, if != to LoadBalancer the service is considered current
//   - service status to check the number of loadbalancer ingresses available
func serviceStatusCheck(object *unstructured.Unstructured) (*Result, error) {
	service := new(corev1.Service)
	if err := runtime.DefaultUnstructuredConverter.FromUnstructured(object.Object, service); err != nil {
		return nil, err
	}

	if service.Spec.Type == corev1.ServiceTypeLoadBalancer {
		if len(service.Status.LoadBalancer.Ingress) == 0 {
			return inProgressResult(serviceInProgressMessage), nil
		}
	}

	return currentResult(currentMessage), nil
}

// pvcStatusCheck will extract information from the PersistenVolumeClaim to check its current Phase
func pvcStatusCheck(object *unstructured.Unstructured) (*Result, error) {
	pvc := new(corev1.PersistentVolumeClaim)
	if err := runtime.DefaultUnstructuredConverter.FromUnstructured(object.Object, pvc); err != nil {
		return nil, err
	}

	phase := pvc.Status.Phase
	if len(phase) == 0 {
		phase = "unknown"
	}

	if phase == corev1.ClaimBound {
		return currentResult(pvcCurrentMessage), nil
	}

	return inProgressResult(fmt.Sprintf(pvcInProgressMessageFormat, phase)), nil
}

// jobStatusCheck will extract information from the Job to check:
//   - if a condition of type Failed with status True exists
//   - if a condition of type Suspended with status true exists
//   - if a condition of type Complete with status true exists
//   - if startTime is present in status
func jobStatusCheck(object *unstructured.Unstructured) (*Result, error) {
	job := new(batchv1.Job)
	if err := runtime.DefaultUnstructuredConverter.FromUnstructured(object.Object, job); err != nil {
		return nil, err
	}

	var parallelism, completions int32
	_ = metav1.Convert_Pointer_int32_To_int32(&job.Spec.Parallelism, &parallelism, nil) // cannot return error
	parallelism = max(parallelism, 1)
	_ = metav1.Convert_Pointer_int32_To_int32(&job.Spec.Completions, &completions, nil) // cannot return error
	completions = max(completions, parallelism)
	succeeded := job.Status.Succeeded

	for _, condition := range job.Status.Conditions {
		switch {
		case condition.Type == batchv1.JobFailed && condition.Status == corev1.ConditionTrue:
			return failedResult(condition.Message), nil
		case condition.Type == batchv1.JobComplete && condition.Status == corev1.ConditionTrue:
			message := fmt.Sprintf(jobCurrentMessageFormat, succeeded, completions)
			return currentResult(message), nil
		case condition.Type == batchv1.JobSuspended && condition.Status == corev1.ConditionTrue:
			return currentResult(jobSuspendedMessage), nil
		}
	}

	if job.Status.StartTime.IsZero() {
		return inProgressResult(jobInProgressMessage), nil
	}

	active := job.Status.Active
	failed := job.Status.Failed
	message := fmt.Sprintf(jobRunningMessage, succeeded, active, failed)
	return inProgressResult(message), nil
}

// podStatusCheck will extract information from the Pod to check:
//   - phase property in status
//   - in case of phase running or pending, eventual conditions available
//   - in case of phase running without correct condition check if some containers are crashlooping
func podStatusCheck(object *unstructured.Unstructured) (*Result, error) {
	pod := new(corev1.Pod)
	if err := runtime.DefaultUnstructuredConverter.FromUnstructured(object.Object, pod); err != nil {
		return nil, err
	}

	switch pod.Status.Phase {
	case corev1.PodSucceeded:
		return currentResult(podSuccededMessage), nil
	case corev1.PodFailed:
		return failedResult(podFailedMessage), nil
	case corev1.PodPending:
		for _, condition := range pod.Status.Conditions {
			if condition.Type == corev1.PodScheduled && condition.Status == corev1.ConditionFalse {
				creationTime := pod.CreationTimestamp.Time
				deltaWindowTime := time.Now().Add(unscheduledWindow) // add some leeway to declare a failed scheduling
				if condition.Reason == corev1.PodReasonUnschedulable && creationTime.After(deltaWindowTime) {
					return inProgressResult(podUnscheduledMessage), nil
				}
				return failedResult(condition.Message), nil
			}
		}
		return inProgressResult(podPendingMessage), nil
	case corev1.PodRunning:
		for _, condition := range pod.Status.Conditions {
			if condition.Type == corev1.PodReady && condition.Status == corev1.ConditionTrue {
				return currentResult(podReadyMessage), nil
			}
		}

		if containers := crashingContainers(pod.Status); len(containers) > 0 {
			msg := fmt.Sprintf(crashloopingContainersMessageFormat, strings.Join(containers, ", "))
			return failedResult(msg), nil
		}

		return inProgressResult(podRunningMessage), nil
	}

	return inProgressResult(podInProgressMessage), nil
}

// crashingContainers will return an array of containers and/or initContainers names that are in CrashLoopBackOff
func crashingContainers(podStatus corev1.PodStatus) []string {
	containers := make([]string, 0)
	for _, cs := range podStatus.ContainerStatuses {
		if waiting := cs.State.Waiting; waiting != nil && waiting.Reason == "CrashLoopBackOff" {
			containers = append(containers, cs.Name)
		}
	}

	for _, cs := range podStatus.InitContainerStatuses {
		if waiting := cs.State.Waiting; waiting != nil && waiting.Reason == "CrashLoopBackOff" {
			containers = append(containers, cs.Name)
		}
	}

	return containers
}

// deployStatusCheck will extract information from the Deployment to check:
//   - presence of "progressing" condition
//   - differences between replicas number reports
//   - presence of "available" condition
func deployStatusCheck(object *unstructured.Unstructured) (*Result, error) {
	deploy := new(appsv1.Deployment)
	if err := runtime.DefaultUnstructuredConverter.FromUnstructured(object.Object, deploy); err != nil {
		return nil, err
	}

	// if ProgressDeadlineSeconds is not set the controller than will not set the `Progressing` condition
	deployIsProgressing := deploy.Spec.ProgressDeadlineSeconds == nil

	deployIsAvailable := false // save if the available condition is found for later
	for _, condition := range deploy.Status.Conditions {
		switch {
		case condition.Type == appsv1.DeploymentProgressing && condition.Reason == "ProgressDeadlineExceeded":
			return failedResult(deploymentDeadlineMessage), nil
		case condition.Type == appsv1.DeploymentProgressing && condition.Reason == "NewReplicaSetAvailable" && condition.Status == corev1.ConditionTrue:
			deployIsProgressing = true
		case condition.Type == appsv1.DeploymentAvailable && condition.Status == corev1.ConditionTrue:
			deployIsAvailable = true
		}
	}

	if result := replicasStatusResult(deploy); result != nil {
		return result, nil
	}

	if !deployIsProgressing {
		return inProgressResult(deploymentNotProgressingMessage), nil
	}

	if !deployIsAvailable {
		return inProgressResult(deploymentNotAvailableMessage), nil
	}

	return currentResult(fmt.Sprintf(deploymentCurrentMessageFormat, deploy.Status.Replicas)), nil
}

// replicasStatusResult return a result for deploy based on the difference between the different replicas
// report in its status, if no differences are found nil is returned
func replicasStatusResult(deploy *appsv1.Deployment) *Result {
	var replicas int32
	if deploy.Spec.Replicas != nil {
		replicas = *deploy.Spec.Replicas
	} else {
		replicas = 1
	}

	statusReplicas := deploy.Status.Replicas
	updatedReplicas := deploy.Status.UpdatedReplicas
	availableReplicas := deploy.Status.AvailableReplicas
	readyReplicas := deploy.Status.ReadyReplicas
	switch {
	case replicas > statusReplicas:
		msg := fmt.Sprintf(deploymentFewReplicasMessageFormat, statusReplicas, replicas)
		return inProgressResult(msg)
	case replicas > updatedReplicas:
		msg := fmt.Sprintf(deploymentUpdatingReplicasMessageFormat, updatedReplicas, replicas)
		return inProgressResult(msg)
	case statusReplicas > replicas:
		msg := fmt.Sprintf(deploymentDeletingReplicasMessageFormat, statusReplicas-replicas)
		return inProgressResult(msg)
	case updatedReplicas > availableReplicas:
		msg := fmt.Sprintf(deploymentWaitingReplicasMessageFormat, availableReplicas, updatedReplicas)
		return inProgressResult(msg)
	case replicas > readyReplicas:
		msg := fmt.Sprintf(deploymentWaitingReplicasMessageFormat, readyReplicas, replicas)
		return inProgressResult(msg)
	}

	return nil
}

// daemonSetStatusCheck will extract information from the DaemonSet to check the scheduled number of pods in their
// different status in the status block of the resource
func daemonSetStatusCheck(object *unstructured.Unstructured) (*Result, error) {
	_, found, err := unstructured.NestedInt64(object.Object, "status", "observedGeneration")
	if err != nil {
		return nil, err
	}

	if !found {
		return inProgressResult(daemonsetInProgressMessage), nil
	}

	ds := new(appsv1.DaemonSet)
	if err := runtime.DefaultUnstructuredConverter.FromUnstructured(object.Object, ds); err != nil {
		return nil, err
	}

	desiredScheduled := ds.Status.DesiredNumberScheduled
	currentScheduled := ds.Status.CurrentNumberScheduled
	updatedScheduled := ds.Status.UpdatedNumberScheduled
	available := ds.Status.NumberAvailable
	ready := ds.Status.NumberReady

	switch {
	case desiredScheduled > currentScheduled:
		msg := fmt.Sprintf(daemonsetFewReplicasMessageFormat, currentScheduled, desiredScheduled)
		return inProgressResult(msg), nil
	case desiredScheduled > updatedScheduled:
		msg := fmt.Sprintf(daemonsetUpdatingReplicasMessageFormat, updatedScheduled, desiredScheduled)
		return inProgressResult(msg), nil
	case desiredScheduled > available:
		msg := fmt.Sprintf(daemonsetWaitingReplicasMessageFormat, available, desiredScheduled)
		return inProgressResult(msg), nil
	case desiredScheduled > ready:
		msg := fmt.Sprintf(daemonsetWaitingReplicasMessageFormat, ready, desiredScheduled)
		return inProgressResult(msg), nil
	}

	return currentResult(fmt.Sprintf(daemonsetCurrentMessageFormat, desiredScheduled)), nil
}

// statefulSetStatusCheck will extract information from the StatefulSet to check:
//   - the update strategy type
//   - the scheduled number of pods in their different status in the status block of the resource
//   - the rollign update partition property and the updated replicas status property
func statefulSetStatusCheck(object *unstructured.Unstructured) (*Result, error) {
	sts := new(appsv1.StatefulSet)
	if err := runtime.DefaultUnstructuredConverter.FromUnstructured(object.Object, sts); err != nil {
		return nil, err
	}

	if sts.Spec.UpdateStrategy.Type == appsv1.OnDeleteStatefulSetStrategyType {
		return currentResult(statefulSetOnDeleteStrategyMessage), nil
	}

	replicas := int32(1)
	if sts.Spec.Replicas != nil {
		replicas = *sts.Spec.Replicas
	}

	readyReplicas := sts.Status.ReadyReplicas
	currentReplicas := sts.Status.CurrentReplicas
	statusReplicas := sts.Status.Replicas

	switch {
	case replicas > statusReplicas:
		msg := fmt.Sprintf(statefulSetFewReplicasMessageFormat, statusReplicas, replicas)
		return inProgressResult(msg), nil
	case replicas > readyReplicas:
		msg := fmt.Sprintf(statefulSetWaitingReplicasMessageFormat, readyReplicas, replicas)
		return inProgressResult(msg), nil
	case statusReplicas > replicas:
		msg := fmt.Sprintf(statefulSetDeletingReplicasMessageFormat, statusReplicas-replicas)
		return inProgressResult(msg), nil
	case replicas > currentReplicas:
		msg := fmt.Sprintf(statefulSetUpdatingReplicasMessageFormat, currentReplicas, replicas)
		return inProgressResult(msg), nil
	}

	if sts.Spec.UpdateStrategy.RollingUpdate != nil && sts.Spec.UpdateStrategy.RollingUpdate.Partition != nil {
		partition := *sts.Spec.UpdateStrategy.RollingUpdate.Partition
		updatedReplicas := sts.Status.UpdatedReplicas
		if updatedReplicas < (replicas - partition) {
			msg := fmt.Sprintf(statefulSetPartitionRolloutMessageFormat, updatedReplicas, replicas-partition)
			return inProgressResult(msg), nil
		}
		return currentResult(fmt.Sprintf(statefulSetPartitionCompleteMessageFormat, updatedReplicas)), nil
	}

	if sts.Status.CurrentRevision != sts.Status.UpdateRevision {
		return inProgressResult(statefulSetRevisionMismatchMessage), nil
	}

	return currentResult(fmt.Sprintf(statefulSetCurrentMessageFormat, statusReplicas)), nil
}
