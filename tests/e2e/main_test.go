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
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/mia-platform/jpl/pkg/flowcontrol"
	"k8s.io/client-go/rest"
	"sigs.k8s.io/controller-runtime/pkg/envtest"
)

// testClusterConfig contains the current configuration for connecting to the testenv "fake" cluster
var testClusterConfig *rest.Config

func TestMain(m *testing.M) {
	// Do Envtest setup
	fmt.Println("Setting up testenv...")
	testEnv := envtest.Environment{
		CRDDirectoryPaths: []string{
			filepath.Join("..", "..", "testdata", "e2e", "crds"),
		},
		ErrorIfCRDPathMissing: true,
	}
	config, err := testEnv.Start()
	testClusterConfig = config

	if err != nil {
		fmt.Printf("failed to start testenv: %s\n", err)
		os.Exit(1)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Second)
	var fcEnabled bool
	if fcEnabled, err = flowcontrol.IsEnabled(ctx, testClusterConfig); err != nil {
		cancel()
		fmt.Printf("failed to check flowcontrol: %s\n", err)
		os.Exit(1)
	}
	cancel()

	if fcEnabled {
		testClusterConfig.QPS = -1
		testClusterConfig.Burst = -1
	}

	// execute go tests
	exitCode := m.Run()

	// Do Envtest teardown
	fmt.Println("Tearing down testenv...")
	if err := testEnv.Stop(); err != nil {
		fmt.Printf("failed to stop testenv: %s\n", err)
	}

	os.Exit(exitCode)
}
