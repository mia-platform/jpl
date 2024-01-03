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
	"fmt"
	"net/http"

	flowcontrolapi "k8s.io/api/flowcontrol/v1beta3"
	"k8s.io/client-go/rest"
)

// wellKnownPath is the path to call for receiving the headers
const wellKnownPath = "/livez/ping"

// IsEnabled return if the flowcontrol APIs are enabled on the target cluster. It will perform a HEAD request against
// a wellKnownPath endpoint and check if the flowcontrolapi.ResponseHeaderMatchedFlowSchemaUID header is present.
func IsEnabled(ctx context.Context, config *rest.Config) (bool, error) {
	// get a plain Client from the rest.Config because we want to access the response headers directly
	// and a normal client will hide that information
	client, err := rest.HTTPClientFor(config)
	if err != nil {
		return false, fmt.Errorf("error while building the client: %w", err)
	}

	// asks the server URL from Config for building the call to the kubernetes server
	url, _, err := rest.DefaultServerUrlFor(config)
	if err != nil {
		return false, fmt.Errorf("error while building the api-server URL: %w", err)
	}

	url.Path = wellKnownPath
	request, err := http.NewRequestWithContext(ctx, http.MethodHead, url.String(), nil)
	if err != nil {
		return false, fmt.Errorf("error building the request: %w", err)
	}

	response, err := client.Do(request)
	if err != nil {
		return false, fmt.Errorf("error making %q request: %w", wellKnownPath, err)
	}

	if response.Body != nil {
		if err := response.Body.Close(); err != nil {
			return false, fmt.Errorf("error closing body: %w", err)
		}
	}

	// check if flowcontrolapi.ResponseHeaderMatchedFlowSchemaUID is present between the response headers,
	// there are two headers, but they're always both set by FlowControl.
	// We don't care about the value but only that is present.
	return response.Header.Get(flowcontrolapi.ResponseHeaderMatchedFlowSchemaUID) != "", nil
}
