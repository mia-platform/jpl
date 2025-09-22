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

package mutator

import (
	"path/filepath"
	"testing"

	pkgtesting "github.com/mia-platform/jpl/pkg/testing"
	"github.com/stretchr/testify/assert"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

func TestCanHandleResource(t *testing.T) {
	t.Parallel()

	assert.False(t, NewLabelsMutator(nil).CanHandleResource(nil))
	assert.True(t, NewLabelsMutator(map[string]string{"foo": "foo"}).CanHandleResource(nil))
}

func TestLabelsMutator(t *testing.T) {
	t.Parallel()

	testdata := "testdata"
	validLabels := map[string]string{
		"foo": "foo",
		"bar": "bar",
	}

	invalidLabels := map[string]string{
		"invalidsimbol@": "value",
		"invalidValue":   "bad,value",
	}

	tests := map[string]struct {
		labels         map[string]string
		obj            *unstructured.Unstructured
		expectedLabels map[string]string
		expectedError  bool
	}{
		"mutate label without already present labels": {
			labels:         validLabels,
			obj:            pkgtesting.UnstructuredFromFile(t, filepath.Join(testdata, "without-labels.yaml")),
			expectedLabels: validLabels,
		},
		"mutate label with already present labels": {
			labels: validLabels,
			obj:    pkgtesting.UnstructuredFromFile(t, filepath.Join(testdata, "with-labels.yaml")),
			expectedLabels: map[string]string{
				"foo": "alreadyhere",
				"bar": "bar",
			},
		},
		"error mutating labels without already present labels": {
			labels:        invalidLabels,
			obj:           pkgtesting.UnstructuredFromFile(t, filepath.Join(testdata, "without-labels.yaml")),
			expectedError: true,
		},
		"error mutating labels wit already present labels": {
			labels: invalidLabels,
			obj:    pkgtesting.UnstructuredFromFile(t, filepath.Join(testdata, "with-labels.yaml")),
			expectedLabels: map[string]string{
				"foo": "alreadyhere",
			},
			expectedError: true,
		},
	}

	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			mutator := NewLabelsMutator(test.labels)
			err := mutator.Mutate(test.obj, nil)
			switch test.expectedError {
			case true:
				assert.Error(t, err)
			default:
				assert.NoError(t, err)
			}

			assert.Equal(t, test.expectedLabels, test.obj.GetLabels())
		})
	}
}
