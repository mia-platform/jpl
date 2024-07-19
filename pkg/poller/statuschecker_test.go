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
	"path/filepath"
	"testing"
	"time"

	pkgtesting "github.com/mia-platform/jpl/pkg/testing"
	"github.com/stretchr/testify/assert"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

func TestStatusCheck(t *testing.T) {
	t.Parallel()

	testdata := "testdata"
	tests := map[string]struct {
		object         *unstructured.Unstructured
		expectedResult *Result
		expectedError  string
	}{
		"resource in deletion return terminating status": {
			object:         pkgtesting.UnstructuredFromFile(t, filepath.Join(testdata, "deletion.yaml")),
			expectedResult: terminatingResult(deletionMessage),
		},
		"resource with matched generations and no other status is current": {
			object:         pkgtesting.UnstructuredFromFile(t, filepath.Join(testdata, "generationMatched.yaml")),
			expectedResult: currentResult(currentMessage),
		},
		"resource with mismatched generations is in progress": {
			object:         pkgtesting.UnstructuredFromFile(t, filepath.Join(testdata, "generationMismatched.yaml")),
			expectedResult: inProgressResult(fmt.Sprintf(generationMessageFormat, "PodDisruptionBudget", 2, 1)),
		},
		"resource with stalled condition set to true return failed result": {
			object:         pkgtesting.UnstructuredFromFile(t, filepath.Join(testdata, "stalled.yaml")),
			expectedResult: failedResult("custom message"),
		},
		"resource with reconciling condition set to true return in progress result": {
			object:         pkgtesting.UnstructuredFromFile(t, filepath.Join(testdata, "reconciling.yaml")),
			expectedResult: inProgressResult("custom message"),
		},
		"resource with ready condition set to true return current result": {
			object:         pkgtesting.UnstructuredFromFile(t, filepath.Join(testdata, "readyTrue.yaml")),
			expectedResult: currentResult(currentMessage),
		},
		"resource with ready condition set to false return in progress result": {
			object:         pkgtesting.UnstructuredFromFile(t, filepath.Join(testdata, "readyFalse.yaml")),
			expectedResult: inProgressResult("custom message"),
		},
		"resource with ready condition set to unknown return in progress result": {
			object:         pkgtesting.UnstructuredFromFile(t, filepath.Join(testdata, "readyUnknown.yaml")),
			expectedResult: inProgressResult("custom message"),
		},
		"resource without any passing checks return current": {
			object:         pkgtesting.UnstructuredFromFile(t, filepath.Join(testdata, "noStatus.yaml")),
			expectedResult: currentResult(currentMessage),
		},
		"crd with no status is considered in progress": {
			object:         pkgtesting.UnstructuredFromFile(t, filepath.Join(testdata, "crdInProgress.yaml")),
			expectedResult: inProgressResult(crdInProgressMessage),
		},
		"crd with Established false is considered in progress": {
			object:         pkgtesting.UnstructuredFromFile(t, filepath.Join(testdata, "crdInstalling.yaml")),
			expectedResult: inProgressResult(crdInProgressMessage),
		},
		"crd established is considered current": {
			object:         pkgtesting.UnstructuredFromFile(t, filepath.Join(testdata, "crdEstablished.yaml")),
			expectedResult: currentResult(crdCurrentMessage),
		},
		"crd names not accepted is considered failed": {
			object:         pkgtesting.UnstructuredFromFile(t, filepath.Join(testdata, "crdNamesNotAccepted.yaml")),
			expectedResult: failedResult("custom message"),
		},
		"service not LB is considered current": {
			object:         pkgtesting.UnstructuredFromFile(t, filepath.Join(testdata, "serviceClusterIP.yaml")),
			expectedResult: currentResult(currentMessage),
		},
		"service LB without ingress in status is considered in progress": {
			object:         pkgtesting.UnstructuredFromFile(t, filepath.Join(testdata, "serviceLBNotReady.yaml")),
			expectedResult: inProgressResult(serviceInProgressMessage),
		},
		"service LB with ingress in status is considered current": {
			object:         pkgtesting.UnstructuredFromFile(t, filepath.Join(testdata, "serviceLBReady.yaml")),
			expectedResult: currentResult(currentMessage),
		},
		"service LB without status is considered in progress": {
			object:         pkgtesting.UnstructuredFromFile(t, filepath.Join(testdata, "serviceLBNoStatus.yaml")),
			expectedResult: inProgressResult(serviceInProgressMessage),
		},
		"pvc without status is considered in progress": {
			object:         pkgtesting.UnstructuredFromFile(t, filepath.Join(testdata, "pvcNoStatus.yaml")),
			expectedResult: inProgressResult(fmt.Sprintf(pvcInProgressMessageFormat, "unknown")),
		},
		"pvc with Pending phase is considered in progress": {
			object:         pkgtesting.UnstructuredFromFile(t, filepath.Join(testdata, "pvcNotBound.yaml")),
			expectedResult: inProgressResult(fmt.Sprintf(pvcInProgressMessageFormat, "Pending")),
		},
		"pvc with Bound phase is considered current": {
			object:         pkgtesting.UnstructuredFromFile(t, filepath.Join(testdata, "pvcBound.yaml")),
			expectedResult: currentResult(pvcCurrentMessage),
		},
		"job without state is in progress because is not started": {
			object:         pkgtesting.UnstructuredFromFile(t, filepath.Join(testdata, "jobNoStatus.yaml")),
			expectedResult: inProgressResult(jobInProgressMessage),
		},
		"job with completed condition is current": {
			object:         pkgtesting.UnstructuredFromFile(t, filepath.Join(testdata, "jobCompleted.yaml")),
			expectedResult: currentResult(fmt.Sprintf(jobCurrentMessageFormat, 1, 1)),
		},
		"job with failed condition is failed": {
			object:         pkgtesting.UnstructuredFromFile(t, filepath.Join(testdata, "jobFailed.yaml")),
			expectedResult: failedResult("custom message"),
		},
		"job with suspended condition is current": {
			object:         pkgtesting.UnstructuredFromFile(t, filepath.Join(testdata, "jobSuspended.yaml")),
			expectedResult: currentResult(jobSuspendedMessage),
		},
		"job in progress is in progress": {
			object:         pkgtesting.UnstructuredFromFile(t, filepath.Join(testdata, "jobInProgress.yaml")),
			expectedResult: inProgressResult(fmt.Sprintf(jobRunningMessage, 3, 2, 0)),
		},
		"pod without status is in progress": {
			object:         pkgtesting.UnstructuredFromFile(t, filepath.Join(testdata, "podNoStatus.yaml")),
			expectedResult: inProgressResult(podInProgressMessage),
		},
		"pod with ready condition is current": {
			object:         pkgtesting.UnstructuredFromFile(t, filepath.Join(testdata, "podReady.yaml")),
			expectedResult: currentResult(podReadyMessage),
		},
		"pod completed with success is current": {
			object:         pkgtesting.UnstructuredFromFile(t, filepath.Join(testdata, "podCompletedSuccess.yaml")),
			expectedResult: currentResult(podSuccededMessage),
		},
		"pod completed with error is failed": {
			object:         pkgtesting.UnstructuredFromFile(t, filepath.Join(testdata, "podCompletedFail.yaml")),
			expectedResult: failedResult(podFailedMessage),
		},
		"pod with containers in crashloop is failed": {
			object:         pkgtesting.UnstructuredFromFile(t, filepath.Join(testdata, "podCrashLoop.yaml")),
			expectedResult: failedResult(fmt.Sprintf(crashloopingContainersMessageFormat, "nginx")),
		},
		"pod unscheduled is failed if creation timestamp is too old": {
			object: func() *unstructured.Unstructured {
				obj := pkgtesting.UnstructuredFromFile(t, filepath.Join(testdata, "podUnscheduled.yaml"))
				obj.SetCreationTimestamp(metav1.NewTime(time.Now().Add(-1 * time.Minute)))
				return obj
			}(),
			expectedResult: failedResult("custom message"),
		},
		"pod unscheduled is in progress if creation timestamp is less than unscheduledWindow": {
			object: func() *unstructured.Unstructured {
				obj := pkgtesting.UnstructuredFromFile(t, filepath.Join(testdata, "podUnscheduled.yaml"))
				obj.SetCreationTimestamp(metav1.NewTime(time.Now()))
				return obj
			}(),
			expectedResult: inProgressResult(podUnscheduledMessage),
		},
		"deployment without status is in progress": {
			object:         pkgtesting.UnstructuredFromFile(t, filepath.Join(testdata, "deployNoStatus.yaml")),
			expectedResult: inProgressResult(fmt.Sprintf(deploymentFewReplicasMessageFormat, 0, 1)),
		},
		"deployment correctly rollout is current": {
			object:         pkgtesting.UnstructuredFromFile(t, filepath.Join(testdata, "deployCurrent.yaml")),
			expectedResult: currentResult(fmt.Sprintf(deploymentCurrentMessageFormat, 1)),
		},
		"deployment not progressing is in progress": {
			object:         pkgtesting.UnstructuredFromFile(t, filepath.Join(testdata, "deployNotProgressing.yaml")),
			expectedResult: inProgressResult(deploymentNotProgressingMessage),
		},
		"deployment not progressing without deadline is current": {
			object:         pkgtesting.UnstructuredFromFile(t, filepath.Join(testdata, "deployNoDeadline.yaml")),
			expectedResult: currentResult(fmt.Sprintf(deploymentCurrentMessageFormat, 1)),
		},
		"deployment not available is in progress": {
			object:         pkgtesting.UnstructuredFromFile(t, filepath.Join(testdata, "deployNotAvailable.yaml")),
			expectedResult: inProgressResult(deploymentNotAvailableMessage),
		},
		"deployment with deadline exceeded is failed": {
			object:         pkgtesting.UnstructuredFromFile(t, filepath.Join(testdata, "deployDeadlineExceeded.yaml")),
			expectedResult: failedResult(deploymentDeadlineMessage),
		},
		"deployment with less updating replicas is in progress": {
			object:         pkgtesting.UnstructuredFromFile(t, filepath.Join(testdata, "deployUpdating.yaml")),
			expectedResult: inProgressResult(fmt.Sprintf(deploymentUpdatingReplicasMessageFormat, 2, 4)),
		},
		"deployment with more replicas in status is in progress": {
			object:         pkgtesting.UnstructuredFromFile(t, filepath.Join(testdata, "deployDeleting.yaml")),
			expectedResult: inProgressResult(fmt.Sprintf(deploymentDeletingReplicasMessageFormat, 2)),
		},
		"deployment with less available replicas is in progress": {
			object:         pkgtesting.UnstructuredFromFile(t, filepath.Join(testdata, "deployLessAvailable.yaml")),
			expectedResult: inProgressResult(fmt.Sprintf(deploymentWaitingReplicasMessageFormat, 4, 6)),
		},
		"deployment with less ready replicas is in progress": {
			object:         pkgtesting.UnstructuredFromFile(t, filepath.Join(testdata, "deployLessReady.yaml")),
			expectedResult: inProgressResult(fmt.Sprintf(deploymentWaitingReplicasMessageFormat, 2, 4)),
		},
		"daemonset with no status is in progress": {
			object:         pkgtesting.UnstructuredFromFile(t, filepath.Join(testdata, "dsNoStatus.yaml")),
			expectedResult: inProgressResult(daemonsetInProgressMessage),
		},
		"daemonset correctly rolled out is current": {
			object:         pkgtesting.UnstructuredFromFile(t, filepath.Join(testdata, "dsCurrent.yaml")),
			expectedResult: currentResult(fmt.Sprintf(daemonsetCurrentMessageFormat, 4)),
		},
		"daemonset with less ready replicas is in progress": {
			object:         pkgtesting.UnstructuredFromFile(t, filepath.Join(testdata, "dsLessReady.yaml")),
			expectedResult: inProgressResult(fmt.Sprintf(daemonsetWaitingReplicasMessageFormat, 2, 4)),
		},
		"daemonset with less available replicas is in progress": {
			object:         pkgtesting.UnstructuredFromFile(t, filepath.Join(testdata, "dsLessAvailable.yaml")),
			expectedResult: inProgressResult(fmt.Sprintf(daemonsetWaitingReplicasMessageFormat, 2, 4)),
		},
		"daemonset with less updated replicas is in progress": {
			object:         pkgtesting.UnstructuredFromFile(t, filepath.Join(testdata, "dsUpdating.yaml")),
			expectedResult: inProgressResult(fmt.Sprintf(daemonsetUpdatingReplicasMessageFormat, 2, 4)),
		},
		"daemonset with less current replicas is in progress": {
			object:         pkgtesting.UnstructuredFromFile(t, filepath.Join(testdata, "dsRolling.yaml")),
			expectedResult: inProgressResult(fmt.Sprintf(daemonsetFewReplicasMessageFormat, 2, 4)),
		},
		"statefulset with no status is in progress": {
			object:         pkgtesting.UnstructuredFromFile(t, filepath.Join(testdata, "stsNoStatus.yaml")),
			expectedResult: inProgressResult(fmt.Sprintf(statefulSetFewReplicasMessageFormat, 0, 1)),
		},
		"statefulset correctly rolled out is current": {
			object:         pkgtesting.UnstructuredFromFile(t, filepath.Join(testdata, "stsCurrent.yaml")),
			expectedResult: currentResult(fmt.Sprintf(statefulSetCurrentMessageFormat, 4)),
		},
		"statefulset with less ready replicas is in progress": {
			object:         pkgtesting.UnstructuredFromFile(t, filepath.Join(testdata, "stsLessReady.yaml")),
			expectedResult: inProgressResult(fmt.Sprintf(statefulSetWaitingReplicasMessageFormat, 2, 4)),
		},
		"statefulset with less current replicas is in progress": {
			object:         pkgtesting.UnstructuredFromFile(t, filepath.Join(testdata, "stsLessCurrent.yaml")),
			expectedResult: inProgressResult(fmt.Sprintf(statefulSetUpdatingReplicasMessageFormat, 2, 4)),
		},
		"statefulset removing replicas is in progress": {
			object:         pkgtesting.UnstructuredFromFile(t, filepath.Join(testdata, "stsDeleting.yaml")),
			expectedResult: inProgressResult(fmt.Sprintf(statefulSetDeletingReplicasMessageFormat, 4)),
		},
		"statefulset with mismatched generation is in progress": {
			object:         pkgtesting.UnstructuredFromFile(t, filepath.Join(testdata, "stsMistached.yaml")),
			expectedResult: inProgressResult(statefulSetRevisionMismatchMessage),
		},
		"statefulset with OnDelete strategy is current": {
			object:         pkgtesting.UnstructuredFromFile(t, filepath.Join(testdata, "stsOnDelete.yaml")),
			expectedResult: currentResult(statefulSetOnDeleteStrategyMessage),
		},
		"statefulset with partion rolling and less update replicas is in progress": {
			object:         pkgtesting.UnstructuredFromFile(t, filepath.Join(testdata, "stsPartition.yaml")),
			expectedResult: inProgressResult(fmt.Sprintf(statefulSetPartitionRolloutMessageFormat, 2, 3)),
		},
		"statefulset with partion rolling and correct updated replica is current": {
			object:         pkgtesting.UnstructuredFromFile(t, filepath.Join(testdata, "stsPartitionCurrent.yaml")),
			expectedResult: currentResult(fmt.Sprintf(statefulSetPartitionCompleteMessageFormat, 3)),
		},
	}

	for testName, testCase := range tests {
		t.Run(testName, func(t *testing.T) {
			result, err := statusCheck(testCase.object)
			if len(testCase.expectedError) > 0 {
				assert.ErrorContains(t, err, testCase.expectedError)
				assert.Equal(t, testCase.expectedResult, result)
				return
			}

			assert.NoError(t, err)
			assert.Equal(t, testCase.expectedResult, result)
		})
	}
}

func TestConditionsCheck(t *testing.T) {
	t.Parallel()

	testdata := "testdata"
	tests := map[string]struct {
		object         *unstructured.Unstructured
		expectedResult *Result
		expectedError  string
	}{
		"resource with other conditions return nil": {
			object:         pkgtesting.UnstructuredFromFile(t, filepath.Join(testdata, "noReadyCondition.yaml")),
			expectedResult: nil,
		},
		"resource without status return nil": {
			object:         pkgtesting.UnstructuredFromFile(t, filepath.Join(testdata, "noStatus.yaml")),
			expectedResult: nil,
		},
		"resource without malformed status condition return error": {
			object:         pkgtesting.UnstructuredFromFile(t, filepath.Join(testdata, "malformed.yaml")),
			expectedResult: nil,
			expectedError:  ".status.conditions accessor error",
		},
	}

	for testName, testCase := range tests {
		t.Run(testName, func(t *testing.T) {
			result, err := checkStatusConditions(testCase.object)
			if len(testCase.expectedError) > 0 {
				assert.ErrorContains(t, err, testCase.expectedError)
				assert.Equal(t, testCase.expectedResult, result)
				return
			}

			assert.NoError(t, err)
			assert.Equal(t, testCase.expectedResult, result)
		})
	}
}
