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

//go:build conformance

package e2e

import (
	"context"
	"io"
	"path/filepath"
	"testing"
	"time"

	jplclient "github.com/mia-platform/jpl/pkg/client"
	"github.com/mia-platform/jpl/pkg/generator"
	"github.com/mia-platform/jpl/pkg/inventory"
	"github.com/mia-platform/jpl/pkg/resourcereader"
	"github.com/mia-platform/jpl/pkg/util"
	"github.com/stretchr/testify/require"
	"k8s.io/cli-runtime/pkg/genericclioptions"
	"sigs.k8s.io/e2e-framework/pkg/envconf"
)

// factoryAndStoreForTesting return a ClientFactory and Store for use in testing environments
func factoryAndStoreForTesting(t *testing.T, cfg *envconf.Config, inventoryName string) (util.ClientFactory, inventory.Store) {
	t.Helper()

	kubeconfig := cfg.KubeContext()
	namespace := cfg.Namespace()
	t.Logf("starting test with kubeconfig %q and namespace %q", cfg.KubeconfigFile(), cfg.Namespace())
	cliConfig := genericclioptions.NewConfigFlags(false)
	cliConfig.KubeConfig = &kubeconfig
	cliConfig.Namespace = &namespace
	factory := util.NewFactory(cliConfig)
	store, err := inventory.NewConfigMapStore(factory, inventoryName, cfg.Namespace(), "e2e-test-jlp")
	require.NoError(t, err)

	return factory, store
}

// testdataPathForPath return a valid path relative to the testadata folder
func testdataPathForPath(t *testing.T, resourcePath string) string {
	t.Helper()
	return filepath.Join("testdata", resourcePath)
}

// applyResources will read resources from reader or path, check that the expectedCount of resource is found
// and than apply them to the remote server
func applyResources(t *testing.T, factory util.ClientFactory, store inventory.Store, reader io.Reader, path string, expectedCount int) {
	t.Helper()
	readerBuilder := resourcereader.NewResourceReaderBuilder(factory)
	resourceReader, err := readerBuilder.ResourceReader(reader, path)
	require.NoError(t, err)

	resources, err := resourceReader.Read()
	require.NoError(t, err)
	require.Len(t, resources, expectedCount, "unexpected count of reasources read from path or buffer")

	applier, err := jplclient.NewBuilder().
		WithGenerators(generator.NewJobGenerator("jpl.mia-platform.eu/create", "true")).
		WithFactory(factory).
		WithInventory(store).
		Build()
	require.NoError(t, err)

	ctx, cancel := context.WithTimeout(t.Context(), 10*time.Minute)
	defer cancel()

	eventCh := applier.Run(ctx, resources, jplclient.ApplierOptions{FieldManager: "jpl-test"})

	for {
		select {
		case event, open := <-eventCh:
			if !open {
				return
			}

			t.Log(event)
		case <-ctx.Done():
			require.NoError(t, ctx.Err())
			return
		}
	}
}
