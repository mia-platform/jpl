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
	"k8s.io/apimachinery/pkg/util/sets"

	"github.com/mia-platform/jpl/pkg/resource"
)

// Store define an interface for working with an inventory of deployed resources, without knowning the underling
// technology that is used for persisting the data
type Store interface {
	// Load will read the inventory data from the remote storage of the inventory
	Load(ctx context.Context) (sets.Set[resource.ObjectMetadata], error)

	// Save will persist the underling in memory inventory data for access on subsequent interaction
	Save(ctx context.Context, dryRun bool) error

	// Delete will remove remote storage of the inventory
	Delete(ctx context.Context, dryRun bool) error

	// SetObjects will replace the current in memory objects inventory data
	SetObjects(objects sets.Set[*unstructured.Unstructured])
}
