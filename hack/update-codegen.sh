#!/usr/bin/env bash

# Copyright 2018 The Kubeflow Authors.
#
# Licensed under the Apache License, Version 2.0 (the "License");
# you may not use this file except in compliance with the License.
# You may obtain a copy of the License at
#
#     http:#www.apache.org/licenses/LICENSE-2.0
#
# Unless required by applicable law or agreed to in writing, software
# distributed under the License is distributed on an "AS IS" BASIS,
# WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
# See the License for the specific language governing permissions and
# limitations under the License.

set -o errexit
set -o nounset
set -o pipefail

SCRIPT_ROOT=$(dirname ${BASH_SOURCE})/..
pushd $SCRIPT_ROOT
SCRIPT_ROOT=$(pwd)
popd

# Note that we use code-generator from `${GOPATH}/pkg/mod/` because we cannot vendor it
# via `go mod vendor` to the project's /vendor directory.
# Reference: https://github.com/kubernetes/code-generator/issues/57
CODEGEN_VERSION=$(grep 'k8s.io/code-generator' go.sum | awk '{print $2}' | sed 's/\/go.mod//g' | head -1)
CODEGEN_PKG=$(echo `go env GOPATH`"/pkg/mod/k8s.io/code-generator@${CODEGEN_VERSION}")
chmod +x ${CODEGEN_PKG}/generate-groups.sh

${CODEGEN_PKG}/generate-groups.sh "deepcopy,client,informer,lister" \
  github.com/kubeflow/mpi-operator/pkg/client github.com/kubeflow/mpi-operator/pkg/apis \
  kubeflow:v1 \
  --go-header-file ${SCRIPT_ROOT}/hack/custom-boilerplate.go.txt

# Notice: The code in code-generator does not generate defaulter by default.
echo "Generating defaulters for mpi-operator/v1"
${GOPATH}/bin/defaulter-gen  --input-dirs github.com/kubeflow/mpi-operator/pkg/apis/kubeflow/v1 \
  -O zz_generated.defaults --go-header-file ${SCRIPT_ROOT}/hack/custom-boilerplate.go.txt "$@"

# v2 is in a different module
pushd v2

CODEGEN_VERSION=$(grep 'k8s.io/code-generator' go.sum | awk '{print $2}' | sed 's/\/go.mod//g' | head -1)
CODEGEN_PKG=$(echo `go env GOPATH`"/pkg/mod/k8s.io/code-generator@${CODEGEN_VERSION}")
chmod +x ${CODEGEN_PKG}/generate-groups.sh

${CODEGEN_PKG}/generate-groups.sh "deepcopy,client,informer,lister" \
  github.com/kubeflow/mpi-operator/v2/pkg/client github.com/kubeflow/mpi-operator/v2/pkg/apis \
  kubeflow:v2beta1 --go-header-file ${SCRIPT_ROOT}/hack/custom-boilerplate.go.txt

popd