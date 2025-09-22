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

package generator

import (
	"path/filepath"
	"testing"

	pkgtesting "github.com/mia-platform/jpl/pkg/testing"
	"github.com/stretchr/testify/assert"
	batchv1 "k8s.io/api/batch/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

func TestCanHandleResource(t *testing.T) {
	t.Parallel()

	annotation := "example.com/annotation"
	value := "true"
	generator := NewJobGenerator(annotation, value)

	testCases := map[string]struct {
		metadata       *metav1.PartialObjectMetadata
		expectedResult bool
	}{
		"non cronjob is false": {
			metadata: &metav1.PartialObjectMetadata{
				TypeMeta: metav1.TypeMeta{
					Kind:       "Deployment",
					APIVersion: "apps/v1",
				},
			},
		},
		"non batch group cronjob is false": {
			metadata: &metav1.PartialObjectMetadata{
				TypeMeta: metav1.TypeMeta{
					Kind:       "CronJob",
					APIVersion: "example.com/v1",
				},
			},
		},
		"batch group job is false": {
			metadata: &metav1.PartialObjectMetadata{
				TypeMeta: metav1.TypeMeta{
					Kind:       "Job",
					APIVersion: batchv1.SchemeGroupVersion.String(),
				},
			},
		},
		"batch group cronjob without annotations is false": {
			metadata: &metav1.PartialObjectMetadata{
				TypeMeta: metav1.TypeMeta{
					Kind:       "CronJob",
					APIVersion: batchv1.SchemeGroupVersion.String(),
				},
			},
		},
		"batch group cronjob with annotations but wrong value is false": {
			metadata: &metav1.PartialObjectMetadata{
				TypeMeta: metav1.TypeMeta{
					Kind:       "CronJob",
					APIVersion: batchv1.SchemeGroupVersion.String(),
				},
				ObjectMeta: metav1.ObjectMeta{
					Annotations: map[string]string{
						annotation: "wrongvalue",
					},
				},
			},
		},
		"batch group cronjob with other annotations is false": {
			metadata: &metav1.PartialObjectMetadata{
				TypeMeta: metav1.TypeMeta{
					Kind:       "CronJob",
					APIVersion: batchv1.SchemeGroupVersion.String(),
				},
				ObjectMeta: metav1.ObjectMeta{
					Annotations: map[string]string{
						"example.com/wrong": value,
						value:               annotation,
					},
				},
			},
		},
		"batch group cronjob with annotation is true": {
			metadata: &metav1.PartialObjectMetadata{
				TypeMeta: metav1.TypeMeta{
					Kind:       "CronJob",
					APIVersion: batchv1.SchemeGroupVersion.String(),
				},
				ObjectMeta: metav1.ObjectMeta{
					Annotations: map[string]string{
						"example.com/wrong": value,
						value:               annotation,
						annotation:          value,
					},
				},
			},
			expectedResult: true,
		},
	}

	for testName, testCase := range testCases {
		t.Run(testName, func(t *testing.T) {
			t.Parallel()
			assert.Equal(t, testCase.expectedResult, generator.CanHandleResource(testCase.metadata))
		})
	}
}

func TestGenerate(t *testing.T) {
	t.Parallel()

	generator := NewJobGenerator("example.com/annotation", "true")
	testdata := "testdata"

	testCases := map[string]struct {
		object    *unstructured.Unstructured
		expectErr bool
	}{
		"return error if object cannot be parsed in CronJob": {
			object:    pkgtesting.UnstructuredFromFile(t, filepath.Join(testdata, "deployment.yaml")),
			expectErr: true,
		},
		"return Job from CronJob": {
			object: pkgtesting.UnstructuredFromFile(t, filepath.Join(testdata, "cronjob.yaml")),
		},
	}

	for testName, testCase := range testCases {
		t.Run(testName, func(t *testing.T) {
			t.Parallel()

			jobs, err := generator.Generate(testCase.object, nil)
			if testCase.expectErr {
				assert.Error(t, err)
				assert.ErrorContains(t, err, "strict decoding error")
				assert.Empty(t, jobs)
				return
			}

			assert.NoError(t, err)
			assert.Len(t, jobs, 1)
		})
	}
}
