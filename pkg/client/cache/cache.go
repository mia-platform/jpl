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

package cache

import (
	"context"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

	"github.com/mia-platform/jpl/pkg/resource"
)

// RemoteResourceGetter define the interface for a getter that can retrieve the remote status of a resource identified
// by its ObjectMetadata, it will return the remote unstructured reppresentation or nil if it wasn't found.
// An error will be returned only if is not for a not found resource.
type RemoteResourceGetter interface {
	// Get return the remote status of ObjectMetadata resource or an error
	Get(context.Context, resource.ObjectMetadata) (*unstructured.Unstructured, error)
}
