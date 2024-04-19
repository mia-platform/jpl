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
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"io"
	"path/filepath"
	"strings"
	"testing"
	"time"

	jplclient "github.com/mia-platform/jpl/pkg/client"
	"github.com/mia-platform/jpl/pkg/generator"
	"github.com/mia-platform/jpl/pkg/inventory"
	"github.com/mia-platform/jpl/pkg/resourcereader"
	"github.com/mia-platform/jpl/pkg/util"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/cli-runtime/pkg/genericclioptions"
	"k8s.io/client-go/rest"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// factoryForTesting return a ClientFactory configured with the current testenv configuration and the providednamespace.
// It also check if the cluster support flowcontrol and set the corresponding burst and qps accordingly.
func factoryForTesting(t *testing.T, namespace *string) util.ClientFactory {
	t.Helper()
	cliConfig := genericclioptions.NewConfigFlags(false)
	cliConfig.WrapConfigFn = func(_ *rest.Config) *rest.Config {
		return rest.CopyConfig(testClusterConfig)
	}

	cliConfig.Namespace = namespace
	return util.NewFactory(cliConfig)
}

// testdataPathForPath return a valid path relative to the testadata folder
func testdataPathForPath(t *testing.T, resourcePath string) string {
	t.Helper()
	return filepath.Join("..", "..", "testdata", "e2e", resourcePath)
}

// namespaceForTesting return a valid namespace name with randomness in it to avoid collision during parallel testing
func namespaceForTesting(t *testing.T) string {
	t.Helper()

	b := make([]byte, 30)
	_, _ = rand.Read(b)
	return fmt.Sprintf("%s-%s", strings.ToLower(t.Name()), hex.EncodeToString(b))[:30]
}

// createNamespaceForTesting will generate a new random namespace name and create it on the api-server targeted
// by client
func createNamespaceForTesting(t *testing.T) string {
	t.Helper()
	envtestClient, err := client.New(testClusterConfig, client.Options{})
	require.NoError(t, err)

	namespace := namespaceForTesting(t)

	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Second)
	defer cancel()

	err = envtestClient.Create(ctx, &corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: namespace}})
	require.NoError(t, err)
	return namespace
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
	require.Equal(t, expectedCount, len(resources), "unexpected count of reasources read from path or buffer")

	applier, err := jplclient.NewBuilder().
		WithGenerators(generator.NewJobGenerator("jpl.mia-platform.eu/create", "true")).
		WithFactory(factory).
		WithInventory(store).
		Build()
	require.NoError(t, err)

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
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

func getResourcesFromEnv(t *testing.T, list client.ObjectList, options client.ListOption) {
	t.Helper()
	envtestClient, err := client.New(testClusterConfig, client.Options{})
	require.NoError(t, err)

	err = envtestClient.List(context.Background(), list, options)
	require.NoError(t, err)
}

func getResourceFromEnv(t *testing.T, name client.ObjectKey, obj client.Object, options client.GetOption) {
	t.Helper()
	envtestClient, err := client.New(testClusterConfig, client.Options{})
	require.NoError(t, err)

	err = envtestClient.Get(context.Background(), name, obj, options)
	require.NoError(t, err)
}
