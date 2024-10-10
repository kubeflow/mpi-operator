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

TMP_VENDOR_DIR=gen-vendor

GO_CMD=${1:-go}
CURRENT_DIR=$(dirname "${BASH_SOURCE[0]}")
MPIOP_ROOT=$(realpath "${CURRENT_DIR}/..")
MPIOP_PKG="github.com/kubeflow/mpi-operator"

# Generate clientset and informers
CODEGEN_PKG=${CODEGEN_PKG:-$(ls -d -1 ./${TMP_VENDOR_DIR}/k8s.io/code-generator 2>/dev/null || echo ../code-generator)}

cd $(dirname ${BASH_SOURCE[0]})/..

source "${CODEGEN_PKG}/kube_codegen.sh"

# Generating conversion and defaults functions
kube::codegen::gen_helpers \
  --boilerplate "${MPIOP_ROOT}/hack/custom-boilerplate.go.txt" \
  "${MPIOP_ROOT}/pkg/apis"

kube::codegen::gen_client \
  --boilerplate "${MPIOP_ROOT}/hack/boilerplate.go.txt" \
  --output-dir "${MPIOP_ROOT}/pkg/client" \
  --output-pkg "${MPIOP_PKG}/pkg/client" \
  --with-watch \
  --with-applyconfig \
  "${MPIOP_ROOT}/pkg/apis"
