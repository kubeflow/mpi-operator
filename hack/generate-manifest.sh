#!/usr/bin/env bash

# Copyright 2023 The Kubeflow Authors.
#
# Licensed under the Apache License, Version 2.0 (the "License");
# you may not use this file except in compliance with the License.
# You may obtain a copy of the License at
#
#     http://www.apache.org/licenses/LICENSE-2.0
#
# Unless required by applicable law or agreed to in writing, software
# distributed under the License is distributed on an "AS IS" BASIS,
# WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
# See the License for the specific language governing permissions and
# limitations under the License.

set -o errexit
set -o nounset
set -o pipefail

cd "$(dirname "$0")/.."
KUSTOMIZE=${1:-"bin/kustomize"}

MANIFEST=deploy/v2beta1/mpi-operator.yaml

cat <<EOF > "${MANIFEST}"
# --------------------------------------------------
# - Single configuration deployment YAML for MPI-Operator
# - Includes:
#      CRD
#      Namespace
#      RBAC
#      Controller deployment
# --------------------------------------------------
---
EOF
"${KUSTOMIZE}" build manifests/overlays/standalone >> "${MANIFEST}"
