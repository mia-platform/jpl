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
	"fmt"

	"github.com/mia-platform/jpl/internal/poller"
	"github.com/mia-platform/jpl/pkg/filter"
	"github.com/mia-platform/jpl/pkg/generator"
	"github.com/mia-platform/jpl/pkg/inventory"
	"github.com/mia-platform/jpl/pkg/mutator"
	"github.com/mia-platform/jpl/pkg/runner"
	"github.com/mia-platform/jpl/pkg/runner/task"
	"github.com/mia-platform/jpl/pkg/util"
)

// Builder is used to correctly instantiate an Applier client with the correct properties
type Builder struct {
	factory    util.ClientFactory
	runner     runner.TaskRunner
	inventory  inventory.Store
	generators []generator.Interface
	mutators   []mutator.Interface
	filters    []filter.Interface
	poller     poller.StatusPoller
}

// NewBuilder return a new Builder instance with configured defaults
func NewBuilder() *Builder {
	return &Builder{
		runner:     runner.NewTaskRunner(),
		generators: []generator.Interface{},
	}
}

// WithFactory assing a ClientFactory to the builder
func (b *Builder) WithFactory(factory util.ClientFactory) *Builder {
	b.factory = factory
	return b
}

// WithInventory assing an inventory.Store to the Builder
func (b *Builder) WithInventory(inventory inventory.Store) *Builder {
	b.inventory = inventory
	return b
}

// WithGenerators assing one or more generators to the Builder
func (b *Builder) WithGenerators(generators ...generator.Interface) *Builder {
	b.generators = generators
	return b
}

// WithMutator assing one or more generators to the Builder
func (b *Builder) WithMutator(mutators ...mutator.Interface) *Builder {
	b.mutators = mutators
	return b
}

// WithFilters assing one or more generators to the Builder
func (b *Builder) WithFilters(filters ...filter.Interface) *Builder {
	b.filters = filters
	return b
}

func (b *Builder) WithStatusPoller(poller poller.StatusPoller) *Builder {
	b.poller = poller
	return b
}

// Build use default values and configured builder porperty for correctly setup an Applier
func (b *Builder) Build() (*Applier, error) {
	if b.factory == nil {
		return nil, fmt.Errorf("cannot build an Applier client without a valid factory")
	}

	if b.runner == nil {
		return nil, fmt.Errorf("cannot build an Applier client without a valid runner")
	}

	if b.inventory == nil {
		return nil, fmt.Errorf("cannot build an Applier client without a valid inventory")
	}

	client, err := b.factory.DynamicClient()
	if err != nil {
		return nil, fmt.Errorf("failed to retrieve a valid kubernetes client: %w", err)
	}

	mapper, err := b.factory.ToRESTMapper()
	if err != nil {
		return nil, fmt.Errorf("failed to retrieve a valid RESTMapper: %w", err)
	}

	fetcher, err := task.DefaultInfoFetcherBuilder(b.factory)
	if err != nil {
		return nil, fmt.Errorf("failed to retrieve a valid Info Fetcher: %w", err)
	}

	statusPoller := b.poller
	if statusPoller == nil {
		statusPoller = poller.NewDefaultStatusPoller(client, mapper)
	}

	return &Applier{
		client:      client,
		mapper:      mapper,
		runner:      b.runner,
		inventory:   b.inventory,
		infoFetcher: fetcher,
		generators:  b.generators,
		mutators:    b.mutators,
		filters:     b.filters,
		poller:      statusPoller,
	}, nil
}
