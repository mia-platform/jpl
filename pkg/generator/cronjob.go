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

package generator

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"

	"github.com/mia-platform/jpl/pkg/client/cache"
	batchv1 "k8s.io/api/batch/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

// NewJobGenerator return a new generator.Interface that will create a Job from a CronJob mimicking the function of
// 'kubectl create job --from cronjob/'. It will done that only on CronJobs that will have a certain value in an
// annotation.
func NewJobGenerator(annotation string, value string) Interface {
	return &jobGenerator{
		annotation: annotation,
		value:      value,
	}
}

// keep it to always check if jobGenerator implement correctly the generator.Interface interface
var _ Interface = &jobGenerator{}

type jobGenerator struct {
	annotation string
	value      string
}

var cronJobGK = schema.GroupKind{
	Group: batchv1.GroupName,
	Kind:  "CronJob",
}

// CanHandleResource implement generator.Interface interface
func (g *jobGenerator) CanHandleResource(objMeta *metav1.PartialObjectMetadata) bool {
	if objMeta.GroupVersionKind().GroupKind() == cronJobGK {
		if value := objMeta.Annotations[g.annotation]; value == g.value {
			return true
		}
	}

	return false
}

// Generate implement generator.Interface interface
func (g *jobGenerator) Generate(obj *unstructured.Unstructured, _ cache.RemoteResourceGetter) ([]*unstructured.Unstructured, error) {
	var cronJob *batchv1.CronJob
	if err := runtime.DefaultUnstructuredConverter.FromUnstructuredWithValidation(obj.Object, &cronJob, true); err != nil {
		return nil, err
	}

	job := createJobFromCronJob(cronJob)
	unstruct, err := runtime.DefaultUnstructuredConverter.ToUnstructured(job)
	if err != nil {
		return nil, err
	}
	return []*unstructured.Unstructured{
		{Object: unstruct},
	}, nil
}

func createJobFromCronJob(cronJob *batchv1.CronJob) *batchv1.Job {
	annotations := make(map[string]string)
	annotations["cronjob.kubernetes.io/instantiate"] = "manual"
	for k, v := range cronJob.Spec.JobTemplate.Annotations {
		annotations[k] = v
	}

	b := make([]byte, 10)
	_, _ = rand.Read(b)

	job := &batchv1.Job{
		// this is ok because we know exactly how we want to be serialized
		TypeMeta: metav1.TypeMeta{APIVersion: batchv1.SchemeGroupVersion.String(), Kind: "Job"},
		ObjectMeta: metav1.ObjectMeta{
			Name:        fmt.Sprintf("%s-%s", cronJob.Name, hex.EncodeToString(b))[:min(64, len(cronJob.Name)+6)],
			Namespace:   cronJob.Namespace,
			Annotations: annotations,
			Labels:      cronJob.Spec.JobTemplate.Labels,
		},
		Spec: cronJob.Spec.JobTemplate.Spec,
	}

	return job
}
