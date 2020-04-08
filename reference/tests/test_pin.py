from unittest import TestCase

import pytest

from ddtrace import Pin


class PinTestCase(TestCase):
    """TestCase for the `Pin` object that is used when an object is wrapped
    with our tracing functionalities.
    """
    def setUp(self):
        # define a simple class object
        class Obj(object):
            pass

        self.Obj = Obj

    def test_pin(self):
        # ensure a Pin can be attached to an instance
        obj = self.Obj()
        pin = Pin(service='metrics')
        pin.onto(obj)

        got = Pin.get_from(obj)
        assert got.service == pin.service
        assert got is pin

    def test_pin_find(self):
        # ensure Pin will find the first available pin

        # Override service
        obj_a = self.Obj()
        pin = Pin(service='service-a')
        pin.onto(obj_a)

        # Override service
        obj_b = self.Obj()
        pin = Pin(service='service-b')
        pin.onto(obj_b)

        # No Pin set
        obj_c = self.Obj()

        # We find the first pin (obj_b)
        pin = Pin._find(obj_c, obj_b, obj_a)
        assert pin is not None
        assert pin.service == 'service-b'

        # We find the first pin (obj_a)
        pin = Pin._find(obj_a, obj_b, obj_c)
        assert pin is not None
        assert pin.service == 'service-a'

        # We don't find a pin if none is there
        pin = Pin._find(obj_c, obj_c, obj_c)
        assert pin is None

    def test_cant_pin_with_slots(self):
        # ensure a Pin can't be attached if the __slots__ is defined
        class Obj(object):
            __slots__ = ['value']

        obj = Obj()
        obj.value = 1

        Pin(service='metrics').onto(obj)
        got = Pin.get_from(obj)
        assert got is None

    def test_cant_modify(self):
        # ensure a Pin is immutable once initialized
        pin = Pin(service='metrics')
        with pytest.raises(AttributeError):
            pin.service = 'intake'

    def test_copy(self):
        # ensure a Pin is copied when using the clone methods
        p1 = Pin(service='metrics', app='flask', tags={'key': 'value'})
        p2 = p1.clone(service='intake')
        # values are the same
        assert p1.service == 'metrics'
        assert p2.service == 'intake'
        assert p1.app == 'flask'
        assert p2.app == 'flask'
        # but it's a copy
        assert p1.tags is not p2.tags
        assert p1._config is not p2._config
        # of almost everything
        assert p1.tracer is p2.tracer

    def test_none(self):
        # ensure get_from returns None if a Pin is not available
        assert Pin.get_from(None) is None

    def test_repr(self):
        # ensure the service name is in the string representation of the Pin
        pin = Pin(service='metrics')
        assert 'metrics' in str(pin)

    def test_override(self):
        # ensure Override works for an instance object
        class A(object):
            pass

        Pin(service='metrics', app='flask').onto(A)
        a = A()
        Pin.override(a, app='django')
        assert Pin.get_from(a).app == 'django'
        assert Pin.get_from(a).service == 'metrics'

        b = A()
        assert Pin.get_from(b).app == 'flask'
        assert Pin.get_from(b).service == 'metrics'

    def test_override_missing(self):
        # ensure overriding an instance doesn't override the Class
        class A(object):
            pass

        a = A()
        assert Pin.get_from(a) is None
        Pin.override(a, service='metrics')
        assert Pin.get_from(a).service == 'metrics'

        b = A()
        assert Pin.get_from(b) is None

    def test_pin_config(self):
        # ensure `Pin` has a configuration object that can be modified
        obj = self.Obj()
        Pin.override(obj, service='metrics')
        pin = Pin.get_from(obj)
        assert pin._config is not None
        pin._config['distributed_tracing'] = True
        assert pin._config['distributed_tracing'] is True

    def test_pin_config_is_a_copy(self):
        # ensure that when a `Pin` is cloned, the config is a copy
        obj = self.Obj()
        Pin.override(obj, service='metrics')
        p1 = Pin.get_from(obj)
        assert p1._config is not None
        p1._config['distributed_tracing'] = True

        Pin.override(obj, service='intake')
        p2 = Pin.get_from(obj)
        assert p2._config is not None
        p2._config['distributed_tracing'] = False

        assert p1._config['distributed_tracing'] is True
        assert p2._config['distributed_tracing'] is False

    def test_pin_does_not_override_global(self):
        # ensure that when a `Pin` is created from a class, the specific
        # instance doesn't override the global one
        class A(object):
            pass

        Pin.override(A, service='metrics')
        global_pin = Pin.get_from(A)
        global_pin._config['distributed_tracing'] = True

        a = A()
        pin = Pin.get_from(a)
        assert pin is not None
        assert pin._config['distributed_tracing'] is True
        pin._config['distributed_tracing'] = False

        assert global_pin._config['distributed_tracing'] is True
        assert pin._config['distributed_tracing'] is False

    def test_pin_does_not_override_global_with_new_instance(self):
        # ensure that when a `Pin` is created from a class, the specific
        # instance doesn't override the global one, even if only the
        # `onto()` API has been used
        class A(object):
            pass

        pin = Pin(service='metrics')
        pin.onto(A)
        global_pin = Pin.get_from(A)
        global_pin._config['distributed_tracing'] = True

        a = A()
        pin = Pin.get_from(a)
        assert pin is not None
        assert pin._config['distributed_tracing'] is True
        pin._config['distributed_tracing'] = False

        assert global_pin._config['distributed_tracing'] is True
        assert pin._config['distributed_tracing'] is False
