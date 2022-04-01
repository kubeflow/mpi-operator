# coding: utf-8

"""
    mpijob

    Python SDK for MPI-Operator  # noqa: E501

    The version of the OpenAPI document: v2beta1
    Generated by: https://openapi-generator.tech
"""


from setuptools import setup, find_packages  # noqa: H301

with open('requirements.txt') as f:
    REQUIRES = f.readlines()

with open('test-requirements.txt') as f:
    TEST_REQUIRES = f.readlines()

setup(
	name='kubeflow-mpijob',
  	version='0.2.0',
	author="Kubeflow Authors",
	url="https://github.com/kubeflow/mpi-operator/sdk/python/v2beta1",
	description="MPI Operator Python SDK V2",
	long_description="MPI Operator Python SDK V2",
	packages=find_packages(include=("mpijob*")),
	package_data={},
	include_package_data=False,
	zip_safe=False,
	install_requires=REQUIRES,
	tests_require=TEST_REQUIRES,
	extras_require={'test': TEST_REQUIRES}
)
