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

package task

import (
	"context"
	"fmt"

	"github.com/go-logr/logr"
	pkgresource "github.com/mia-platform/jpl/pkg/resource"
	"github.com/mia-platform/jpl/pkg/runner"
	"github.com/mia-platform/jpl/pkg/util"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	utilerrors "k8s.io/apimachinery/pkg/util/errors"
	"k8s.io/cli-runtime/pkg/resource"
)

const (
	warningChangesOnDeletingResource = "WARNING: try to change %s resource which is currently being deleted."
)

// InfoFetcher function for transforming a jpl Resources in a kubernetes resource Info object
type InfoFetcher func(*unstructured.Unstructured) (resource.Info, error)

// keep it to always check if ApplyTask implement correctly the Task interface
var _ runner.Task = &ApplyTask{}

// ApplyTask will apply the Objects to a remote api-server with the server-side config of kubectl
type ApplyTask struct {
	DryRun       bool
	FieldManager string

	Objects     []*unstructured.Unstructured
	InfoFetcher InfoFetcher

	cancel context.CancelFunc
}

// Run implement the runner.Task interface
func (t *ApplyTask) Run(state runner.CurrentState) error {
	withCancel, cancel := context.WithCancel(state.GetContext())
	t.cancel = cancel
	defer t.Cancel()

	var errList []error
	for _, obj := range t.Objects {
		info, err := t.InfoFetcher(obj)
		if err != nil {
			errList = append(errList, err)
			continue
		}

		if err := applyObject(withCancel, info, t.DryRun, t.FieldManager); err != nil {
			errList = append(errList, err)
			// if the error returned is unsupported media, it means that api-server don't support server side apply
			// and so every other requests will fail as well. Bail out
			if apierrors.IsUnsupportedMediaType(err) {
				break
			}
		}
	}

	return utilerrors.NewAggregate(errList)
}

// Cancel implement the runner.Task interface
func (t *ApplyTask) Cancel() {
	if t.cancel != nil {
		t.cancel()
	}
}

// applyObject encapsulate the logic for making a PATCH request to the api-server with server side merging logic
// and strict validation of the resource fields
func applyObject(ctx context.Context, info resource.Info, dryRun bool, fieldManager string) error {
	forceConflictingFields := true
	options := &metav1.PatchOptions{
		Force:           &forceConflictingFields,
		FieldManager:    fieldManager,
		FieldValidation: metav1.FieldValidationStrict,
	}

	if dryRun {
		options.DryRun = []string{metav1.DryRunAll}
	}

	// Send the full object to be applied on the server side.
	data, err := runtime.Encode(unstructured.UnstructuredJSONScheme, info.Object)
	if err != nil {
		return err
	}

	obj, err := info.Client.Patch(types.ApplyPatchType).
		NamespaceIfScoped(info.Namespace, info.Mapping.Scope.Name() == meta.RESTScopeNameNamespace).
		Resource(info.Mapping.Resource.Resource).
		Name(info.Name).
		VersionedParams(options, metav1.ParameterCodec).
		Body(data).
		Do(ctx).
		Get()

	if err != nil {
		if apierrors.IsUnsupportedMediaType(err) {
			err = fmt.Errorf("server-side apply not available on the server: %w", err)
		}
		return err
	}

	// we ignore the error, so no need to catch it
	_ = info.Refresh(obj, true)

	warnIfDeleting(info.Object, logr.FromContextOrDiscard(ctx))
	return nil
}

func DefaultInfoFetcherBuilder(factory util.ClientFactory) (InfoFetcher, error) {
	mapper, err := factory.ToRESTMapper()
	if err != nil {
		return nil, err
	}

	return func(r *unstructured.Unstructured) (resource.Info, error) {
		info := pkgresource.Info(r)
		gvk := info.Object.GetObjectKind().GroupVersionKind()
		mapping, err := mapper.RESTMapping(gvk.GroupKind(), gvk.Version)
		if err != nil {
			return info, err
		}
		info.Mapping = mapping

		c, err := factory.UnstructuredClientForMapping(mapping)
		if err != nil {
			return info, err
		}
		info.Client = c
		return info, nil
	}, nil
}

// warnIfDeleting prints a warning if a resource is being deleted
func warnIfDeleting(obj runtime.Object, logger logr.Logger) {
	if metadata, _ := meta.Accessor(obj); metadata != nil && metadata.GetDeletionTimestamp() != nil {
		logger.Info(fmt.Sprintf(warningChangesOnDeletingResource, metadata.GetName()))
	}
}
