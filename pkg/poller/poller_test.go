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
	"fmt"
	"path/filepath"
	"testing"
	"time"

	"github.com/mia-platform/jpl/pkg/event"
	"github.com/mia-platform/jpl/pkg/resource"
	pkgtesting "github.com/mia-platform/jpl/pkg/testing"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/watch"
	dynamicfake "k8s.io/client-go/dynamic/fake"
	clienttesting "k8s.io/client-go/testing"
)

func TestPoller(t *testing.T) {
	t.Parallel()

	testdata := "testdata"
	deployNamespace := "test-poller1"
	podNamespace := "test-poller2"

	deployment := pkgtesting.UnstructuredFromFile(t, filepath.Join(testdata, "deployNoStatus.yaml"))
	deploymentUpdate1 := pkgtesting.UnstructuredFromFile(t, filepath.Join(testdata, "deployUpdating.yaml"))
	deploymentUpdate2 := pkgtesting.UnstructuredFromFile(t, filepath.Join(testdata, "deployCurrent.yaml"))
	deployment.SetNamespace(deployNamespace)
	deploymentUpdate1.SetNamespace(deployNamespace)
	deploymentUpdate2.SetNamespace(deployNamespace)
	deplyGVK := deployment.GroupVersionKind()

	deployment2 := pkgtesting.UnstructuredFromFile(t, filepath.Join(testdata, "deployNoStatus.yaml"))
	deployment2.SetNamespace(deployNamespace)
	deployment2.SetName("another-name")

	pod := pkgtesting.UnstructuredFromFile(t, filepath.Join(testdata, "podNoStatus.yaml"))
	podUpdate := pkgtesting.UnstructuredFromFile(t, filepath.Join(testdata, "podReady.yaml"))
	pod.SetNamespace(podNamespace)
	podUpdate.SetNamespace(podNamespace)
	podGVK := pod.GroupVersionKind()

	crd := pkgtesting.UnstructuredFromFile(t, filepath.Join(testdata, "crdInstalling.yaml"))
	crdUpdate1 := pkgtesting.UnstructuredFromFile(t, filepath.Join(testdata, "crdNamesNotAccepted.yaml"))
	crdGVK := crd.GroupVersionKind()

	mapper := fakeRESTMapper(
		deplyGVK,
		podGVK,
		crdGVK,
	)

	deployMapping, err := mapper.RESTMapping(deplyGVK.GroupKind(), deplyGVK.Version)
	require.NoError(t, err)

	podMapping, err := mapper.RESTMapping(podGVK.GroupKind(), podGVK.Version)
	require.NoError(t, err)

	crdMapping, err := mapper.RESTMapping(crdGVK.GroupKind(), crdGVK.Version)
	require.NoError(t, err)

	tests := map[string]struct {
		resources      []*unstructured.Unstructured
		expectedEvents []event.Event
		updates        []func(*dynamicfake.FakeDynamicClient)
	}{
		"test single resource": {
			resources: []*unstructured.Unstructured{
				deployment,
			},
			expectedEvents: []event.Event{
				{
					Type: event.TypeStatusUpdate,
					StatusUpdateInfo: event.StatusUpdateInfo{
						Status:         event.StatusPending,
						Message:        fmt.Sprintf(deploymentFewReplicasMessageFormat, 0, 1),
						ObjectMetadata: resource.ObjectMetadataFromUnstructured(deployment),
					},
				},
				{
					Type: event.TypeStatusUpdate,
					StatusUpdateInfo: event.StatusUpdateInfo{
						Status:         event.StatusPending,
						Message:        fmt.Sprintf(deploymentUpdatingReplicasMessageFormat, 2, 4),
						ObjectMetadata: resource.ObjectMetadataFromUnstructured(deployment),
					},
				},
				{
					Type: event.TypeStatusUpdate,
					StatusUpdateInfo: event.StatusUpdateInfo{
						Status:         event.StatusSuccessful,
						Message:        fmt.Sprintf(deploymentCurrentMessageFormat, 1),
						ObjectMetadata: resource.ObjectMetadataFromUnstructured(deployment),
					},
				},
			},
			updates: []func(*dynamicfake.FakeDynamicClient){
				func(client *dynamicfake.FakeDynamicClient) {
					require.NoError(t, client.Tracker().Create(deployMapping.Resource, deployment, deployNamespace))
				},
				func(client *dynamicfake.FakeDynamicClient) {
					require.NoError(t, client.Tracker().Update(deployMapping.Resource, deploymentUpdate1, deployNamespace))
				},
				func(client *dynamicfake.FakeDynamicClient) {
					require.NoError(t, client.Tracker().Update(deployMapping.Resource, deploymentUpdate2, deployNamespace))
				},
			},
		},
		"test multiple resources in different namespaces": {
			resources: []*unstructured.Unstructured{
				pod,
				deployment,
			},
			expectedEvents: []event.Event{
				{
					Type: event.TypeStatusUpdate,
					StatusUpdateInfo: event.StatusUpdateInfo{
						Status:         event.StatusPending,
						Message:        fmt.Sprintf(deploymentFewReplicasMessageFormat, 0, 1),
						ObjectMetadata: resource.ObjectMetadataFromUnstructured(deployment),
					},
				},
				{
					Type: event.TypeStatusUpdate,
					StatusUpdateInfo: event.StatusUpdateInfo{
						Status:         event.StatusPending,
						Message:        podInProgressMessage,
						ObjectMetadata: resource.ObjectMetadataFromUnstructured(pod),
					},
				},
				{
					Type: event.TypeStatusUpdate,
					StatusUpdateInfo: event.StatusUpdateInfo{
						Status:         event.StatusSuccessful,
						Message:        podReadyMessage,
						ObjectMetadata: resource.ObjectMetadataFromUnstructured(pod),
					},
				},
				{
					Type: event.TypeStatusUpdate,
					StatusUpdateInfo: event.StatusUpdateInfo{
						Status:         event.StatusSuccessful,
						Message:        fmt.Sprintf(deploymentCurrentMessageFormat, 1),
						ObjectMetadata: resource.ObjectMetadataFromUnstructured(deployment),
					},
				},
			},
			updates: []func(*dynamicfake.FakeDynamicClient){
				func(client *dynamicfake.FakeDynamicClient) {
					require.NoError(t, client.Tracker().Create(deployMapping.Resource, deployment, deployNamespace))
				},
				func(client *dynamicfake.FakeDynamicClient) {
					require.NoError(t, client.Tracker().Create(podMapping.Resource, pod, podNamespace))
				},
				func(client *dynamicfake.FakeDynamicClient) {
					require.NoError(t, client.Tracker().Update(podMapping.Resource, podUpdate, podNamespace))
				},
				func(client *dynamicfake.FakeDynamicClient) {
					require.NoError(t, client.Tracker().Update(deployMapping.Resource, deploymentUpdate2, deployNamespace))
				},
			},
		},
		"test cluster resources": {
			resources: []*unstructured.Unstructured{
				crd,
			},
			expectedEvents: []event.Event{
				{
					Type: event.TypeStatusUpdate,
					StatusUpdateInfo: event.StatusUpdateInfo{
						Status:         event.StatusPending,
						Message:        crdInProgressMessage,
						ObjectMetadata: resource.ObjectMetadataFromUnstructured(crd),
					},
				},
				{
					Type: event.TypeStatusUpdate,
					StatusUpdateInfo: event.StatusUpdateInfo{
						Status:         event.StatusFailed,
						Message:        "custom message",
						ObjectMetadata: resource.ObjectMetadataFromUnstructured(crd),
					},
				},
			},
			updates: []func(*dynamicfake.FakeDynamicClient){
				func(client *dynamicfake.FakeDynamicClient) {
					require.NoError(t, client.Tracker().Create(crdMapping.Resource, crd, ""))
				},
				func(client *dynamicfake.FakeDynamicClient) {
					require.NoError(t, client.Tracker().Update(crdMapping.Resource, crdUpdate1, ""))
				},
			},
		},
		"delete object with ignored ones": {
			resources: []*unstructured.Unstructured{
				deployment,
			},
			expectedEvents: []event.Event{
				{
					Type: event.TypeStatusUpdate,
					StatusUpdateInfo: event.StatusUpdateInfo{
						Status:         event.StatusSuccessful,
						Message:        fmt.Sprintf(deploymentCurrentMessageFormat, 1),
						ObjectMetadata: resource.ObjectMetadataFromUnstructured(deployment),
					},
				},
				{
					Type: event.TypeStatusUpdate,
					StatusUpdateInfo: event.StatusUpdateInfo{
						Status:         event.StatusPending,
						Message:        deletionMessage,
						ObjectMetadata: resource.ObjectMetadataFromUnstructured(deployment),
					},
				},
			},
			updates: []func(*dynamicfake.FakeDynamicClient){
				func(client *dynamicfake.FakeDynamicClient) {
					require.NoError(t, client.Tracker().Create(deployMapping.Resource, deploymentUpdate2, deployNamespace))
					require.NoError(t, client.Tracker().Create(deployMapping.Resource, deployment2, deployNamespace))
				},
				func(client *dynamicfake.FakeDynamicClient) {
					require.NoError(t, client.Tracker().Delete(deployMapping.Resource, deployNamespace, deployment2.GetName()))
					require.NoError(t, client.Tracker().Delete(deployMapping.Resource, deployNamespace, deploymentUpdate2.GetName()))
				},
			},
		},
	}

	for testName, testCase := range tests {
		t.Run(testName, func(t *testing.T) {
			t.Parallel()

			ctx, cancel := context.WithTimeout(t.Context(), 1*time.Second)
			defer cancel()

			client := dynamicfake.NewSimpleDynamicClient(pkgtesting.Scheme)
			client.PrependReactor("*", "*", func(action clienttesting.Action) (bool, runtime.Object, error) {
				t.Logf(
					"Client received action with: Verb: %q, Resource: %q, Namespace: %q",
					action.GetVerb(),
					action.GetResource().Resource,
					action.GetNamespace(),
				)
				return false, nil, nil
			})
			client.PrependWatchReactor("*", func(action clienttesting.Action) (bool, watch.Interface, error) {
				t.Logf(
					"Client received watch action with: Verb: %q, Resource: %q, Namespace: %q",
					action.GetVerb(),
					action.GetResource().Resource,
					action.GetNamespace(),
				)
				return false, nil, nil
			})

			poller := NewDefaultStatusPoller(client, mapper, nil)
			eventCh := poller.Start(ctx, testCase.resources)
			time.Sleep(150 * time.Millisecond) // Allow the pollers to start

			steppingCh := make(chan struct{})
			defer close(steppingCh)

			go func() {
				<-steppingCh
				for _, update := range testCase.updates {
					update(client)
					<-steppingCh
				}
				cancel()
			}()

			steppingCh <- struct{}{}

			receivedEvents := make([]event.Event, 0)
		loop:
			for {
				select {
				case event, open := <-eventCh:
					if !open {
						break loop
					}
					t.Log(event)
					receivedEvents = append(receivedEvents, event)
					steppingCh <- struct{}{}

				case <-ctx.Done():
					break loop
				}
			}

			require.NotEqual(t, context.DeadlineExceeded, ctx.Err())
			assert.Len(t, receivedEvents, len(testCase.expectedEvents))
			assert.Equal(t, testCase.expectedEvents, receivedEvents)
		})
	}
}

func TestPollerErrors(t *testing.T) {
	t.Parallel()

	testdata := "testdata"
	cr := pkgtesting.UnstructuredFromFile(t, filepath.Join(testdata, "noStatus.yaml"))
	cr.SetNamespace(testdata)

	mapper := fakeRESTMapper()

	tests := map[string]struct {
		resources      []*unstructured.Unstructured
		expectedEvents []event.Event
		updates        []func(*dynamicfake.FakeDynamicClient)
	}{
		"unrecognized resource": {
			resources: []*unstructured.Unstructured{
				cr,
			},
			expectedEvents: []event.Event{
				{
					Type: event.TypeError,
					ErrorInfo: event.ErrorInfo{
						Error: &meta.NoKindMatchError{
							GroupKind: cr.GroupVersionKind().GroupKind(),
						},
					},
				},
			},
			updates: []func(*dynamicfake.FakeDynamicClient){
				func(*dynamicfake.FakeDynamicClient) {
					// do nothing, but go haed with the tests
				},
			},
		},
	}

	for testName, testCase := range tests {
		t.Run(testName, func(t *testing.T) {
			t.Parallel()

			ctx, cancel := context.WithTimeout(t.Context(), 5*time.Second)
			defer cancel()

			client := dynamicfake.NewSimpleDynamicClient(pkgtesting.Scheme)
			client.PrependReactor("*", "*", func(action clienttesting.Action) (bool, runtime.Object, error) {
				t.Logf(
					"Client received action with: Verb: %q, Resource: %q, Namespace: %q",
					action.GetVerb(),
					action.GetResource().Resource,
					action.GetNamespace(),
				)
				return false, nil, nil
			})
			client.PrependWatchReactor("*", func(action clienttesting.Action) (bool, watch.Interface, error) {
				t.Logf(
					"Client received watch action with: Verb: %q, Resource: %q, Namespace: %q",
					action.GetVerb(),
					action.GetResource().Resource,
					action.GetNamespace(),
				)
				return false, nil, nil
			})

			poller := NewDefaultStatusPoller(client, mapper, nil)
			eventCh := poller.Start(ctx, testCase.resources)

			steppingCh := make(chan struct{})
			defer close(steppingCh)

			go func() {
				<-steppingCh
				for _, update := range testCase.updates {
					update(client)
					<-steppingCh
				}
			}()

			steppingCh <- struct{}{}

			receivedEvents := make([]event.Event, 0)
			for event := range eventCh {
				t.Log(event)
				receivedEvents = append(receivedEvents, event)
				steppingCh <- struct{}{}
			}

			require.NotEqual(t, context.DeadlineExceeded, ctx.Err())
			assert.Len(t, receivedEvents, len(testCase.expectedEvents))
			assert.Equal(t, testCase.expectedEvents, receivedEvents)
		})
	}
}
