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
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/apis/meta/v1/validation"
	"k8s.io/apimachinery/pkg/util/validation/field"

	"github.com/mia-platform/jpl/pkg/client/cache"
)

// NewLabelsMutator return a new mutateor.Interface that will integrate the objects labels with the ones provided
// during initialization
func NewLabelsMutator(labels map[string]string) Interface {
	return &labelsMutator{
		labels: labels,
	}
}

// keep it to always check if labelsMutator implement correctly the mutator.Interface interface
var _ Interface = &labelsMutator{}

type labelsMutator struct {
	labels map[string]string
}

// CanHandleResource implement mutator.Interface interface
func (m *labelsMutator) CanHandleResource(*metav1.PartialObjectMetadata) bool {
	return len(m.labels) > 0
}

// Mutate implement mutator.Interface interface
func (m *labelsMutator) Mutate(obj *unstructured.Unstructured, _ cache.RemoteResourceGetter) error {
	if errs := validation.ValidateLabels(m.labels, field.NewPath("labels")); len(errs) != 0 {
		return errs.ToAggregate()
	}

	labels := obj.GetLabels()
	if labels == nil {
		labels = make(map[string]string)
	}

	for key, value := range m.labels {
		if _, found := labels[key]; !found {
			labels[key] = value
		}
	}

	obj.SetLabels(labels)
	return nil
}
