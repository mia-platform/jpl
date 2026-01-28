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
	"os"

	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/cli-runtime/pkg/resource"
	"k8s.io/client-go/util/csaupgrade"
	"sigs.k8s.io/structured-merge-diff/v6/fieldpath"

	"github.com/mia-platform/jpl/pkg/client/cache"
	"github.com/mia-platform/jpl/pkg/event"
	"github.com/mia-platform/jpl/pkg/filter"
	pkgresource "github.com/mia-platform/jpl/pkg/resource"
	"github.com/mia-platform/jpl/pkg/runner"
	"github.com/mia-platform/jpl/pkg/util"
)

const (
	maxPatchRetry = 5

	warningMigrationPatchFailed      = "WARNING: server rejected managed fields migration to Server-Side Apply. This is non-fatal and will be retried next time you apply. Error: %[1]s\n"
	warningChangesOnDeletingResource = "WARNING: try to change %[1]s resource which is currently being deleted.\n"
	warningMigrationReapplyFailed    = "WARNING: failed to re-apply configuration after performing Server-Side Apply migration. This is non-fatal and will be retried next time you apply. Error: %[1]s\n"
)

var (
	lastAppliedAnnotationFieldPath = fieldpath.NewSet(
		fieldpath.MakePathOrDie(
			"metadata", "annotations",
			corev1.LastAppliedConfigAnnotation),
	)
)

// InfoFetcher function for transforming a jpl Resources in a kubernetes resource Info object
type InfoFetcher func(*unstructured.Unstructured) (*resource.Info, error)

// keep it to always check if ApplyTask implement correctly the Task interface
var _ runner.Task = &ApplyTask{}

// ApplyTask will apply the Objects to a remote api-server with the server-side config of kubectl
type ApplyTask struct {
	DryRun       bool
	FieldManager string

	RemoteGetter cache.RemoteResourceGetter
	Objects      []*unstructured.Unstructured
	Filters      []filter.Interface
	InfoFetcher  InfoFetcher
}

// Run implement the runner.Task interface
func (t *ApplyTask) Run(state runner.State) {
	ctx := state.GetContext()

	for _, obj := range t.Objects {
		filteredObj := false
		for _, filter := range t.Filters {
			filtered, filterError := filter.Filter(obj, t.RemoteGetter)
			if filterError != nil {
				state.SendEvent(applyEvent(event.StatusFailed, obj, filterError))
				filteredObj = true
				break
			}

			if filtered {
				state.SendEvent(skippedEvent(obj))
				filteredObj = true
				break
			}
		}

		if filteredObj {
			continue
		}

		state.SendEvent(applyEvent(event.StatusPending, obj, nil))
		info, err := t.InfoFetcher(obj)
		if err != nil {
			state.SendEvent(applyEvent(event.StatusFailed, obj, err))
			continue
		}

		if err := applyObject(ctx, info, t.DryRun, t.FieldManager); err != nil {
			state.SendEvent(applyEvent(event.StatusFailed, obj, err))
			// if the error returned is unsupported media, it means that api-server don't support server side apply
			// and so every other requests will fail as well. Bail out
			if apierrors.IsUnsupportedMediaType(err) {
				break
			}
			continue
		}

		state.SendEvent(applyEvent(event.StatusSuccessful, obj, nil))
	}
}

// applyEvent create an Event for an apply action with the passed object and status
func applyEvent(status event.Status, obj *unstructured.Unstructured, err error) event.Event {
	return event.Event{
		Type: event.TypeApply,
		ApplyInfo: event.ApplyInfo{
			Status: status,
			Object: obj,
			Error:  err,
		},
	}
}

func skippedEvent(obj *unstructured.Unstructured) event.Event {
	return event.Event{
		Type: event.TypeApply,
		ApplyInfo: event.ApplyInfo{
			Status: event.StatusSkipped,
			Object: obj,
			Error:  nil,
		},
	}
}

// applyObject encapsulate the logic for making a PATCH request to the api-server with server side merging logic
// and strict validation of the resource fields
func applyObject(ctx context.Context, info *resource.Info, dryRun bool, fieldManager string) error {
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

	if migrated, err := migrateToSSAIfNecessary(ctx, info, fieldManager); err != nil {
		fmt.Fprintf(os.Stderr, warningMigrationPatchFailed, err.Error())
	} else if migrated {
		if _, err := info.Client.Patch(types.ApplyPatchType).
			NamespaceIfScoped(info.Namespace, info.Mapping.Scope.Name() == meta.RESTScopeNameNamespace).
			Resource(info.Mapping.Resource.Resource).
			Name(info.Name).
			VersionedParams(options, metav1.ParameterCodec).
			Body(data).
			Do(ctx).
			Get(); err != nil {
			fmt.Fprintf(os.Stderr, warningMigrationReapplyFailed, err.Error())
		}
	} else {
		_ = info.Refresh(obj, true)
	}

	warnIfDeleting(info.Object)
	return nil
}

func DefaultInfoFetcherBuilder(factory util.ClientFactory) (InfoFetcher, error) {
	mapper, err := factory.ToRESTMapper()
	if err != nil {
		return nil, err
	}

	return func(r *unstructured.Unstructured) (*resource.Info, error) {
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

// migrateToSSAIfNecessary check if the returned object needs to have its kubectl-client-side-apply
// managed fields migrated server-side-apply.
func migrateToSSAIfNecessary(ctx context.Context, info *resource.Info, fieldManager string) (bool, error) {
	accessor, err := meta.Accessor(info.Object)
	if err != nil {
		return false, err
	}

	csaManagers := csaupgrade.FindFieldsOwners(
		accessor.GetManagedFields(),
		metav1.ManagedFieldsOperationUpdate,
		lastAppliedAnnotationFieldPath)

	managerNames := sets.New(fieldManager)
	for _, entry := range csaManagers {
		managerNames.Insert(entry.Manager)
	}

	// Re-attempt patch as many times as it is conflicting due to ResourceVersion
	// test failing
	for range maxPatchRetry {
		var patchData []byte
		var obj runtime.Object

		patchData, err = csaupgrade.UpgradeManagedFieldsPatch(
			info.Object, managerNames, fieldManager)

		if err != nil {
			// If patch generation failed there was likely a bug.
			return false, err
		} else if patchData == nil {
			// nil patch data means nothing to do - object is already migrated
			return false, nil
		}

		options := &metav1.PatchOptions{
			FieldManager: fieldManager,
		}

		obj, err = info.Client.Patch(types.JSONPatchType).
			NamespaceIfScoped(info.Namespace, info.Mapping.Scope.Name() == meta.RESTScopeNameNamespace).
			Resource(info.Mapping.Resource.Resource).
			Name(info.Name).
			VersionedParams(options, metav1.ParameterCodec).
			Body(patchData).
			Do(ctx).
			Get()

		if err == nil {
			// Stop retrying upon success.
			_ = info.Refresh(obj, true)
			return true, nil
		} else if !apierrors.IsConflict(err) {
			// Only retry if there was a conflict
			return false, err
		}

		// Refresh the object for next iteration
		err = info.Get()
		if err != nil {
			// If there was an error fetching, return error
			return false, err
		}
	}

	// Reaching this point with non-nil error means there was a conflict and
	// max retries was hit
	// Return the last error witnessed (which will be a conflict)
	return false, err
}

// warnIfDeleting prints a warning if a resource is being deleted
func warnIfDeleting(obj runtime.Object) {
	if metadata, _ := meta.Accessor(obj); metadata != nil && metadata.GetDeletionTimestamp() != nil {
		fmt.Fprintf(os.Stderr, warningChangesOnDeletingResource, metadata.GetName())
	}
}
