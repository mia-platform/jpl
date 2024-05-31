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
	"errors"
	"fmt"
	"io"
	"slices"
	"sync"

	"github.com/mia-platform/jpl/pkg/event"
	"github.com/mia-platform/jpl/pkg/resource"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/client-go/tools/cache"
)

// InformerMultiplexer can be used to manage multiple informer better than a working group. With InformerMultiplexer
// we can stop and restart informer based on the responses to better manage the calls made to the remote api
type InformerMultiplexer struct {
	InformerBuilder InformerBuilder
	Resources       []InformerResource
	ObjectToObserve []resource.ObjectMetadata

	context            context.Context
	cancelFunc         context.CancelFunc
	channelMultiplexer *Multiplexer[event.Event]

	informersMapLock sync.Mutex
	informers        map[InformerResource]*Informer

	running bool
	stopped bool
	lock    sync.Mutex
}

// Run will start the multiplexer for the set Resources and ObjectToObserve, it will return a channel where all the
// obkect statuses will be reported
func (im *InformerMultiplexer) Run(ctx context.Context) <-chan event.Event {
	im.lock.Lock()
	defer im.lock.Unlock()

	if im.running {
		errorCh := make(chan event.Event)
		go func() {
			defer close(errorCh)
			im.handleBlockingError(errorCh, fmt.Errorf("cannot restart an already running informer"))
		}()
		return errorCh
	}

	im.context, im.cancelFunc = context.WithCancel(ctx)
	im.channelMultiplexer = NewMultiplexer[event.Event](im.context.Done())
	im.informers = make(map[InformerResource]*Informer, len(im.Resources))
	for _, resource := range im.Resources {
		im.startInformer(resource)
	}
	im.running = true

	// block in another goroutine until the context is marked as done to flip the stopped property to true
	go func() {
		<-im.context.Done()

		im.lock.Lock()
		defer im.lock.Unlock()
		im.stopped = true
	}()

	return im.channelMultiplexer.MultiplexedChannel()
}

// Stop will mark the multiplexer context as done and stop new informer to be added, the channel will remain open
// until all currently started informer will send their last message
func (im *InformerMultiplexer) Stop() {
	im.cancelFunc()
}

func (im *InformerMultiplexer) startInformer(resource InformerResource) {
	go func() {
		im.informersMapLock.Lock()
		defer im.informersMapLock.Unlock()

		if _, found := im.informers[resource]; found {
			return
		}

		informer, err := im.runInformer(resource)
		if err != nil {
			errorCh := make(chan event.Event)
			defer close(errorCh)
			if err := im.channelMultiplexer.AddChannel(errorCh); err != nil {
				// the channel multiplexer is already closed, do nothing
				return
			}

			im.handleInformerError(errorCh, err)
			return
		}

		im.informers[resource] = informer
	}()
}

func (im *InformerMultiplexer) runInformer(resource InformerResource) (*Informer, error) {
	informer, err := im.InformerBuilder.NewInformer(im.context, resource)
	if err != nil {
		return informer, err
	}

	informerCh := make(chan event.Event)

	watchErrorHandler := func(_ *cache.Reflector, err error) {
		im.watchErrorHandler(resource, informerCh, err)
	}
	if err := informer.SetWatchErrorHandler(watchErrorHandler); err != nil {
		close(informerCh)
		return informer, err
	}

	if _, err := informer.AddEventHandler(im.eventHandler(im.context, informerCh)); err != nil {
		close(informerCh)
		return informer, err
	}

	if err = im.channelMultiplexer.AddChannel(informerCh); err != nil {
		close(informerCh)
		return informer, err
	}

	go func() {
		informer.Run()
		close(informerCh)
	}()
	return informer, nil
}

// func (im *InformerMultiplexer) stopInformer(resource InformerResource) {
// 	im.informersMapLock.Lock()
// 	defer im.informersMapLock.Unlock()

// 	informer, found := im.informers[resource]
// 	if !found {
// 		return
// 	}

// 	informer.Stop()
// 	delete(im.informers, resource)
// }

func (im *InformerMultiplexer) watchErrorHandler(_ InformerResource, eventCh chan<- event.Event, err error) {
	// TODO: handle the various errors
	switch {
	case err == io.EOF:
	case err == io.ErrUnexpectedEOF:
	case errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded):
	case apierrors.IsNotFound(err):
	case apierrors.IsResourceExpired(err) || apierrors.IsGone(err):
	case apierrors.IsForbidden(err):
		im.handleInformerError(eventCh, err)
	default:
	}
}

func (im *InformerMultiplexer) eventHandler(ctx context.Context, eventCh chan<- event.Event) cache.ResourceEventHandler {
	handler := cache.ResourceEventHandlerFuncs{}
	handler.AddFunc = func(obj interface{}) {
		if ctx.Err() != nil {
			return
		}

		unstruct, ok := obj.(*unstructured.Unstructured)
		if !ok {
			im.handleInformerError(eventCh, fmt.Errorf("informer received unexpected object type %T", obj))
			return
		}

		if im.filterObject(unstruct) {
			// object received is not of our interest, move along
			return
		}

		result, err := statusCheck(unstruct)
		if err != nil {
			im.handleInformerError(eventCh, err)
			return
		}

		eventCh <- eventFromResult(result, unstruct)
	}

	// we don't care of the old version of the object
	handler.UpdateFunc = func(_, obj interface{}) {
		if ctx.Err() != nil {
			return
		}

		unstruct, casted := obj.(*unstructured.Unstructured)
		if !casted {
			im.handleInformerError(eventCh, fmt.Errorf("informer received unexpected object type %T", obj))
			return
		}

		if im.filterObject(unstruct) {
			// object received is not of our interest, move along
			return
		}

		result, err := statusCheck(unstruct)
		if err != nil {
			im.handleInformerError(eventCh, err)
			return
		}

		eventCh <- eventFromResult(result, unstruct)
	}

	handler.DeleteFunc = func(obj interface{}) {
		if ctx.Err() != nil {
			return
		}

		// check if the returned obj is a DeletedFinalStateUnknown because the state before the deletion is unknown
		if deletedObject, casted := obj.(cache.DeletedFinalStateUnknown); casted {
			obj = deletedObject.Obj
		}

		unstruct, casted := obj.(*unstructured.Unstructured)
		if !casted {
			im.handleInformerError(eventCh, fmt.Errorf("informer received unexpected object type %T", obj))
			return
		}

		if im.filterObject(unstruct) {
			// object received is not of our interest, move along
			return
		}

		eventCh <- eventFromResult(terminatingResult(deletionMessage), unstruct)
	}

	return handler
}

func (im *InformerMultiplexer) filterObject(obj *unstructured.Unstructured) bool {
	return !slices.Contains(im.ObjectToObserve, resource.ObjectMetadataFromUnstructured(obj))
}

func eventFromResult(result *Result, obj *unstructured.Unstructured) event.Event {
	statusEvent := event.Event{
		Type: event.TypeStatusUpdate,
		StatusUpdateInfo: event.StatusUpdateInfo{
			Message:        result.Message,
			ObjectMetadata: resource.ObjectMetadataFromUnstructured(obj),
		},
	}
	switch result.Status {
	case StatusCurrent:
		statusEvent.StatusUpdateInfo.Status = event.StatusSuccessful
	case StatusInProgress, StatusTerminating:
		statusEvent.StatusUpdateInfo.Status = event.StatusPending
	case StatusFailed:
		statusEvent.StatusUpdateInfo.Status = event.StatusFailed
	}

	return statusEvent
}

func (im *InformerMultiplexer) handleBlockingError(ch chan<- event.Event, err error) {
	ch <- event.Event{
		Type: event.TypeError,
		ErrorInfo: event.ErrorInfo{
			Error: err,
		},
	}
}

func (im *InformerMultiplexer) handleInformerError(ch chan<- event.Event, err error) {
	if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
		return
	}

	im.handleBlockingError(ch, err)
	im.Stop()
}
