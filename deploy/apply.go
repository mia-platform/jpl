// Copyright 2022 Mia srl
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

package jpl

import (
	"context"
	"fmt"

	"github.com/pkg/errors"
	batchv1 "k8s.io/api/batch/v1"
	batchv1beta1 "k8s.io/api/batch/v1beta1"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/jsonmergepatch"
	"k8s.io/apimachinery/pkg/util/mergepatch"
	"k8s.io/apimachinery/pkg/util/strategicpatch"
	"k8s.io/client-go/dynamic"
	"k8s.io/kubectl/pkg/scheme"
)

type ApplyFunction func(clients *K8sClients, res Resource, deployConfig DeployConfig) error

// DecorateApplyFunction allows to generate an apply function with a generic number of decorators
// before calling the actual apply
func DecorateApplyFunction(apply ApplyFunction, decorators ...func(ApplyFunction) ApplyFunction) ApplyFunction {
	for _, f := range decorators {
		apply = f(apply)
	}
	return apply
}

// DecorateDefaultApplyFunction allows to decorate the default apply function
func DecorateDefaultApplyFunction(decorators ...func(ApplyFunction) ApplyFunction) ApplyFunction {
	return DecorateApplyFunction(defaultApplyResource, decorators...)
}

// defaultApplyResource applies the resource to the cluster following
// the default apply logic
func defaultApplyResource(clients *K8sClients, res Resource, deployConfig DeployConfig) error {
	gvr, err := FromGVKtoGVR(clients.discovery, res.Object.GroupVersionKind())
	if err != nil {
		return err
	}

	onClusterObj, err := GetResource(gvr, clients, res)
	if err != nil {
		if apierrors.IsNotFound(err) {
			return CreateResource(gvr, clients, res)
		} else {
			return err
		}
	}

	if res.Object.GetKind() == "Secret" || res.Object.GetKind() == "ConfigMap" || res.Object.GetKind() == "CustomResourceDefinition" {
		fmt.Printf("Replacing %s: %s\n", res.Object.GetKind(), res.Object.GetName())
		return ReplaceResource(gvr, clients, res)
	}

	return PatchResource(gvr, clients, res, onClusterObj)
}

// GetResource returns the identified resource if present on the cluster
// if the resource does not exist, returns a NotFound error
func GetResource(gvr schema.GroupVersionResource, clients *K8sClients, res Resource) (*unstructured.Unstructured, error) {
	return clients.dynamic.Resource(gvr).
		Namespace(res.Object.GetNamespace()).
		Get(context.Background(), res.Object.GetName(), metav1.GetOptions{})
}

// CreateResource handles the creation of a k8s resource when
// not already present on the cluster
func CreateResource(gvr schema.GroupVersionResource, clients *K8sClients, res Resource) error {
	fmt.Printf("Creating %s: %s\n", res.Object.GetKind(), res.Object.GetName())

	// creates kubectl.kubernetes.io/last-applied-configuration annotation
	// inside the resource except for Secrets and ConfigMaps
	if res.Object.GetKind() != "Secret" && res.Object.GetKind() != "ConfigMap" {
		orignAnn := res.Object.GetAnnotations()
		if orignAnn == nil {
			orignAnn = make(map[string]string)
		}
		objJson, err := res.Object.MarshalJSON()
		if err != nil {
			return err
		}
		orignAnn[corev1.LastAppliedConfigAnnotation] = string(objJson)
		res.Object.SetAnnotations(orignAnn)
	}

	if err := cronJobAutoCreate(clients.dynamic, &res.Object); err != nil {
		return err
	}

	_, err := clients.dynamic.Resource(gvr).
		Namespace(res.Object.GetNamespace()).
		Create(context.Background(),
			&res.Object,
			metav1.CreateOptions{})

	return err
}

// ReplaceResource handles resource replacement on the cluster
// e.g. for Secrets and ConfigMaps
func ReplaceResource(gvr schema.GroupVersionResource, clients *K8sClients, res Resource) error {
	_, err := clients.dynamic.Resource(gvr).
		Namespace(res.Object.GetNamespace()).
		Update(context.Background(),
			&res.Object,
			metav1.UpdateOptions{})
	return err
}

// PatchResource patches a resource that already exists on the cluster
// and needs to be updated
func PatchResource(gvr schema.GroupVersionResource, clients *K8sClients, res Resource, onClusterObj *unstructured.Unstructured) error {
	// create the patch
	patch, patchType, err := createPatch(*onClusterObj, res)
	if err != nil {
		return errors.Wrap(err, "failed to create patch")
	}

	if _, err := clients.dynamic.Resource(gvr).
		Namespace(res.Object.GetNamespace()).
		Patch(context.Background(),
			res.Object.GetName(), patchType, patch, metav1.PatchOptions{}); err != nil {
		return errors.Wrap(err, "failed to patch")
	}
	return err
}

// annotateWithLastApplied annotates a given resource with corev1.LastAppliedConfigAnnotation
func annotateWithLastApplied(res Resource) (unstructured.Unstructured, error) {
	annotatedRes := res.Object.DeepCopy()
	annotations := annotatedRes.GetAnnotations()
	if annotations == nil {
		annotations = make(map[string]string)
	}
	if _, found := annotations[corev1.LastAppliedConfigAnnotation]; found {
		delete(annotations, corev1.LastAppliedConfigAnnotation)
		annotatedRes.SetAnnotations(annotations)
	}

	resJson, err := annotatedRes.MarshalJSON()
	if err != nil {
		return unstructured.Unstructured{}, err
	}

	annotations[corev1.LastAppliedConfigAnnotation] = string(resJson)
	annotatedRes.SetAnnotations(annotations)

	return *annotatedRes, nil
}

// createPatch returns the patch to be used in order to update the resource inside the cluster.
// The function performs a Three Way Merge Patch with the last applied configuration written in the
// object annotation, the actual resource state deployed inside the cluster and the desired state after
// the update.
func createPatch(currentObj unstructured.Unstructured, target Resource) ([]byte, types.PatchType, error) {
	// Get last applied config from current object annotation
	lastAppliedConfigJson := currentObj.GetAnnotations()[corev1.LastAppliedConfigAnnotation]

	// Get the desired configuration
	annotatedTarget, err := annotateWithLastApplied(target)
	if err != nil {
		return nil, types.StrategicMergePatchType, err
	}
	targetJson, err := annotatedTarget.MarshalJSON()
	if err != nil {
		return nil, types.StrategicMergePatchType, err
	}

	// Get the resource in the cluster
	currentJson, err := currentObj.MarshalJSON()
	if err != nil {
		return nil, types.StrategicMergePatchType, errors.Wrap(err, "serializing live configuration")
	}

	// Get the resource scheme
	versionedObject, err := scheme.Scheme.New(*target.GroupVersionKind)

	// use a three way json merge if the resource is a CRD
	if runtime.IsNotRegisteredError(err) {
		// fall back to generic JSON merge patch
		patchType := types.MergePatchType
		preconditions := []mergepatch.PreconditionFunc{mergepatch.RequireKeyUnchanged("apiVersion"),
			mergepatch.RequireKeyUnchanged("kind"), mergepatch.RequireMetadataKeyUnchanged("name")}
		patch, err := jsonmergepatch.CreateThreeWayJSONMergePatch([]byte(lastAppliedConfigJson), targetJson, currentJson, preconditions...)
		return patch, patchType, err
	} else if err != nil {
		return nil, types.StrategicMergePatchType, err
	}

	patchMeta, err := strategicpatch.NewPatchMetaFromStruct(versionedObject)
	if err != nil {
		return nil, types.StrategicMergePatchType, errors.Wrap(err, "unable to create patch metadata from object")
	}

	patch, err := strategicpatch.CreateThreeWayMergePatch([]byte(lastAppliedConfigJson), targetJson, currentJson, patchMeta, true)
	return patch, types.StrategicMergePatchType, err
}

// cronJobAutoCreate creates a Job from every CronJob having the mia-platform.eu/autocreate annotation set to true
func cronJobAutoCreate(k8sClient dynamic.Interface, res *unstructured.Unstructured) error {
	if res.GetKind() != "CronJob" {
		return nil
	}
	val, ok := res.GetAnnotations()[GetMiaAnnotation("autocreate")]
	if !ok || val != "true" {
		return nil
	}

	if _, err := createJobFromCronjob(k8sClient, res); err != nil {
		return err
	}
	return nil
}

// createJobFromCronjob creates a job from a given cronjob, returning its name
func createJobFromCronjob(k8sClient dynamic.Interface, res *unstructured.Unstructured) (string, error) {

	var cronjobObj batchv1beta1.CronJob
	err := runtime.DefaultUnstructuredConverter.
		FromUnstructured(res.Object, &cronjobObj)
	if err != nil {
		return "", fmt.Errorf("error in conversion to Cronjob")
	}
	annotations := make(map[string]string)
	annotations["cronjob.kubernetes.io/instantiate"] = "manual"
	job := &batchv1.Job{
		TypeMeta: metav1.TypeMeta{APIVersion: batchv1.SchemeGroupVersion.String(), Kind: "Job"},
		ObjectMeta: metav1.ObjectMeta{
			// Use this instead of Name field to avoid name conflicts
			GenerateName: res.GetName() + "-",
			Annotations:  annotations,
			Labels:       cronjobObj.Spec.JobTemplate.Labels,

			// TODO: decide if it necessary to include it or not. At the moment it
			// prevents the pod creation saying that it cannot mount the default token
			// inside the container
			//
			// OwnerReferences: []metav1.OwnerReference{
			// 	{
			// 		APIVersion: batchv1beta1.SchemeGroupVersion.String(),
			// 		Kind:       cronjobObj.Kind,
			// 		Name:       cronjobObj.GetName(),
			// 		UID:        cronjobObj.GetUID(),
			// 	},
			// },
		},
		Spec: cronjobObj.Spec.JobTemplate.Spec,
	}

	fmt.Printf("Creating job from cronjob: %s\n", res.GetName())

	unstrCurrentObj, err := runtime.DefaultUnstructuredConverter.ToUnstructured(&job)
	if err != nil {
		return "", err
	}

	jobCreated, err := k8sClient.Resource(gvrJobs).
		Namespace(res.GetNamespace()).
		Create(context.Background(),
			&unstructured.Unstructured{
				Object: unstrCurrentObj,
			},
			metav1.CreateOptions{})
	if err != nil {
		return "", err
	}

	return jobCreated.GetName(), nil
}
