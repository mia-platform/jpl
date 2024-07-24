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

//go:generate ${TOOLS_BIN}/stringer -type=Status -trimprefix=Status
type Status int

const (
	StatusInProgress Status = iota
	StatusFailed
	StatusTerminating
	StatusCurrent
)

type Result struct {
	Status  Status
	Message string
}

func currentResult(message string) *Result {
	return &Result{
		Status:  StatusCurrent,
		Message: message,
	}
}

func inProgressResult(message string) *Result {
	return &Result{
		Status:  StatusInProgress,
		Message: message,
	}
}

func terminatingResult(message string) *Result {
	return &Result{
		Status:  StatusTerminating,
		Message: message,
	}
}

func failedResult(message string) *Result {
	return &Result{
		Status:  StatusFailed,
		Message: message,
	}
}
