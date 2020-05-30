import datetime
import logging
import os
import uuid

import pytest

from kubeflow.mpioperator.notebook_tests import nb_test_util
from kubeflow.testing import util

def test_mnist_gcp(record_xml_attribute, name, namespace, # pylint: disable=too-many-branches,too-many-statements
                   repos, image):
  '''Generate Job and summit.'''
  util.set_pytest_junit(record_xml_attribute, "test_mpioperator")

  if not name:
    name = "mpioperator_notebook-" + datetime.datetime.now().strftime("%H%M%S") + "-"
    name = name + uuid.uuid4().hex[0:3]

  util.set_pytest_junit(record_xml_attribute, "test_mpioperator_notebook")

  notebook_path = "kubeflow/mpi-operator/examples/v1alpha2/mpi_notebook.ipynb"
  nb_test_util.run_papermill_job(notebook_path, name, namespace, repos, image)


if __name__ == "__main__":
  logging.basicConfig(level=logging.INFO,
                      format=('%(levelname)s|%(asctime)s'
                              '|%(pathname)s|%(lineno)d| %(message)s'),
                      datefmt='%Y-%m-%dT%H:%M:%S',
                      )
  logging.getLogger().setLevel(logging.INFO)
  pytest.main()