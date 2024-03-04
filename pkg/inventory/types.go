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

package inventory

import (
	"context"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

// ResourceMetadata reppresent the minimum subset of data to uniquily identify a resource deployed on a remote cluster
type ResourceMetadata struct {
	Name      string
	Namespace string
	Group     string
	Kind      string
}

// Store define an interface for working with an inventory of deployed resources, without knowning the underling
// technology that is used for persisting the data
type Store interface {
	// Save will persist the underling in memory inventory data for access on subsequent interaction
	Save(ctx context.Context, dryRun bool) error

	// Load will retrieve the inventory data saved, if available, and return it in ResourceMetadata form
	Load(ctx context.Context) ([]ResourceMetadata, error)

	// SetObjects will replace the current in memory objects inventory data
	SetObjects([]*unstructured.Unstructured)
}
