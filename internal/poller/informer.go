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
	"sync"
	"time"

	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/watch"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/tools/cache"
)

// InformerResource encapsulate the needed information from a resource for instatiating an informer
type InformerResource struct {
	GroupKind schema.GroupKind
	Namespace string
}

// InformerBuilder can be used for setting up a series of new informers reusing the same configurations
type InformerBuilder struct {
	Client       dynamic.Interface
	Mapper       meta.RESTMapper
	ResyncPeriod time.Duration
	Indexers     cache.Indexers
}

// NewInfromerBuilder return a new informer builder configured with client and resync period
func NewInfromerBuilder(client dynamic.Interface, mapper meta.RESTMapper, resync time.Duration) *InformerBuilder {
	return &InformerBuilder{
		Client:       client,
		Mapper:       mapper,
		ResyncPeriod: resync,
		Indexers: cache.Indexers{
			cache.NamespaceIndex: cache.MetaNamespaceIndexFunc,
		},
	}
}

// NewInformer return a new SharedInformer configured for resource or an error if the resource group/kind cannot be
// looked up to the remote server
func (b *InformerBuilder) NewInformer(informerCtx context.Context, resource InformerResource) (*Informer, error) {
	mapping, err := b.Mapper.RESTMapping(resource.GroupKind)
	if err != nil {
		return nil, err
	}

	lw := &cache.ListWatch{
		ListFunc: func(options metav1.ListOptions) (runtime.Object, error) {
			return b.Client.
				Resource(mapping.Resource).
				Namespace(resource.Namespace).
				List(informerCtx, options)
		},
		WatchFunc: func(options metav1.ListOptions) (watch.Interface, error) {
			return b.Client.
				Resource(mapping.Resource).
				Namespace(resource.Namespace).
				Watch(informerCtx, options)
		},
	}

	example := &unstructured.Unstructured{}
	example.SetGroupVersionKind(mapping.GroupVersionKind)
	informerCtx, cancelFunc := context.WithCancel(informerCtx)

	return &Informer{
		sharedInformer: cache.NewSharedIndexInformer(lw, example, b.ResyncPeriod, b.Indexers),
		context:        informerCtx,
		cancelFunc:     cancelFunc,
	}, nil
}

type Informer struct {
	sharedInformer cache.SharedIndexInformer
	context        context.Context
	cancelFunc     context.CancelFunc

	started bool
	lock    sync.Mutex
}

func (i *Informer) Run() {
	i.lock.Lock()
	defer i.lock.Unlock()

	if i.started {
		return
	}

	i.sharedInformer.Run(i.context.Done())
	i.started = true
}

func (i *Informer) Stop() {
	i.lock.Lock()
	defer i.lock.Unlock()

	i.cancelFunc()
}

func (i *Informer) SetWatchErrorHandler(handler cache.WatchErrorHandler) error {
	return i.sharedInformer.SetWatchErrorHandler(handler)
}

func (i *Informer) AddEventHandler(handler cache.ResourceEventHandler) (cache.ResourceEventHandlerRegistration, error) {
	return i.sharedInformer.AddEventHandler(handler)
}
