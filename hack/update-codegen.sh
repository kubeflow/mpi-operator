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

CURRENT_DIR=$(dirname "${BASH_SOURCE[0]}")
MPI_OPERATOR_ROOT=$(realpath "${CURRENT_DIR}/..")
MPI_OPERATOR_PKG="github.com/kubeflow/mpi-operator"
CODEGEN_PKG=$(go list -m -mod=readonly -f "{{.Dir}}" k8s.io/code-generator)

cd "${CURRENT_DIR}/.."

source "${CODEGEN_PKG}/kube_codegen.sh"

kube::codegen::gen_helpers \
  --boilerplate "${MPI_OPERATOR_ROOT}/hack/custom-boilerplate.go.txt" \
  "${MPI_OPERATOR_ROOT}/pkg/apis"

# Generating OpenAPI
cp "${MPI_OPERATOR_ROOT}/pkg/apis/kubeflow/v2beta1/zz_generated.openapi.go" \
  "${MPI_OPERATOR_ROOT}/pkg/apis/kubeflow/v2beta1/zz_generated.openapi.go.backup"

kube::codegen::gen_openapi \
  --boilerplate "${MPI_OPERATOR_ROOT}/hack/custom-boilerplate.go.txt" \
  --output-dir "${MPI_OPERATOR_ROOT}/pkg/apis/kubeflow/v2beta1" \
  --output-pkg "${MPI_OPERATOR_PKG}/pkg/apis/kubeflow/v2beta1" \
  --update-report \
  "${MPI_OPERATOR_ROOT}/pkg/apis/kubeflow/v2beta1"

kube::codegen::gen_client \
  --with-watch \
  --with-applyconfig \
  --output-dir "${MPI_OPERATOR_ROOT}/pkg/client" \
  --output-pkg "${MPI_OPERATOR_PKG}/pkg/client" \
  --boilerplate "${MPI_OPERATOR_ROOT}/hack/custom-boilerplate.go.txt" \
  "${MPI_OPERATOR_ROOT}/pkg/apis"
