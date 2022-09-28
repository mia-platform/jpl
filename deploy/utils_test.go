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
	"path/filepath"
	"reflect"
	"testing"

	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	dynamicFake "k8s.io/client-go/dynamic/fake"
)

const testdata = "testdata/utils/"

func TestNewResources(t *testing.T) {
	t.Run("Read a valid kubernetes resource", func(t *testing.T) {
		filePath := filepath.Join(testdata, "kubernetesersource.yaml")
		actual, err := NewResourcesFromFile(filePath, "default")
		require.Nil(t, err)
		expected := map[string]interface{}{"apiVersion": "v1", "data": map[string]interface{}{"dueKey": "deuValue", "unaKey": "unValue"}, "kind": "ConfigMap", "metadata": map[string]interface{}{"name": "literal", "namespace": "default"}}
		require.Nil(t, err, "Reading a valid k8s file err must be nil")
		require.Equal(t, len(actual), 1, "1 Resource")
		require.Equal(t, actual[0].GroupVersionKind.Kind, "ConfigMap")
		require.EqualValues(t, expected, actual[0].Object.Object, "confimap on disk different")
	})
	t.Run("Read 2 valid kubernetes resource", func(t *testing.T) {
		filePath := filepath.Join(testdata, "tworesources.yaml")
		actual, err := NewResourcesFromFile(filePath, "default")
		expected1 := map[string]interface{}{"apiVersion": "v1", "data": map[string]interface{}{"dueKey": "deuValue", "unaKey": "unValue"}, "kind": "ConfigMap", "metadata": map[string]interface{}{"name": "literal", "namespace": "default"}}
		expected2 := map[string]interface{}{"apiVersion": "v1", "data": map[string]interface{}{"dueKey": "deuValue2", "unaKey": "unValue2"}, "kind": "ConfigMap", "metadata": map[string]interface{}{"name": "literal2", "namespace": "default"}}
		require.Nil(t, err, "Reading two valid k8s file err must be nil")
		require.Equal(t, len(actual), 2, "2 Resource")
		require.Equal(t, actual[0].GroupVersionKind.Kind, "ConfigMap")
		require.Equal(t, actual[1].GroupVersionKind.Kind, "ConfigMap")
		require.EqualValues(t, expected1, actual[0].Object.Object, "confimap 1 on disk different")
		require.EqualValues(t, expected2, actual[1].Object.Object, "confimap 2 on disk different")
	})
	t.Run("Read not standard resource", func(t *testing.T) {
		filePath := filepath.Join(testdata, "non-standard-resource.yaml")
		actual, err := NewResourcesFromFile(filePath, "default")
		expected := map[string]interface{}{"apiVersion": "traefik.containo.us/v1alpha1", "kind": "IngressRoute", "metadata": map[string]interface{}{"name": "ingressroute1", "namespace": "default"}, "spec": map[string]interface{}{"entryPoints": []interface{}{"websecure"}, "routes": []interface{}{}}}
		require.Nil(t, err, "Reading non standard k8s file err must be nil")
		require.Equal(t, len(actual), 1, "1 Resource")
		require.EqualValues(t, expected, actual[0].Object.Object, "even a crd is unstructurable")
	})
	t.Run("Read an invalid kubernetes resource", func(t *testing.T) {
		filePath := filepath.Join(testdata, "invalidresource.yaml")
		_, err := NewResourcesFromFile(filePath, "default")
		require.EqualError(t, err, "error converting YAML to JSON: yaml: line 3: could not find expected ':'")
	})
}

func TestMakeResources(t *testing.T) {
	testCases := []struct {
		desc       string
		inputFiles []string
		expected   int
	}{
		{
			desc:       "3 valid resources in 2 files",
			inputFiles: []string{"kubernetesersource.yaml", "tworesources.yaml"},
			expected:   3,
		},
		{
			desc:       "resource with ---",
			inputFiles: []string{"configmap-with-minus.yaml"},
			expected:   1,
		},
	}
	for _, tC := range testCases {
		t.Run(tC.desc, func(t *testing.T) {
			var filePath []string
			for _, v := range tC.inputFiles {
				filePath = append(filePath, filepath.Join(testdata, v))
			}
			actual, err := MakeResources(filePath, "default")
			require.Nil(t, err)
			require.Equal(t, tC.expected, len(actual))
		})
	}
}

func TestIsNotUsingSemver(t *testing.T) {
	testcases := []struct {
		description string
		input       []interface{}
		expected    bool
	}{
		{

			description: "following semver",
			input:       []interface{}{map[string]interface{}{"image": "test:1.0.0"}},
			expected:    false,
		},
		{
			description: "not following semver",
			input:       []interface{}{map[string]interface{}{"image": "test:latest"}},
			expected:    true,
		},
		{
			description: "all following semver",
			input: []interface{}{map[string]interface{}{"image": "test:1.0.0"},
				map[string]interface{}{"image": "test:1.0.0-alpha"},
				map[string]interface{}{"image": "test:1.0.0+20130313144700"},
				map[string]interface{}{"image": "test:1.0.0-beta+exp.sha.5114f85"}},
			expected: false,
		},
		{
			description: "one not following semver",
			input: []interface{}{map[string]interface{}{"image": "test:1.0.0"},
				map[string]interface{}{"image": "test:1.0.0-alpha"},
				map[string]interface{}{"image": "test:1.0.0+20130313144700"},
				map[string]interface{}{"image": "test:tag1"},
			},
			expected: true,
		},
	}

	for _, tt := range testcases {
		types := []struct {
			typ            string
			path           string
			containersPath []string
		}{
			{
				typ:            "deployments",
				path:           "testdata/apply/test-deployment.yaml",
				containersPath: []string{"spec", "template", "spec", "containers"},
			},
			{
				typ:            "cronjobs",
				path:           "testdata/apply/cronjob-test.cronjob.yml",
				containersPath: []string{"spec", "jobTemplate", "spec", "template", "spec", "containers"},
			},
		}
		for _, typ := range types {
			t.Run(fmt.Sprintf("%s - %s", typ.typ, tt.description), func(t *testing.T) {
				targetObject, err := NewResourcesFromFile(typ.path, "default")
				require.Nil(t, err)
				err = unstructured.SetNestedField(targetObject[0].Object.Object, tt.input, typ.containersPath...)
				require.Nil(t, err)
				boolRes, err := IsNotUsingSemver(&targetObject[0])
				require.Nil(t, err)
				require.Equal(t, tt.expected, boolRes)
			})
		}
	}
}

func TestUpdateResourceSecret(t *testing.T) {
	expected := corev1.Secret{
		Data: map[string][]byte{"resources": []byte(`{"CronJob":{"kind":{"Group":"batch","Version":"v1beta1","Kind":"CronJob"},"resources":["bar"]}}`)},
		ObjectMeta: metav1.ObjectMeta{
			Name:      resourceSecretName,
			Namespace: "foo",
		},
		TypeMeta: metav1.TypeMeta{Kind: "Secret", APIVersion: "v1"},
	}

	resources := map[string]*ResourceList{
		"CronJob": {
			Kind:      &schema.GroupVersionKind{Group: "batch", Version: "v1beta1", Kind: "CronJob"},
			Resources: []string{"bar"},
		},
	}
	scheme := runtime.NewScheme()
	_ = corev1.AddToScheme(scheme)

	t.Run("Create resource-deployed secret for the first time", func(t *testing.T) {
		dynamicClient := dynamicFake.NewSimpleDynamicClient(scheme)
		err := updateResourceSecret(dynamicClient, "foo", resources)
		require.Nil(t, err)
		var actual corev1.Secret
		expUnstr, err := dynamicClient.Resource(gvrSecrets).
			Namespace("foo").
			Get(context.Background(), resourceSecretName, metav1.GetOptions{})
		require.Nil(t, err)
		err = runtime.DefaultUnstructuredConverter.FromUnstructured(expUnstr.Object, &actual)
		require.Nil(t, err)
		require.Equal(t, string(expected.Data["resources"]), string(actual.Data["resources"]))
	})
	t.Run("Update resource-deployed", func(t *testing.T) {
		existingSecret := &corev1.Secret{
			Data: map[string][]byte{"resources": []byte(`{"CronJob":{"kind":{"Group":"batch","Version":"v1beta1","Kind":"CronJob"},"resources":["foo", "sss"]}}`)},
			ObjectMeta: metav1.ObjectMeta{
				Name:      resourceSecretName,
				Namespace: "foo",
			},
			TypeMeta: metav1.TypeMeta{Kind: "Secret", APIVersion: "v1"},
		}
		dynamicClient := dynamicFake.NewSimpleDynamicClient(scheme, existingSecret)

		err := updateResourceSecret(dynamicClient, "foo", resources)
		require.Nil(t, err)
		var actual corev1.Secret
		expUnstr, err := dynamicClient.Resource(gvrSecrets).
			Namespace("foo").
			Get(context.Background(), resourceSecretName, metav1.GetOptions{})
		require.Nil(t, err)
		err = runtime.DefaultUnstructuredConverter.FromUnstructured(expUnstr.Object, &actual)
		require.Nil(t, err)
		require.Equal(t, string(expected.Data["resources"]), string(actual.Data["resources"]))
	})
}

func TestMakeResourceMap(t *testing.T) {
	gvkSecret := schema.GroupVersionKind{Group: "", Version: "v1", Kind: "Secret"}
	gvkCm := schema.GroupVersionKind{Group: "", Version: "v1", Kind: "ConfigMap"}

	testcases := []struct {
		description string
		input       []Resource
		expected    map[string]*ResourceList
	}{
		{
			description: "All secrets",
			input: []Resource{
				{
					Object:           unstructured.Unstructured{Object: map[string]interface{}{"apiVersion": "v1", "metadata": map[string]interface{}{"name": "secret1"}}},
					GroupVersionKind: &gvkSecret,
				},
				{
					Object:           unstructured.Unstructured{Object: map[string]interface{}{"apiVersion": "v1", "metadata": map[string]interface{}{"name": "secret2"}}},
					GroupVersionKind: &gvkSecret,
				},
			},
			expected: map[string]*ResourceList{"Secret": {
				Kind:      &gvkSecret,
				Resources: []string{"secret1", "secret2"},
			}},
		},
		{
			description: "1 secret 1 cm",
			input: []Resource{
				{
					Object:           unstructured.Unstructured{Object: map[string]interface{}{"apiVersion": "v1", "metadata": map[string]interface{}{"name": "secret1"}}},
					GroupVersionKind: &gvkSecret,
				},
				{
					Object:           unstructured.Unstructured{Object: map[string]interface{}{"apiVersion": "v1", "metadata": map[string]interface{}{"name": "cm1"}}},
					GroupVersionKind: &gvkCm,
				},
			},
			expected: map[string]*ResourceList{"Secret": {
				Kind:      &gvkSecret,
				Resources: []string{"secret1"},
			},
				"ConfigMap": {
					Kind:      &gvkCm,
					Resources: []string{"cm1"},
				},
			},
		},
	}

	for _, tt := range testcases {
		t.Run(tt.description, func(t *testing.T) {
			actual := makeResourceMap(tt.input)
			require.Equal(t, tt.expected, actual)
		})
	}
}

func TestGetOldResourceMap(t *testing.T) {
	testcases := []struct {
		description string
		input       *corev1.Secret
		expected    map[string]*ResourceList
		error       func(t *testing.T, err error)
	}{
		{
			description: "resources field is unmarshaled correctly",
			input: &corev1.Secret{
				Data: map[string][]byte{"resources": []byte(`{"Secret":{"resources":["foo", "bar"]}, "ConfigMap": {"resources":[]}}`)},
				ObjectMeta: metav1.ObjectMeta{
					Name:      resourceSecretName,
					Namespace: "foo",
				},
				TypeMeta: metav1.TypeMeta{Kind: "Secret", APIVersion: "v1"},
			},
			expected: map[string]*ResourceList{"Secret": {Resources: []string{"foo", "bar"}}, "ConfigMap": {Resources: []string{}}},
			error: func(t *testing.T, err error) {
				require.Nil(t, err)
			},
		},
		{
			description: "resources in in v0 format",
			input: &corev1.Secret{
				Data: map[string][]byte{"resources": []byte(`{"Deployment":{"kind":"Deployment","Mapping":{"Group":"apps","Version":"v1","Resource":"deployments"},"resources":["test-deployment","test-deployment-2"]}}`)},
				ObjectMeta: metav1.ObjectMeta{
					Name:      resourceSecretName,
					Namespace: "foo",
				},
				TypeMeta: metav1.TypeMeta{Kind: "Secret", APIVersion: "v1"},
			},
			expected: map[string]*ResourceList{"Deployment": {Resources: []string{"test-deployment", "test-deployment-2"}, Kind: &schema.GroupVersionKind{Group: "apps", Version: "v1", Kind: "Deployment"}}},
			error: func(t *testing.T, err error) {
				require.Nil(t, err)
			},
		},
		{
			description: "resource field is empty",
			input: &corev1.Secret{
				Data: map[string][]byte{"resources": []byte(`{}`)},
				ObjectMeta: metav1.ObjectMeta{
					Name:      resourceSecretName,
					Namespace: "foo",
				},
				TypeMeta: metav1.TypeMeta{Kind: "Secret", APIVersion: "v1"},
			},
			expected: nil,
			error: func(t *testing.T, err error) {
				require.NotNil(t, err)
				require.EqualError(t, err, "resource field is empty")
			},
		},
		{
			description: "resource field does not contain map[string][]string but map[string]string ",
			input: &corev1.Secret{
				Data: map[string][]byte{"resources": []byte(`{ "foo": "bar" `)},
				ObjectMeta: metav1.ObjectMeta{
					Name:      resourceSecretName,
					Namespace: "foo",
				},
				TypeMeta: metav1.TypeMeta{Kind: "Secret", APIVersion: "v1"},
			},
			expected: nil,
			error: func(t *testing.T, err error) {
				require.NotNil(t, err)
			},
		},
		{
			description: "secret is not found",
			input: &corev1.Secret{
				Data: map[string][]byte{"resources": []byte(`{ "foo": "bar" `)},
				ObjectMeta: metav1.ObjectMeta{
					Name:      "wrong-name",
					Namespace: "foo",
				},
				TypeMeta: metav1.TypeMeta{Kind: "Secret", APIVersion: "v1"},
			},
			expected: map[string]*ResourceList{},
			error: func(t *testing.T, err error) {
				require.Nil(t, err)
			},
		},
	}

	scheme := runtime.NewScheme()
	err := corev1.AddToScheme(scheme)
	require.Nil(t, err)
	for _, tt := range testcases {
		t.Run(tt.description, func(t *testing.T) {
			dynamicClient := dynamicFake.NewSimpleDynamicClient(scheme, tt.input)
			actual, err := getOldResourceMap(&K8sClients{dynamic: dynamicClient}, "foo")
			require.Equal(t, tt.expected, actual)
			tt.error(t, err)
		})
	}
}

func TestDeletedResources(t *testing.T) {
	testcases := []struct {
		description string
		old         map[string]*ResourceList
		new         map[string]*ResourceList
		expected    map[string]*ResourceList
	}{
		{
			description: "No diff with equal maps",
			old:         map[string]*ResourceList{"secrets": {Resources: []string{"foo", "bar"}}},
			new:         map[string]*ResourceList{"secrets": {Resources: []string{"foo", "bar"}}},
			expected:    map[string]*ResourceList{},
		},
		{
			description: "Expected old map if new is empty",
			old:         map[string]*ResourceList{"secrets": {Resources: []string{"foo", "bar"}}},
			new:         map[string]*ResourceList{"secrets": {Resources: []string{}}},
			expected:    map[string]*ResourceList{"secrets": {Resources: []string{"foo", "bar"}}},
		},
		{
			description: "Remove one resource from resourceList",
			old:         map[string]*ResourceList{"secrets": {Resources: []string{"foo", "bar"}}},
			new:         map[string]*ResourceList{"secrets": {Resources: []string{"foo"}}},
			expected:    map[string]*ResourceList{"secrets": {Resources: []string{"bar"}}},
		},
		{
			description: "Add one resource type",
			old:         map[string]*ResourceList{"secrets": {Resources: []string{"foo", "bar"}}},
			new:         map[string]*ResourceList{"secrets": {Resources: []string{"foo"}}, "configmaps": {Resources: []string{"foo"}}},
			expected:    map[string]*ResourceList{"secrets": {Resources: []string{"bar"}}},
		},
		{
			description: "Delete one resource type",
			old:         map[string]*ResourceList{"secrets": {Resources: []string{"foo"}}, "configmaps": {Resources: []string{"foo"}}},
			new:         map[string]*ResourceList{"secrets": {Resources: []string{"foo"}}},
			expected:    map[string]*ResourceList{"configmaps": {Resources: []string{"foo"}}},
		},
	}

	for _, tt := range testcases {
		t.Run(tt.description, func(t *testing.T) {
			actual := deletedResources(tt.new, tt.old)
			require.True(t, reflect.DeepEqual(tt.expected, actual))
		})
	}
}

func TestDiffResourceArray(t *testing.T) {
	testcases := []struct {
		description string
		old         []string
		new         []string
		expected    []string
	}{
		{
			description: "No diff with equal slices",
			old:         []string{"foo", "bar"},
			new:         []string{"foo", "bar"},
			expected:    []string{},
		},
		{
			description: "Expected old array if new is empty",
			old:         []string{"foo", "bar"},
			new:         []string{},
			expected:    []string{"foo", "bar"},
		},
	}

	for _, tt := range testcases {
		t.Run(tt.description, func(t *testing.T) {
			actual := diffResourceArray(tt.new, tt.old)
			require.Equal(t, tt.expected, actual)
		})
	}
}

func TestContains(t *testing.T) {
	testcases := []struct {
		description string
		array       []string
		element     string
		expected    bool
	}{
		{
			description: "the element is contained in the slice",
			array:       []string{"foo", "bar"},
			element:     "foo",
			expected:    true,
		},
		{
			description: "the element is not contained in the slice",
			array:       []string{"foo", "bar"},
			element:     "foobar",
			expected:    false,
		},
		{
			description: "the element is not contained in empty slice",
			array:       []string{},
			element:     "foobar",
			expected:    false,
		},
	}

	for _, tt := range testcases {
		t.Run(tt.description, func(t *testing.T) {
			actual := contains(tt.array, tt.element)
			require.Equal(t, tt.expected, actual)
		})
	}
}

func TestConvertSecretFormat(t *testing.T) {
	oldres := []byte("{\"Deployment\":{\"kind\":\"Deployment\",\"Mapping\":{\"Group\":\"apps\",\"Version\":\"v1\",\"Resource\":\"deployments\"},\"resources\":[\"test-deployment\",\"test-deployment-2\"]}}")
	actual, err := convertSecretFormat(oldres)
	require.Nil(t, err)
	require.Equal(t, []string{"test-deployment", "test-deployment-2"}, actual["Deployment"].Resources)
	require.Equal(t, schema.GroupVersionKind{Group: "apps", Version: "v1", Kind: "Deployment"}, *actual["Deployment"].Kind)
}
