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
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestMultiplexerError(t *testing.T) {
	t.Parallel()

	// create a new context and cancel it immediately
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	multiplexer := NewMultiplexer[string](ctx.Done())
	// wait until context is cancelled
	<-ctx.Done()

	err := multiplexer.AddChannel(make(chan string))
	assert.Error(t, err)
	assert.ErrorContains(t, err, closedErrorMessage)

	_, isOpen := <-multiplexer.MultiplexedChannel()
	assert.False(t, isOpen, "channel is still open")
}

func TestAddingSingleChannel(t *testing.T) {
	t.Parallel()

	// create a new context and cancel it immediately
	ctx, cancel := context.WithCancel(context.Background())

	multiplexer := NewMultiplexer[string](ctx.Done())

	eventCh := make(chan string)
	multiplexer.AddChannel(eventCh)

	totalEvents := 100
	go func() {
		defer cancel()
		for i := 0; i < totalEvents; i++ {
			eventCh <- fmt.Sprintf("event number %d", i+1)
		}
		close(eventCh)
	}()

	count := 0
	for event := range multiplexer.MultiplexedChannel() {
		count++
		t.Log(event)
	}

	assert.Equal(t, totalEvents, count)
}

func TestAdddingMultipleChannels(t *testing.T) {
	t.Parallel()

	totalEventFirstChan := 50
	totalEventSecondChan := 40
	ctx, cancel := context.WithCancel(context.Background())

	multiplexer := NewMultiplexer[string](ctx.Done())
	firstCtx := runChannel(ctx, t, multiplexer, totalEventFirstChan)
	secondCtx := runChannel(ctx, t, multiplexer, totalEventSecondChan)
	firstDone := firstCtx.Done()
	secondDone := secondCtx.Done()

	multiplexChan := multiplexer.MultiplexedChannel()
	count := 0
loop:
	for {
		select {
		case <-firstDone:
			firstDone = nil
		case <-secondDone:
			secondDone = nil
		case event, isOpen := <-multiplexChan:
			if !isOpen {
				cancel() // here the context is always closed, but we make the compiler happy
				break loop
			}

			count++
			t.Log(event)
		}

		if firstDone == nil && secondDone == nil {
			cancel()
		}
	}

	totalEvents := totalEventFirstChan + totalEventSecondChan
	assert.Equal(t, totalEvents, count)
}

func runChannel(ctx context.Context, t *testing.T, multiplexer *Multiplexer[string], totalMessages int) context.Context {
	t.Helper()

	ctx, cancel := context.WithCancel(ctx)
	go func() {
		eventCh := make(chan string)
		defer func() {
			close(eventCh)
			cancel()
		}()

		err := multiplexer.AddChannel(eventCh)
		if !assert.NoError(t, err) {
			return
		}
		for i := 0; i < totalMessages; i++ {
			eventCh <- fmt.Sprintf("event number %d of %d", i+1, totalMessages)
		}
	}()

	return ctx
}
