# coding: utf-8

"""
    mpijob

    Python SDK for MPI-Operator  # noqa: E501

    The version of the OpenAPI document: v2beta1
    Generated by: https://openapi-generator.tech
"""


import inspect
import pprint
import re  # noqa: F401
import six

from mpijob.configuration import Configuration


class V1Status(object):
    """NOTE: This class is auto generated by OpenAPI Generator.
    Ref: https://openapi-generator.tech

    Do not edit the class manually.
    """

    """
    Attributes:
      openapi_types (dict): The key is attribute name
                            and the value is attribute type.
      attribute_map (dict): The key is attribute name
                            and the value is json key in definition.
    """
    openapi_types = {
        'api_version': 'str',
        'code': 'int',
        'details': 'V1StatusDetails',
        'kind': 'str',
        'message': 'str',
        'metadata': 'V1ListMeta',
        'reason': 'str',
        'status': 'str'
    }

    attribute_map = {
        'api_version': 'apiVersion',
        'code': 'code',
        'details': 'details',
        'kind': 'kind',
        'message': 'message',
        'metadata': 'metadata',
        'reason': 'reason',
        'status': 'status'
    }

    def __init__(self, api_version=None, code=None, details=None, kind=None, message=None, metadata=None, reason=None, status=None, local_vars_configuration=None):  # noqa: E501
        """V1Status - a model defined in OpenAPI"""  # noqa: E501
        if local_vars_configuration is None:
            local_vars_configuration = Configuration.get_default_copy()
        self.local_vars_configuration = local_vars_configuration

        self._api_version = None
        self._code = None
        self._details = None
        self._kind = None
        self._message = None
        self._metadata = None
        self._reason = None
        self._status = None
        self.discriminator = None

        if api_version is not None:
            self.api_version = api_version
        if code is not None:
            self.code = code
        if details is not None:
            self.details = details
        if kind is not None:
            self.kind = kind
        if message is not None:
            self.message = message
        if metadata is not None:
            self.metadata = metadata
        if reason is not None:
            self.reason = reason
        if status is not None:
            self.status = status

    @property
    def api_version(self):
        """Gets the api_version of this V1Status.  # noqa: E501

        APIVersion defines the versioned schema of this representation of an object. Servers should convert recognized schemas to the latest internal value, and may reject unrecognized values. More info: https://git.k8s.io/community/contributors/devel/sig-architecture/api-conventions.md#resources  # noqa: E501

        :return: The api_version of this V1Status.  # noqa: E501
        :rtype: str
        """
        return self._api_version

    @api_version.setter
    def api_version(self, api_version):
        """Sets the api_version of this V1Status.

        APIVersion defines the versioned schema of this representation of an object. Servers should convert recognized schemas to the latest internal value, and may reject unrecognized values. More info: https://git.k8s.io/community/contributors/devel/sig-architecture/api-conventions.md#resources  # noqa: E501

        :param api_version: The api_version of this V1Status.  # noqa: E501
        :type api_version: str
        """

        self._api_version = api_version

    @property
    def code(self):
        """Gets the code of this V1Status.  # noqa: E501

        Suggested HTTP return code for this status, 0 if not set.  # noqa: E501

        :return: The code of this V1Status.  # noqa: E501
        :rtype: int
        """
        return self._code

    @code.setter
    def code(self, code):
        """Sets the code of this V1Status.

        Suggested HTTP return code for this status, 0 if not set.  # noqa: E501

        :param code: The code of this V1Status.  # noqa: E501
        :type code: int
        """

        self._code = code

    @property
    def details(self):
        """Gets the details of this V1Status.  # noqa: E501


        :return: The details of this V1Status.  # noqa: E501
        :rtype: V1StatusDetails
        """
        return self._details

    @details.setter
    def details(self, details):
        """Sets the details of this V1Status.


        :param details: The details of this V1Status.  # noqa: E501
        :type details: V1StatusDetails
        """

        self._details = details

    @property
    def kind(self):
        """Gets the kind of this V1Status.  # noqa: E501

        Kind is a string value representing the REST resource this object represents. Servers may infer this from the endpoint the client submits requests to. Cannot be updated. In CamelCase. More info: https://git.k8s.io/community/contributors/devel/sig-architecture/api-conventions.md#types-kinds  # noqa: E501

        :return: The kind of this V1Status.  # noqa: E501
        :rtype: str
        """
        return self._kind

    @kind.setter
    def kind(self, kind):
        """Sets the kind of this V1Status.

        Kind is a string value representing the REST resource this object represents. Servers may infer this from the endpoint the client submits requests to. Cannot be updated. In CamelCase. More info: https://git.k8s.io/community/contributors/devel/sig-architecture/api-conventions.md#types-kinds  # noqa: E501

        :param kind: The kind of this V1Status.  # noqa: E501
        :type kind: str
        """

        self._kind = kind

    @property
    def message(self):
        """Gets the message of this V1Status.  # noqa: E501

        A human-readable description of the status of this operation.  # noqa: E501

        :return: The message of this V1Status.  # noqa: E501
        :rtype: str
        """
        return self._message

    @message.setter
    def message(self, message):
        """Sets the message of this V1Status.

        A human-readable description of the status of this operation.  # noqa: E501

        :param message: The message of this V1Status.  # noqa: E501
        :type message: str
        """

        self._message = message

    @property
    def metadata(self):
        """Gets the metadata of this V1Status.  # noqa: E501


        :return: The metadata of this V1Status.  # noqa: E501
        :rtype: V1ListMeta
        """
        return self._metadata

    @metadata.setter
    def metadata(self, metadata):
        """Sets the metadata of this V1Status.


        :param metadata: The metadata of this V1Status.  # noqa: E501
        :type metadata: V1ListMeta
        """

        self._metadata = metadata

    @property
    def reason(self):
        """Gets the reason of this V1Status.  # noqa: E501

        A machine-readable description of why this operation is in the \"Failure\" status. If this value is empty there is no information available. A Reason clarifies an HTTP status code but does not override it.  # noqa: E501

        :return: The reason of this V1Status.  # noqa: E501
        :rtype: str
        """
        return self._reason

    @reason.setter
    def reason(self, reason):
        """Sets the reason of this V1Status.

        A machine-readable description of why this operation is in the \"Failure\" status. If this value is empty there is no information available. A Reason clarifies an HTTP status code but does not override it.  # noqa: E501

        :param reason: The reason of this V1Status.  # noqa: E501
        :type reason: str
        """

        self._reason = reason

    @property
    def status(self):
        """Gets the status of this V1Status.  # noqa: E501

        Status of the operation. One of: \"Success\" or \"Failure\". More info: https://git.k8s.io/community/contributors/devel/sig-architecture/api-conventions.md#spec-and-status  # noqa: E501

        :return: The status of this V1Status.  # noqa: E501
        :rtype: str
        """
        return self._status

    @status.setter
    def status(self, status):
        """Sets the status of this V1Status.

        Status of the operation. One of: \"Success\" or \"Failure\". More info: https://git.k8s.io/community/contributors/devel/sig-architecture/api-conventions.md#spec-and-status  # noqa: E501

        :param status: The status of this V1Status.  # noqa: E501
        :type status: str
        """

        self._status = status

    def to_dict(self, serialize=False):
        """Returns the model properties as a dict"""
        result = {}

        def convert(x):
            if hasattr(x, "to_dict"):
                args = inspect.getargspec(x.to_dict).args
                if len(args) == 1:
                    return x.to_dict()
                else:
                    return x.to_dict(serialize)
            else:
                return x

        for attr, _ in six.iteritems(self.openapi_types):
            value = getattr(self, attr)
            attr = self.attribute_map.get(attr, attr) if serialize else attr
            if isinstance(value, list):
                result[attr] = list(map(
                    lambda x: convert(x),
                    value
                ))
            elif isinstance(value, dict):
                result[attr] = dict(map(
                    lambda item: (item[0], convert(item[1])),
                    value.items()
                ))
            else:
                result[attr] = convert(value)

        return result

    def to_str(self):
        """Returns the string representation of the model"""
        return pprint.pformat(self.to_dict())

    def __repr__(self):
        """For `print` and `pprint`"""
        return self.to_str()

    def __eq__(self, other):
        """Returns true if both objects are equal"""
        if not isinstance(other, V1Status):
            return False

        return self.to_dict() == other.to_dict()

    def __ne__(self, other):
        """Returns true if both objects are not equal"""
        if not isinstance(other, V1Status):
            return True

        return self.to_dict() != other.to_dict()
