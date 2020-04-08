from functools import partial
from ddtrace.utils.importlib import func_name


class SomethingCallable(object):
    """
    A dummy class that implements __call__().
    """
    value = 42

    def __call__(self):
        return 'something'

    def me(self):
        return self

    @staticmethod
    def add(a, b):
        return a + b

    @classmethod
    def answer(cls):
        return cls.value


def some_function():
    """
    A function doing nothing.
    """
    return 'nothing'


def minus(a, b):
    return a - b


minus_two = partial(minus, b=2)  # partial funcs need special handling (no module)

# disabling flake8 test below, yes, declaring a func like this is bad, we know
plus_three = lambda x: x + 3  # noqa: E731


class TestContrib(object):
    """
    Ensure that contrib utility functions handles corner cases
    """
    def test_func_name(self):
        # check that func_name works on anything callable, not only funcs.
        assert 'nothing' == some_function()
        assert 'tests.contrib.test_utils.some_function' == func_name(some_function)

        f = SomethingCallable()
        assert 'something' == f()
        assert 'tests.contrib.test_utils.SomethingCallable' == func_name(f)

        assert f == f.me()
        assert 'tests.contrib.test_utils.me' == func_name(f.me)
        assert 3 == f.add(1, 2)
        assert 'tests.contrib.test_utils.add' == func_name(f.add)
        assert 42 == f.answer()
        assert 'tests.contrib.test_utils.answer' == func_name(f.answer)

        assert 'tests.contrib.test_utils.minus' == func_name(minus)
        assert 5 == minus_two(7)
        assert 'partial' == func_name(minus_two)
        assert 10 == plus_three(7)
        assert 'tests.contrib.test_utils.<lambda>' == func_name(plus_three)
