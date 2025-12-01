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
	"sync"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/dynamic"

	"github.com/mia-platform/jpl/pkg/resource"
)

type resourceStatus struct {
	obj *unstructured.Unstructured
	err error
}

type CachedResourceGetter struct {
	mapper meta.RESTMapper
	client dynamic.Interface

	lock  sync.Mutex
	cache map[resource.ObjectMetadata]resourceStatus
}

func NewCachedResourceGetter(mapper meta.RESTMapper, client dynamic.Interface) *CachedResourceGetter {
	return &CachedResourceGetter{
		mapper: mapper,
		client: client,
		cache:  make(map[resource.ObjectMetadata]resourceStatus),
	}
}

// Get implement RemoteResourceGetter interface
func (rg *CachedResourceGetter) Get(ctx context.Context, id resource.ObjectMetadata) (*unstructured.Unstructured, error) {
	rg.lock.Lock()
	defer rg.lock.Unlock()

	if status, found := rg.cache[id]; found {
		return status.obj, status.err
	}

	return rg.getObject(ctx, id)
}

func (rg *CachedResourceGetter) getObject(ctx context.Context, id resource.ObjectMetadata) (*unstructured.Unstructured, error) {
	mapping, err := rg.mapper.RESTMapping(schema.GroupKind{Group: id.Group, Kind: id.Kind})
	if err != nil {
		return nil, err
	}

	obj, err := rg.client.Resource(mapping.Resource).Namespace(id.Namespace).Get(ctx, id.Name, metav1.GetOptions{})
	if meta.IsNoMatchError(err) || apierrors.IsNotFound(err) {
		rg.cache[id] = resourceStatus{}
		return nil, nil
	}

	rg.cache[id] = resourceStatus{
		obj: obj,
		err: err,
	}

	return obj, err
}

// keep it to always check if CachedResourceGetter implement correctly the RemoteResourceGetter interface
var _ RemoteResourceGetter = &CachedResourceGetter{}
