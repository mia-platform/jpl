// Copyright 2020 Mia srl
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
	"testing"

	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/types"
)

// func TestCronJobAutoCreate(t *testing.T) {
// 	scheme := runtime.NewScheme()
// 	_ = corev1.AddToScheme(scheme)
// 	_ = batchv1.AddToScheme(scheme)

// 	testcases := []struct {
// 		description string
// 		setup       func(obj *unstructured.Unstructured)
// 		expected    int
// 	}{
// 		{
// 			description: "autocreate true",
// 			expected:    1,
// 			setup: func(obj *unstructured.Unstructured) {
// 				obj.SetAnnotations(map[string]string{
// 					"mia-platform.eu/autocreate": "true",
// 				})
// 			},
// 		},
// 		{
// 			description: "autocreate false",
// 			expected:    0,
// 			setup: func(obj *unstructured.Unstructured) {
// 				obj.SetAnnotations(map[string]string{
// 					"mia-platform.eu/autocreate": "false",
// 				})
// 			},
// 		},
// 		{
// 			description: "no annotation",
// 			expected:    0,
// 			setup: func(obj *unstructured.Unstructured) {
// 				obj.SetAnnotations(map[string]string{})
// 			},
// 		},
// 	}

// 	for _, tt := range testcases {
// 		t.Run(tt.description, func(t *testing.T) {
// 			_, cronjob, err := NewResourcesFromFile("testdata/apply/cronjob-test.cronjob.yml", "default", FakeSupportedResourcesGetter{Testing: t}, FakeK8sClients())
// 			require.Nil(t, err)
// 			tt.setup(&cronjob[0].Object)
// 			dynamicClient := dynamicFake.NewSimpleDynamicClient(scheme)
// 			err = cronJobAutoCreate(dynamicClient, &cronjob[0].Object)
// 			require.Nil(t, err)
// 			list, err := dynamicClient.Resource(gvrJobs).
// 				Namespace("default").List(context.Background(), metav1.ListOptions{})
// 			require.Nil(t, err)
// 			require.Equal(t, tt.expected, len(list.Items))
// 		})
// 	}
// }

// func TestCreateJobFromCronJob(t *testing.T) {
// 	_, cron, err := NewResourcesFromFile("testdata/apply/cronjob-test.cronjob.yml", "default", FakeSupportedResourcesGetter{Testing: t}, FakeK8sClients())
// 	require.Nil(t, err)
// 	expected := map[string]interface{}{"apiVersion": "batch/v1", "kind": "Job", "metadata": map[string]interface{}{"annotations": map[string]interface{}{"cronjob.kubernetes.io/instantiate": "manual"}, "creationTimestamp": interface{}(nil), "generateName": "hello-", "namespace": "default"}, "spec": map[string]interface{}{"template": map[string]interface{}{"metadata": map[string]interface{}{"creationTimestamp": interface{}(nil)}, "spec": map[string]interface{}{"containers": []interface{}{map[string]interface{}{"args": []interface{}{"/bin/sh", "-c", "date; sleep 120"}, "image": "busybox", "name": "hello", "resources": map[string]interface{}{}}}, "restartPolicy": "OnFailure"}}}, "status": map[string]interface{}{}}

// 	scheme := runtime.NewScheme()
// 	_ = corev1.AddToScheme(scheme)
// 	_ = batchv1.AddToScheme(scheme)

// 	dynamicClient := dynamicFake.NewSimpleDynamicClient(scheme)

// 	jobName, err := createJobFromCronjob(dynamicClient, &cron[0].Object)
// 	require.Nil(t, err)
// 	actual, err := dynamicClient.Resource(gvrJobs).
// 		Namespace("default").
// 		Get(context.Background(), jobName, metav1.GetOptions{})
// 	require.Nil(t, err)
// 	require.Equal(t, expected, actual.Object)
// }

func TestCreatePatch(t *testing.T) {
	createDeployment := func(annotation string, lastApplied *Resource) *Resource {
		t.Helper()
		_, deployments, err := NewResourcesFromFile("testdata/apply/test-deployment.yaml")
		require.Nil(t, err)

		deployment := deployments[0]

		if annotation != "" {
			annotations := deployment.Object.GetAnnotations()
			if annotations == nil {
				annotations = make(map[string]string)
			}
			annotations[annotation] = "value"
			deployment.Object.SetAnnotations(annotations)
		}

		if lastApplied != nil {
			annotations := deployment.Object.GetAnnotations()
			if annotations == nil {
				annotations = make(map[string]string)
			}

			lastAppliedJSON, err := lastApplied.Object.MarshalJSON()
			require.Nil(t, err)

			annotations[corev1.LastAppliedConfigAnnotation] = string(lastAppliedJSON)
			deployment.Object.SetAnnotations(annotations)
		}

		return &deployment
	}

	deployment := createDeployment("", nil)

	deploymentWithLastApplied := createDeployment("", deployment)

	deploymentWith2Replicas := createDeployment("", nil)
	err := unstructured.SetNestedField(deploymentWith2Replicas.Object.Object, "2", "spec", "replicas")
	require.Nil(t, err)

	deploymentWithDifferentAnnotationValue := createDeployment("current", createDeployment("current", nil))
	annotations := deploymentWithDifferentAnnotationValue.Object.GetAnnotations()
	annotations["current"] = "other"
	deploymentWithDifferentAnnotationValue.Object.SetAnnotations(annotations)

	testCases := []struct {
		desc     string
		current  *unstructured.Unstructured
		target   *Resource
		expected string
	}{
		{
			desc:     "Pass the same object should produce empty patch",
			current:  &deploymentWithLastApplied.Object,
			target:   deployment,
			expected: "{}",
		}, {
			desc:     "Change replicas",
			current:  &deploymentWithLastApplied.Object,
			target:   deploymentWith2Replicas,
			expected: "{\"metadata\":{\"annotations\":{\"kubectl.kubernetes.io/last-applied-configuration\":\"{\\\"apiVersion\\\":\\\"apps/v1\\\",\\\"kind\\\":\\\"Deployment\\\",\\\"metadata\\\":{\\\"creationTimestamp\\\":null,\\\"labels\\\":{\\\"app\\\":\\\"test-deployment\\\"},\\\"name\\\":\\\"test-deployment\\\",\\\"namespace\\\":\\\"default\\\"},\\\"spec\\\":{\\\"replicas\\\":\\\"2\\\",\\\"selector\\\":{\\\"matchLabels\\\":{\\\"app\\\":\\\"test-deployment\\\"}},\\\"strategy\\\":{},\\\"template\\\":{\\\"metadata\\\":{\\\"creationTimestamp\\\":null,\\\"labels\\\":{\\\"app\\\":\\\"test-deployment\\\"}},\\\"spec\\\":{\\\"containers\\\":[{\\\"image\\\":\\\"nginx\\\",\\\"name\\\":\\\"nginx\\\",\\\"resources\\\":{}}]}}},\\\"status\\\":{}}\\n\"}},\"spec\":{\"replicas\":\"2\"}}",
		}, {
			desc:     "Keep annotation if present in current but not in last applied",
			current:  &createDeployment("current", deployment).Object,
			target:   deployment,
			expected: "{}",
		}, {
			desc:     "Delete annotation if present in last applied but not in target",
			current:  &createDeployment("current", createDeployment("current", nil)).Object,
			target:   deployment,
			expected: "{\"metadata\":{\"annotations\":{\"current\":null,\"kubectl.kubernetes.io/last-applied-configuration\":\"{\\\"apiVersion\\\":\\\"apps/v1\\\",\\\"kind\\\":\\\"Deployment\\\",\\\"metadata\\\":{\\\"creationTimestamp\\\":null,\\\"labels\\\":{\\\"app\\\":\\\"test-deployment\\\"},\\\"name\\\":\\\"test-deployment\\\",\\\"namespace\\\":\\\"default\\\"},\\\"spec\\\":{\\\"replicas\\\":1,\\\"selector\\\":{\\\"matchLabels\\\":{\\\"app\\\":\\\"test-deployment\\\"}},\\\"strategy\\\":{},\\\"template\\\":{\\\"metadata\\\":{\\\"creationTimestamp\\\":null,\\\"labels\\\":{\\\"app\\\":\\\"test-deployment\\\"}},\\\"spec\\\":{\\\"containers\\\":[{\\\"image\\\":\\\"nginx\\\",\\\"name\\\":\\\"nginx\\\",\\\"resources\\\":{}}]}}},\\\"status\\\":{}}\\n\"}}}",
		}, {
			desc:     "Target has priority on current",
			current:  &deploymentWithDifferentAnnotationValue.Object,
			target:   createDeployment("current", nil),
			expected: "{\"metadata\":{\"annotations\":{\"current\":\"value\"}}}",
		},
	}
	for _, tC := range testCases {
		t.Run(tC.desc, func(t *testing.T) {
			patch, patchType, err := createPatch(*tC.current, *tC.target)

			require.Equal(t, tC.expected, string(patch))
			require.Equal(t, types.StrategicMergePatchType, patchType)
			require.Nil(t, err)
		})
	}
}
