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
	"os"
	"path/filepath"
	"testing"

	pkgtesting "github.com/mia-platform/jpl/pkg/testing"
	"github.com/stretchr/testify/assert"
)

func TestFilepathReader(t *testing.T) {
	t.Parallel()

	testdataFolder := filepath.Join("..", "..", "testdata", "commons")
	deploymentFilename := filepath.Join(testdataFolder, "deployment.yaml")
	namespaceFilename := filepath.Join(testdataFolder, "namespace.yaml")

	localFilename := filepath.Join("testdata", "local.yaml")
	invalidFilename := filepath.Join("testdata", "invalid.yaml")

	testCases := map[string]struct {
		manifests     map[string][]byte
		expectedCount int
		expectedError string
	}{
		"read one manifest": {
			manifests: map[string][]byte{
				"file.yml": pkgtesting.ReadBytesFromFile(t, deploymentFilename),
			},
			expectedCount: 1,
		},
		"read multiple manifests, filtering local ones": {
			manifests: map[string][]byte{
				"file.yml":   pkgtesting.ReadBytesFromFile(t, deploymentFilename),
				"file2.yaml": pkgtesting.ReadBytesFromFile(t, namespaceFilename),
				"file3.yml":  pkgtesting.ReadBytesFromFile(t, localFilename),
				"ignored":    pkgtesting.ReadBytesFromFile(t, invalidFilename),
			},
			expectedCount: 2,
		},
		"read invalid manifest": {
			manifests: map[string][]byte{
				"invalid.yaml": pkgtesting.ReadBytesFromFile(t, invalidFilename),
			},
			expectedCount: 0,
			expectedError: "fail to read from path",
		},
	}

	for testName, testCase := range testCases {
		t.Run(testName, func(t *testing.T) {
			dir := t.TempDir()

			for filePath, content := range testCase.manifests {
				p := filepath.Join(dir, filePath)
				err := os.WriteFile(p, content, 0600)
				assert.NoError(t, err)
			}

			objects, err := (&FilepathReader{Path: dir}).Read()
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
