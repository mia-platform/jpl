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
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

	"github.com/mia-platform/jpl/pkg/client/cache"
)

// Interface defines the interface for a generator that can create new resources based on another
type Interface interface {
	// CanHandleResource will be called to check if calling the Generate function
	CanHandleResource(*metav1.PartialObjectMetadata) bool
	// Generate receive a resource and return an array of new resources or an error
	Generate(*unstructured.Unstructured, cache.RemoteResourceGetter) ([]*unstructured.Unstructured, error)
}
