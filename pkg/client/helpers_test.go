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

package client

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net/http"
	"testing"

	"github.com/mia-platform/jpl/pkg/client/cache"
	"github.com/mia-platform/jpl/pkg/event"
	"github.com/mia-platform/jpl/pkg/filter"
	"github.com/mia-platform/jpl/pkg/generator"
	fakeinventory "github.com/mia-platform/jpl/pkg/inventory/fake"
	"github.com/mia-platform/jpl/pkg/mutator"
	"github.com/mia-platform/jpl/pkg/poller"
	pkgtesting "github.com/mia-platform/jpl/pkg/testing"
	"github.com/mia-platform/jpl/pkg/util"
	"github.com/stretchr/testify/require"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/cli-runtime/pkg/resource"
	dynamicfake "k8s.io/client-go/dynamic/fake"
	"k8s.io/client-go/rest/fake"
)

var (
	codec = pkgtesting.Codecs.LegacyCodec(pkgtesting.Scheme.PrioritizedVersionsAllGroups()...)
)

func newTestApplier(t *testing.T, objects []*unstructured.Unstructured, inventoryObjects []*unstructured.Unstructured, statusEvents []event.Event, generator generator.Interface, mutator mutator.Interface, filter filter.Interface) *Applier {
	t.Helper()

	builder := NewBuilder().
		WithFactory(factoryForTesting(t, objects, inventoryObjects)).
		WithInventory(&fakeinventory.Inventory{InventoryObjects: inventoryObjects}).
		WithStatusPoller(&fakePollerBuilder{events: statusEvents})
	if generator != nil {
		builder.WithGenerators(generator)
	}
	if mutator != nil {
		builder.WithMutator(mutator)
	}
	if filter != nil {
		builder.WithFilters(filter)
	}

	applier, err := builder.Build()
	require.NoError(t, err)

	return applier
}

func factoryForTesting(t *testing.T, objects []*unstructured.Unstructured, remoteObjects []*unstructured.Unstructured) util.ClientFactory {
	t.Helper()

	tf := pkgtesting.NewTestClientFactory()

	mapper, err := tf.ToRESTMapper()
	require.NoError(t, err)

	handler := &handler{
		mapper:  mapper,
		objects: objects,
	}

	tf.Client = fakeRESTClient(t, handler)
	tf.FakeDynamicClient = fakeDynamicClient(t, remoteObjects)
	return tf
}

// fakeRESTClient returns a fake REST client.
func fakeRESTClient(t *testing.T, handler *handler) *fake.RESTClient {
	t.Helper()

	return &fake.RESTClient{
		NegotiatedSerializer: resource.UnstructuredPlusDefaultContentConfig().NegotiatedSerializer,
		Client: fake.CreateHTTPClient(func(request *http.Request) (*http.Response, error) {
			t.Logf("Received %s call for %s", request.Method, request.URL)
			if handled, response, err := handler.handleRequest(t, request); handled {
				return response, err
			}

			t.Logf("unexpected request: %#v\n%#v", request.URL, request)
			return nil, nil
		}),
	}
}

// fakeDynamicClient returns a fake dynamic client.
func fakeDynamicClient(t *testing.T, objs []*unstructured.Unstructured) *dynamicfake.FakeDynamicClient {
	t.Helper()

	fakeClient := dynamicfake.NewSimpleDynamicClient(pkgtesting.Scheme)
	for _, obj := range objs {
		err := fakeClient.Tracker().Add(obj)
		require.NoError(t, err)
	}

	return fakeClient
}

type handler struct {
	mapper  meta.RESTMapper
	objects []*unstructured.Unstructured
}

func (h *handler) handleRequest(t *testing.T, request *http.Request) (bool, *http.Response, error) {
	t.Helper()

	for _, obj := range h.objects {
		gvk := obj.GroupVersionKind()
		mapping, err := h.mapper.RESTMapping(gvk.GroupKind(), gvk.Version)
		if err != nil {
			return false, nil, err
		}

		path := fmt.Sprintf("/%s/%s", mapping.Resource.Resource, obj.GetName())
		if mapping.Scope == meta.RESTScopeNamespace {
			path = "/namespaces/" + obj.GetNamespace() + path
		}

		if request.URL.Path == path && request.Method == http.MethodPatch {
			data, err := runtime.Encode(unstructured.NewJSONFallbackEncoder(codec), obj)
			require.NoError(t, err)
			bodyRC := io.NopCloser(bytes.NewReader(data))
			return true, &http.Response{StatusCode: http.StatusOK, Header: pkgtesting.DefaultHeaders(), Body: bodyRC}, nil
		}
	}

	return false, nil, nil
}

var _ generator.Interface = &fakeGenerator{}

type fakeGenerator struct {
	resource *unstructured.Unstructured
}

func (g *fakeGenerator) CanHandleResource(r *metav1.PartialObjectMetadata) bool {
	return r.Kind == "CronJob"
}

func (g *fakeGenerator) Generate(_ *unstructured.Unstructured, _ cache.RemoteResourceGetter) ([]*unstructured.Unstructured, error) {
	return []*unstructured.Unstructured{g.resource}, nil
}

var _ generator.Interface = &errorGenerator{}

type errorGenerator struct {
	err error
}

func (g *errorGenerator) CanHandleResource(_ *metav1.PartialObjectMetadata) bool {
	return true
}

func (g *errorGenerator) Generate(_ *unstructured.Unstructured, _ cache.RemoteResourceGetter) ([]*unstructured.Unstructured, error) {
	return []*unstructured.Unstructured{}, g.err
}

var _ poller.StatusPoller = &fakePollerBuilder{}

type fakePollerBuilder struct {
	events []event.Event
}

func (b *fakePollerBuilder) Start(ctx context.Context, _ []*unstructured.Unstructured) <-chan event.Event {
	eventCh := make(chan event.Event)

	go func() {
		defer close(eventCh)

		for _, event := range b.events {
			eventCh <- event
		}

		<-ctx.Done()
	}()

	return eventCh
}
