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

package flowcontrol

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	flowcontrolapi "k8s.io/api/flowcontrol/v1beta3"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd/api"
)

func TestIsEnabled(t *testing.T) {
	t.Parallel()

	testCases := map[string]struct {
		handler         http.HandlerFunc
		expectedEnabled bool
	}{
		"headers found": {
			handler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				defer r.Body.Close()
				w.Header().Add(flowcontrolapi.ResponseHeaderMatchedFlowSchemaUID, "unused-value")
				w.WriteHeader(http.StatusOK)
				w.Write([]byte(""))
			}),
			expectedEnabled: true,
		},
		"headers not found": {
			handler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				defer r.Body.Close()
				w.WriteHeader(http.StatusOK)
				w.Write([]byte(""))
			}),
		},
		"return code != 200": {
			expectedEnabled: true,
			handler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				defer r.Body.Close()
				w.Header().Add(flowcontrolapi.ResponseHeaderMatchedFlowSchemaUID, "unused-value")
				w.WriteHeader(http.StatusNotFound)
				w.Write([]byte(""))
			}),
		},
	}

	for testName, testCase := range testCases {
		t.Run(testName, func(t *testing.T) {
			t.Parallel()

			ctx, cancel := context.WithTimeout(t.Context(), 1*time.Second)
			defer cancel()

			server := httptest.NewServer(testCase.handler)
			defer server.Close()

			config := &rest.Config{
				Host: server.URL,
			}
			enabled, err := IsEnabled(ctx, config)
			assert.NoError(t, err)
			assert.Equal(t, testCase.expectedEnabled, enabled)
		})
	}
}

func TestMalformedData(t *testing.T) {
	t.Parallel()

	testCases := map[string]struct {
		config        *rest.Config
		expectedError string
	}{
		"invalid URL": {
			config: &rest.Config{
				Host: "invalid.com/beacause-path-without-scheme",
			},
			expectedError: "error while building the api-server URL:",
		},
		"invalid config for HTTPClient": {
			config: &rest.Config{
				ExecProvider: &api.ExecConfig{},
				AuthProvider: &api.AuthProviderConfig{},
			},
			expectedError: "error while building the client:",
		},
		"valid config, but missing context": {
			config:        &rest.Config{},
			expectedError: "error building the request:",
		},
	}

	for testName, testCase := range testCases {
		t.Run(testName, func(t *testing.T) {
			t.Parallel()

			// turn off staticcheck to avoid SA1012 we know is an error, but is the simplest one to trigger for our use case
			enabled, err := IsEnabled(nil, testCase.config) //nolint:staticcheck
			require.False(t, enabled)
			assert.ErrorContains(t, err, testCase.expectedError)
		})
	}
}
