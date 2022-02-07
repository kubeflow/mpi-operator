// Copyright 2021 The Kubeflow Authors.
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

#include "mpi.h"
#include <random>
#include <cstdio>

int main(int argc, char *argv[]) {
  int rank, workers, proc_name_size;
  MPI_Init(&argc, &argv);
  MPI_Comm_rank(MPI_COMM_WORLD, &rank);
  MPI_Comm_size(MPI_COMM_WORLD, &workers);
  if (rank == 0) {
    printf("Workers: %d\n", workers);
  }
  char hostname[MPI_MAX_PROCESSOR_NAME];
  MPI_Get_processor_name(hostname, &proc_name_size);
  printf("Rank %d on host %s\n", rank, hostname);

  std::minstd_rand generator(rank);
  std::uniform_real_distribution<double> distribution(-1.0, 1.0);
  double x, y;
  long long worker_count = 0;
  int worker_tests = 10000000;
  for (int i = 0; i < worker_tests; i++) {
    x = distribution(generator);
    y = distribution(generator);
    if (x * x + y * y <= 1.0) {
      worker_count++;
    }
  }
  long long total_count = 0;
  MPI_Reduce(&worker_count, &total_count, 1, MPI_LONG_LONG, MPI_SUM, 0, MPI_COMM_WORLD);
  if (rank == 0) {
    double pi = 4 * (double)total_count / (double)(worker_tests) / (double)(workers);
    printf("pi is approximately %.16lf\n", pi);
  }
  MPI_Finalize();
  return 0;
}
