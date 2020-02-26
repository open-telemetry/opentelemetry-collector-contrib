# 3d party
from django.test import TestCase

# project
from ddtrace.contrib.django.utils import quantize_key_values


class DjangoUtilsTest(TestCase):
    def test_quantize_key_values(self):
        """
        Ensure that the utility functions properly convert a dictionary object
        """
        key = {'second_key': 2, 'first_key': 1}
        result = quantize_key_values(key)
        assert len(result) == 2
        assert 'first_key' in result
        assert 'second_key' in result
