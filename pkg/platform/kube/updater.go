/*
Copyright 2017 The Nuclio Authors.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package kube

import (
	"strconv"
	"time"

	"github.com/nuclio/nuclio/pkg/platform"

	"github.com/nuclio/errors"
	"github.com/nuclio/logger"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type updater struct {
	logger   logger.Logger
	platform platform.Platform
	consumer *consumer
}

func newUpdater(parentLogger logger.Logger, consumer *consumer, platform platform.Platform) (*updater, error) {
	newupdater := &updater{
		logger:   parentLogger.GetChild("updater"),
		platform: platform,
		consumer: consumer,
	}

	return newupdater, nil
}

func (u *updater) update(updateFunctionOptions *platform.UpdateFunctionOptions) error {
	u.logger.InfoWith("Updating function", "name", updateFunctionOptions.FunctionMeta.Name)

	// get specific function CR
	function, err := u.consumer.nuclioClientSet.NuclioV1beta1().
		NuclioFunctions(updateFunctionOptions.FunctionMeta.Namespace).
		Get(updateFunctionOptions.FunctionMeta.Name, metav1.GetOptions{})

	if err != nil {
		return errors.Wrap(err, "Failed to get function")
	}

	// update it with spec if passed
	if updateFunctionOptions.FunctionSpec != nil {
		function.Spec = *updateFunctionOptions.FunctionSpec

		// update the spec with a new image hash to trigger pod restart. in the future this can be removed,
		// assuming the processor can reload configuration
		function.Spec.ImageHash = strconv.Itoa(int(time.Now().UnixNano()))
	}

	// update it with status if passed
	if updateFunctionOptions.FunctionStatus != nil {
		function.Status = *updateFunctionOptions.FunctionStatus
	}

	// get clientset
	nuclioClientSet, err := u.consumer.getNuclioClientSet(updateFunctionOptions.AuthConfig)
	if err != nil {
		return errors.Wrap(err, "Failed to get nuclio clientset")
	}

	// trigger an update
	functionCreateOrUpdateTimestamp := time.Now()
	updatedFunction, err := nuclioClientSet.
		NuclioV1beta1().
		NuclioFunctions(updateFunctionOptions.FunctionMeta.Namespace).
		Update(function)
	if err != nil {
		return errors.Wrap(err, "Failed to update function CR")
	}

	// wait for the function to be ready
	if _, err = waitForFunctionReadiness(u.logger,
		u.consumer,
		updatedFunction.Namespace,
		updatedFunction.Name,
		functionCreateOrUpdateTimestamp); err != nil {
		return errors.Wrap(err, "Failed to wait for function readiness")
	}

	u.logger.InfoWith("Function updated", "functionName", updatedFunction.Name)
	return nil
}
