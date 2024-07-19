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

package poller

import (
	"context"
	"time"

	"github.com/mia-platform/jpl/pkg/event"
	"github.com/mia-platform/jpl/pkg/resource"
	"k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/client-go/dynamic"
)

type StatusPoller interface {
	// Start will start polling the remote api-server for getting updates to the passed resources
	Start(context.Context, []*unstructured.Unstructured) <-chan event.Event
}

type defaultStatusPoller struct {
	client dynamic.Interface
	mapper meta.RESTMapper
	resync time.Duration
}

// NewDefaultStatusPoller return a default implementation of StatusPoller that will connect to the
// remote api-server and elaborate the current status of the requested resources
func NewDefaultStatusPoller(client dynamic.Interface, mapper meta.RESTMapper) StatusPoller {
	return &defaultStatusPoller{
		client: client,
		mapper: mapper,
		resync: 5 * time.Minute,
	}
}

// Start implement StatusPoller interface
func (p *defaultStatusPoller) Start(ctx context.Context, objects []*unstructured.Unstructured) <-chan event.Event {
	informerResources, ids := resourcesAndIDsFromObjects(objects)
	multiplexer := &informerMultiplexer{
		InformerBuilder: *newInfromerBuilder(p.client, p.mapper, p.resync),
		Resources:       informerResources,
		ObjectToObserve: ids,
	}

	return multiplexer.Run(ctx)
}

// resourcesAndIDsFromObjects return an array of unique InformerResources created from objects
func resourcesAndIDsFromObjects(objects []*unstructured.Unstructured) ([]informerResource, []resource.ObjectMetadata) {
	results := make(sets.Set[informerResource], 0)
	ids := make([]resource.ObjectMetadata, 0, len(objects))
	for _, obj := range objects {
		results.Insert(informerResource{
			GroupKind: obj.GroupVersionKind().GroupKind(),
			Namespace: obj.GetNamespace(),
		})
		ids = append(ids, resource.ObjectMetadataFromUnstructured(obj))
	}

	return results.UnsortedList(), ids
}
