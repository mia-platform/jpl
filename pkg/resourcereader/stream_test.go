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

package resourcereader

import (
	"bytes"
	"path/filepath"
	"testing"

	pkgtesting "github.com/mia-platform/jpl/pkg/testing"
	"github.com/stretchr/testify/assert"
)

func TestStreamReader(t *testing.T) {
	t.Parallel()

	testdataFolder := filepath.Join("..", "..", "testdata", "commons")
	deploymentFilename := filepath.Join(testdataFolder, "deployment.yaml")

	invalidFilename := filepath.Join("testdata", "invalid.yaml")
	multipleResources := filepath.Join("testdata", "multiple-resources.yaml")

	testCases := map[string]struct {
		manifests     []byte
		expectedCount int
		expectedError string
	}{
		"read one manifest": {
			manifests:     pkgtesting.ReadBytesFromFile(t, deploymentFilename),
			expectedCount: 1,
		},
		"read multiple manifests, filtering local ones": {
			manifests:     pkgtesting.ReadBytesFromFile(t, multipleResources),
			expectedCount: 2,
		},
		"read invalid manifest": {
			manifests:     pkgtesting.ReadBytesFromFile(t, invalidFilename),
			expectedCount: 0,
			expectedError: "fail to read from stream:",
		},
	}

	for testName, testCase := range testCases {
		t.Run(testName, func(t *testing.T) {
			reader := bytes.NewReader(testCase.manifests)
			objects, err := (&StreamReader{Reader: reader}).Read()
			switch len(testCase.expectedError) {
			default:
				assert.ErrorContains(t, err, testCase.expectedError)
			case 0:
				assert.NoError(t, err)
			}

			assert.Equal(t, testCase.expectedCount, len(objects))
		})
	}
}
