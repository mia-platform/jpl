// Copyright 2020 Mia srl
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

package deploy

type applyFunction func(clients *K8sClients, res Resource, deployConfig DeployConfig) error

// decoreteApplyResource allows to decorate your apply function with a generic number of decorator
// before calling the actual apply
func DecorateApplyResource(decorators ...func(applyFunction) applyFunction) applyFunction {
	decoratedApplyFn := applyResource
	for _, f := range decorators {
		decoratedApplyFn = f(decoratedApplyFn)
	}
	return decoratedApplyFn
}

func applyResource(clients *K8sClients, res Resource, deployConfig DeployConfig) error {
	// !TODO
	return nil
}
