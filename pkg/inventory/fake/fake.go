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

// fake package provide a fake implementation of an inventory Store for using during tests.
package fake

import (
	"context"

	"github.com/mia-platform/jpl/pkg/inventory"
	"github.com/mia-platform/jpl/pkg/resource"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/util/sets"
)

// keep it to always check if Inventory implement correctly the Store interface
var _ inventory.Store = &Inventory{}

type Inventory struct {
	InventoryObjects []*unstructured.Unstructured

	SaveFunc func(context.Context, bool) error

	// LoadErr will be returned when a load request is supposed to be made
	LoadErr error
	// SaveErr will be returned when a save request is supposed to be made
	SaveErr error
	// DeleteErr will be returned when a delete request is supposed to be made
	DeleteErr error
}

// Load implement Store interface
func (i *Inventory) Load(_ context.Context) (sets.Set[resource.ObjectMetadata], error) {
	if i.LoadErr != nil {
		return nil, i.LoadErr
	}

	set := sets.New[resource.ObjectMetadata]()
	for _, obj := range i.InventoryObjects {
		set.Insert(resource.ObjectMetadataFromUnstructured(obj))
	}

	return set, nil
}

// Save implement Store interface
func (i *Inventory) Save(ctx context.Context, dryRun bool) error {
	if i.SaveErr != nil {
		return i.SaveErr
	}

	if i.SaveFunc != nil {
		return i.SaveFunc(ctx, dryRun)
	}

	return nil
}

// Delete implement Store interface
func (i *Inventory) Delete(_ context.Context, _ bool) error {
	if i.DeleteErr != nil {
		return i.DeleteErr
	}

	return nil
}

// SetObejcts implement Store interface
func (i *Inventory) SetObjects(_ sets.Set[*unstructured.Unstructured]) {}

// Diff implement Store interface
func (i *Inventory) Diff(ctx context.Context, objects []*unstructured.Unstructured) (sets.Set[resource.ObjectMetadata], error) {
	remoteObjects, err := i.Load(ctx)
	if err != nil {
		return sets.New[resource.ObjectMetadata](), err
	}

	for _, obj := range objects {
		remoteObjects.Delete(resource.ObjectMetadataFromUnstructured(obj))
	}

	return remoteObjects, nil
}
