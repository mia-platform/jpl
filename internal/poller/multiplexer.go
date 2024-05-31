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
	"fmt"
	"sync/atomic"
)

const (
	closedErrorMessage = "multiplexer is closed, we cannot add new channels"
)

type Multiplexer[T any] struct {
	multiplexedChannel chan T

	atomicCounter *atomic.Int32
	doneCh        <-chan struct{}
}

func NewMultiplexer[T any](doneCh <-chan struct{}) *Multiplexer[T] {
	multiplexer := &Multiplexer[T]{
		multiplexedChannel: make(chan T),
		atomicCounter:      &atomic.Int32{},
		doneCh:             doneCh,
	}

	// start a goroutine to monitoring if all the multiplexed channels han been closed
	// and the main context is done. Only then we can close the multiplexed channel
	go func() {
		defer close(multiplexer.multiplexedChannel)

		contextDone := doneCh
		for {
			select {
			case <-contextDone:
				// the context is done,
				// set to nil to avoid busy waiting
				contextDone = nil
			default:
				// context is not done or nil, continue to check if counter is 0 or less
			}

			if contextDone == nil && multiplexer.atomicCounter.Load() <= 0 {
				break
			}
		}
	}()

	return multiplexer
}

func (m *Multiplexer[T]) MultiplexedChannel() <-chan T {
	return m.multiplexedChannel
}

func (m *Multiplexer[T]) AddChannel(ch <-chan T) error {
	select {
	case <-m.doneCh:
		return fmt.Errorf(closedErrorMessage)
	default:
		// context is not done, we can add the channel
		m.atomicCounter.Add(1)
	}

	go func() {
		defer m.atomicCounter.Add(-1)

		// parse all channel events and send them to the multiplexed channel
		for event := range ch {
			m.multiplexedChannel <- event
		}
	}()

	return nil
}
