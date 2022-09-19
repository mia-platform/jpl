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

package deploy

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	appsv1 "k8s.io/api/apps/v1"
	batchv1 "k8s.io/api/batch/v1"
	batchv1beta1 "k8s.io/api/batch/v1beta1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	dynamicFake "k8s.io/client-go/dynamic/fake"
)

func TestCheckIfCreateJob(t *testing.T) {
	cronjob, err := NewResources("testdata/apply/cronjob-test.cronjob.yml", "default")
	require.Nil(t, err)
	scheme := runtime.NewScheme()
	_ = corev1.AddToScheme(scheme)
	_ = batchv1.AddToScheme(scheme)

	t.Run("without last-applied", func(t *testing.T) {
		dynamicClient := dynamicFake.NewSimpleDynamicClient(scheme)
		err := checkIfCreateJob(dynamicClient, &cronjob[0].Object, cronjob[0])
		require.Nil(t, err)
	})
	t.Run("same last-applied", func(t *testing.T) {
		cronjob[0].Object.SetAnnotations(map[string]string{
			"kubectl.kubernetes.io/last-applied-configuration": "{\"apiVersion\":\"batch/v1beta1\",\"kind\":\"CronJob\",\"metadata\":{\"annotations\":{\"mia-platform.eu/autocreate\":\"true\"},\"name\":\"hello\",\"namespace\":\"default\"},\"spec\":{\"jobTemplate\":{\"spec\":{\"template\":{\"spec\":{\"containers\":[{\"args\":[\"/bin/sh\",\"-c\",\"date; sleep 120\"],\"image\":\"busybox\",\"name\":\"hello\"}],\"restartPolicy\":\"OnFailure\"}}}},\"schedule\":\"*/5 * * * *\"}}\n",
			"mia-platform.eu/autocreate":                       "true",
		})
		dynamicClient := dynamicFake.NewSimpleDynamicClient(scheme)
		err = checkIfCreateJob(dynamicClient, &cronjob[0].Object, cronjob[0])
		require.Nil(t, err)
	})
	t.Run("different last-applied", func(t *testing.T) {
		obj := cronjob[0].Object.DeepCopy()
		obj.SetAnnotations(map[string]string{
			"kubectl.kubernetes.io/last-applied-configuration": "{\"apiVersion\":\"batch/v1beta1\",\"kind\":\"CronJob\",\"metadata\":{\"annotations\":{\"mia-platform.eu/autocreate\":\"true\"},\"name\":\"hello\",\"namespace\":\"default\"},\"spec\":{\"jobTemplate\":{\"spec\":{\"template\":{\"spec\":{\"containers\":[{\"args\":[\"/bin/sh\",\"-c\",\"date; sleep 2\"],\"image\":\"busybox\",\"name\":\"hello\"}],\"restartPolicy\":\"OnFailure\"}}}},\"schedule\":\"*/5 * * * *\"}}\n",
		})
		dynamicClient := dynamicFake.NewSimpleDynamicClient(scheme)
		err = checkIfCreateJob(dynamicClient, obj, cronjob[0])
		require.Nil(t, err)
		list, err := dynamicClient.Resource(gvrJobs).
			Namespace("default").List(context.Background(), metav1.ListOptions{})
		require.Nil(t, err)
		require.Equal(t, 1, len(list.Items))
	})
}

func TestCronJobAutoCreate(t *testing.T) {
	scheme := runtime.NewScheme()
	_ = corev1.AddToScheme(scheme)
	_ = batchv1.AddToScheme(scheme)

	testcases := []struct {
		description string
		setup       func(obj *unstructured.Unstructured)
		expected    int
	}{
		{
			description: "autocreate true",
			expected:    1,
			setup: func(obj *unstructured.Unstructured) {
				obj.SetAnnotations(map[string]string{
					"mia-platform.eu/autocreate": "true",
				})
			},
		},
		{
			description: "autocreate false",
			expected:    0,
			setup: func(obj *unstructured.Unstructured) {
				obj.SetAnnotations(map[string]string{
					"mia-platform.eu/autocreate": "false",
				})
			},
		},
		{
			description: "no annotation",
			expected:    0,
			setup: func(obj *unstructured.Unstructured) {
				obj.SetAnnotations(map[string]string{})
			},
		},
	}

	for _, tt := range testcases {
		t.Run(tt.description, func(t *testing.T) {
			cronjob, err := NewResources("testdata/apply/cronjob-test.cronjob.yml", "default")
			require.Nil(t, err)
			tt.setup(&cronjob[0].Object)
			dynamicClient := dynamicFake.NewSimpleDynamicClient(scheme)
			err = cronJobAutoCreate(dynamicClient, &cronjob[0].Object)
			require.Nil(t, err)
			list, err := dynamicClient.Resource(gvrJobs).
				Namespace("default").List(context.Background(), metav1.ListOptions{})
			require.Nil(t, err)
			require.Equal(t, tt.expected, len(list.Items))
		})
	}
}

func TestCreateJobFromCronJob(t *testing.T) {
	cron, err := NewResources("testdata/apply/cronjob-test.cronjob.yml", "default")
	require.Nil(t, err)
	expected := map[string]interface{}{"apiVersion": "batch/v1", "kind": "Job", "metadata": map[string]interface{}{"annotations": map[string]interface{}{"cronjob.kubernetes.io/instantiate": "manual"}, "creationTimestamp": interface{}(nil), "generateName": "hello-", "namespace": "default"}, "spec": map[string]interface{}{"template": map[string]interface{}{"metadata": map[string]interface{}{"creationTimestamp": interface{}(nil)}, "spec": map[string]interface{}{"containers": []interface{}{map[string]interface{}{"args": []interface{}{"/bin/sh", "-c", "date; sleep 120"}, "image": "busybox", "name": "hello", "resources": map[string]interface{}{}}}, "restartPolicy": "OnFailure"}}}, "status": map[string]interface{}{}}

	scheme := runtime.NewScheme()
	_ = corev1.AddToScheme(scheme)
	_ = batchv1.AddToScheme(scheme)

	dynamicClient := dynamicFake.NewSimpleDynamicClient(scheme)

	jobName, err := createJobFromCronjob(dynamicClient, &cron[0].Object)
	require.Nil(t, err)
	actual, err := dynamicClient.Resource(gvrJobs).
		Namespace("default").
		Get(context.Background(), jobName, metav1.GetOptions{})
	require.Nil(t, err)
	require.Equal(t, expected, actual.Object)
}

func TestCreatePatch(t *testing.T) {
	createDeployment := func(annotation string, lastApplied *Resource) *Resource {
		t.Helper()
		deployments, err := NewResources("testdata/apply/test-deployment.yaml", "default")
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

			lastAppliedJson, err := lastApplied.Object.MarshalJSON()
			require.Nil(t, err)

			annotations[corev1.LastAppliedConfigAnnotation] = string(lastAppliedJson)
			deployment.Object.SetAnnotations(annotations)
		}

		return &deployment
	}

	deployment := createDeployment("", nil)

	deploymentWithLastApplied := createDeployment("", deployment)

	deploymentWith2Replicas := createDeployment("", nil)
	unstructured.SetNestedField(deploymentWith2Replicas.Object.Object, "2", "spec", "replicas")

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
			require.Equal(t, patchType, types.StrategicMergePatchType)
			require.Nil(t, err)
		})
	}
}

func TestEnsureDeployAll(t *testing.T) {

	mockTime := time.Date(2009, time.November, 10, 23, 0, 0, 0, time.UTC)
	expectedCheckSum := "6ab733c74e26e73bca78aa9c4c9db62664f339d9eefac51dd503c9ff0cf0c329"

	t.Run("Add deployment annotation", func(t *testing.T) {
		deployment, err := NewResources("testdata/apply/test-deployment.yaml", "default")
		require.Nil(t, err)
		err = ensureDeployAll(&deployment[0], mockTime)
		require.Nil(t, err)

		var dep appsv1.Deployment
		err = runtime.DefaultUnstructuredConverter.
			FromUnstructured(deployment[0].Object.Object, &dep)
		require.Nil(t, err)
		require.Equal(t, map[string]string{GetMiaAnnotation(deployChecksum): expectedCheckSum}, dep.Spec.Template.ObjectMeta.Annotations)
	})

	t.Run("Add cronJob annotation", func(t *testing.T) {
		cronJob, err := NewResources("testdata/apply/cronjob-test.cronjob.yml", "default")
		require.Nil(t, err)

		err = ensureDeployAll(&cronJob[0], mockTime)
		require.Nil(t, err)
		var cronj batchv1beta1.CronJob
		err = runtime.DefaultUnstructuredConverter.
			FromUnstructured(cronJob[0].Object.Object, &cronj)
		require.Nil(t, err)
		require.Equal(t, map[string]string{GetMiaAnnotation(deployChecksum): expectedCheckSum}, cronj.Spec.JobTemplate.Spec.Template.ObjectMeta.Annotations)
	})
	t.Run("Keep existing annotations", func(t *testing.T) {
		// testing only deployment because annotation accessing method is the same
		deployment, err := NewResources("testdata/apply/test-deployment.yaml", "default")
		require.Nil(t, err)
		unstructured.SetNestedStringMap(deployment[0].Object.Object, map[string]string{
			"existing-key": "value1",
		},
			"spec", "template", "metadata", "annotations")
		err = ensureDeployAll(&deployment[0], mockTime)
		require.Nil(t, err)
		var dep appsv1.Deployment
		err = runtime.DefaultUnstructuredConverter.
			FromUnstructured(deployment[0].Object.Object, &dep)
		require.Nil(t, err)
		require.Equal(t, map[string]string{
			GetMiaAnnotation(deployChecksum): expectedCheckSum,
			"existing-key":                   "value1",
		}, dep.Spec.Template.ObjectMeta.Annotations)
	})
}

func TestEnsureSmartDeploy(t *testing.T) {
	expectedCheckSum := "6ab733c74e26e73bca78aa9c4c9db62664f339d9eefac51dd503c9ff0cf0c329"

	t.Run("Add deployment deploy/checksum annotation", func(t *testing.T) {
		targetObject, err := NewResources("testdata/apply/test-deployment.yaml", "default")
		require.Nil(t, err)
		currentObj := targetObject[0].Object.DeepCopy()
		unstructured.SetNestedStringMap(currentObj.Object, map[string]string{
			"mia-platform.eu/deploy-checksum": expectedCheckSum,
			"test":                            "test",
		}, "spec", "template", "metadata", "annotations")
		t.Logf("targetObj: %s\n", currentObj.Object)
		err = ensureSmartDeploy(currentObj, &targetObject[0])
		require.Nil(t, err)
		targetAnn, _, err := unstructured.NestedStringMap(targetObject[0].Object.Object,
			"spec", "template", "metadata", "annotations")
		require.Nil(t, err)
		require.Equal(t, targetAnn["mia-platform.eu/deploy-checksum"], expectedCheckSum)
	})

	t.Run("Add deployment without deploy/checksum annotation", func(t *testing.T) {
		targetObject, err := NewResources("testdata/apply/test-deployment.yaml", "default")
		require.Nil(t, err)
		currentObj := targetObject[0].Object.DeepCopy()
		err =
			unstructured.SetNestedStringMap(targetObject[0].Object.Object, map[string]string{
				"test": "test",
			}, "spec", "template", "annotations")
		require.Nil(t, err)

		err = ensureSmartDeploy(currentObj, &targetObject[0])
		require.Nil(t, err)

		targetAnn, _, err := unstructured.NestedStringMap(targetObject[0].Object.Object,
			"spec", "template", "annotations")
		require.Nil(t, err)
		require.Equal(t, "test", targetAnn["test"])
	})

	t.Run("Add cronjob deploy/checksum annotation", func(t *testing.T) {
		targetObject, err := NewResources("testdata/apply/cronjob-test.cronjob.yml", "default")
		require.Nil(t, err)
		currentObj := targetObject[0].Object.DeepCopy()
		unstructured.SetNestedStringMap(currentObj.Object, map[string]string{
			"mia-platform.eu/deploy-checksum": expectedCheckSum,
			"test":                            "test",
		}, "spec", "jobTemplate", "spec", "template", "metadata", "annotations")
		t.Logf("targetObj: %s\n", currentObj.Object)
		err = ensureSmartDeploy(currentObj, &targetObject[0])

		require.Nil(t, err)
		targetAnn, _, err := unstructured.NestedStringMap(targetObject[0].Object.Object,
			"spec", "jobTemplate", "spec", "template", "metadata", "annotations")
		require.Nil(t, err)
		require.Equal(t, targetAnn["mia-platform.eu/deploy-checksum"], expectedCheckSum)
	})
}
