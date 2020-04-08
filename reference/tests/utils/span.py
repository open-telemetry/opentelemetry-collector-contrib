from ddtrace.span import Span

NO_CHILDREN = object()


class TestSpan(Span):
    """
    Test wrapper for a :class:`ddtrace.span.Span` that provides additional functions and assertions

    Example::

        span = tracer.trace('my.span')
        span = TestSpan(span)

        if span.matches(name='my.span'):
            print('matches')

        # Raises an AssertionError
        span.assert_matches(name='not.my.span', meta={'system.pid': getpid()})
    """
    def __init__(self, span):
        """
        Constructor for TestSpan

        :param span: The :class:`ddtrace.span.Span` to wrap
        :type span: :class:`ddtrace.span.Span`
        """
        if isinstance(span, TestSpan):
            span = span._span

        # DEV: Use `object.__setattr__` to by-pass this class's `__setattr__`
        object.__setattr__(self, '_span', span)

    def __getattr__(self, key):
        """
        First look for property on the base :class:`ddtrace.span.Span` otherwise return this object's attribute
        """
        if hasattr(self._span, key):
            return getattr(self._span, key)

        return self.__getattribute__(key)

    def __setattr__(self, key, value):
        """Pass through all assignment to the base :class:`ddtrace.span.Span`"""
        return setattr(self._span, key, value)

    def __eq__(self, other):
        """
        Custom equality code to ensure we are using the base :class:`ddtrace.span.Span.__eq__`

        :param other: The object to check equality with
        :type other: object
        :returns: True if equal, False otherwise
        :rtype: bool
        """
        if isinstance(other, TestSpan):
            return other._span == self._span
        elif isinstance(other, Span):
            return other == self._span
        return other == self

    def matches(self, **kwargs):
        """
        Helper function to check if this span's properties matches the expected.

        Example::

            span = TestSpan(span)
            span.matches(name='my.span', resource='GET /')

        :param kwargs: Property/Value pairs to evaluate on this span
        :type kwargs: dict
        :returns: True if the arguments passed match, False otherwise
        :rtype: bool
        """
        for name, value in kwargs.items():
            # Special case for `meta`
            if name == 'meta' and not self.meta_matches(value):
                return False

            # Ensure it has the property first
            if not hasattr(self, name):
                return False

            # Ensure the values match
            if getattr(self, name) != value:
                return False

        return True

    def meta_matches(self, meta, exact=False):
        """
        Helper function to check if this span's meta matches the expected

        Example::

            span = TestSpan(span)
            span.meta_matches({'system.pid': getpid()})

        :param meta: Property/Value pairs to evaluate on this span
        :type meta: dict
        :param exact: Whether to do an exact match on the meta values or not, default: False
        :type exact: bool
        :returns: True if the arguments passed match, False otherwise
        :rtype: bool
        """
        if exact:
            return self.meta == meta

        for key, value in meta.items():
            if key not in self.meta:
                return False
            if self.meta[key] != value:
                return False
        return True

    def assert_matches(self, **kwargs):
        """
        Assertion method to ensure this span's properties match as expected

        Example::

            span = TestSpan(span)
            span.assert_matches(name='my.span')

        :param kwargs: Property/Value pairs to evaluate on this span
        :type kwargs: dict
        :raises: AssertionError
        """
        for name, value in kwargs.items():
            # Special case for `meta`
            if name == 'meta':
                self.assert_meta(value)
            elif name == 'metrics':
                self.assert_metrics(value)
            else:
                assert hasattr(self, name), '{0!r} does not have property {1!r}'.format(self, name)
                assert getattr(self, name) == value, (
                    '{0!r} property {1}: {2!r} != {3!r}'
                    .format(self, name, getattr(self, name), value)
                )

    def assert_meta(self, meta, exact=False):
        """
        Assertion method to ensure this span's meta match as expected

        Example::

            span = TestSpan(span)
            span.assert_meta({'system.pid': getpid()})

        :param meta: Property/Value pairs to evaluate on this span
        :type meta: dict
        :param exact: Whether to do an exact match on the meta values or not, default: False
        :type exact: bool
        :raises: AssertionError
        """
        if exact:
            assert self.meta == meta
        else:
            for key, value in meta.items():
                assert key in self.meta, '{0} meta does not have property {1!r}'.format(self, key)
                assert self.meta[key] == value, (
                    '{0} meta property {1!r}: {2!r} != {3!r}'
                    .format(self, key, self.meta[key], value)
                )

    def assert_metrics(self, metrics, exact=False):
        """
        Assertion method to ensure this span's metrics match as expected

        Example::

            span = TestSpan(span)
            span.assert_metrics({'_dd1.sr.eausr': 1})

        :param metrics: Property/Value pairs to evaluate on this span
        :type metrics: dict
        :param exact: Whether to do an exact match on the metrics values or not, default: False
        :type exact: bool
        :raises: AssertionError
        """
        if exact:
            assert self.metrics == metrics
        else:
            for key, value in metrics.items():
                assert key in self.metrics, '{0} metrics does not have property {1!r}'.format(self, key)
                assert self.metrics[key] == value, (
                    '{0} metrics property {1!r}: {2!r} != {3!r}'
                    .format(self, key, self.metrics[key], value)
                )


class TestSpanContainer(object):
    """
    Helper class for a container of Spans.

    Subclasses of this class must implement a `get_spans` method::

        def get_spans(self):
            return []

    This class provides methods and assertions over a list of spans::

        class TestCases(BaseTracerTestCase):
            def test_spans(self):
                # TODO: Create spans

                self.assert_has_spans()
                self.assert_span_count(3)
                self.assert_structure( ... )

                # Grab only the `requests.request` spans
                spans = self.filter_spans(name='requests.request')
    """
    def _ensure_test_spans(self, spans):
        """
        internal helper to ensure the list of spans are all :class:`tests.utils.span.TestSpan`

        :param spans: List of :class:`ddtrace.span.Span` or :class:`tests.utils.span.TestSpan`
        :type spans: list
        :returns: A list og :class:`tests.utils.span.TestSpan`
        :rtype: list
        """
        return [
            span if isinstance(span, TestSpan) else TestSpan(span) for span in spans
        ]

    @property
    def spans(self):
        return self._ensure_test_spans(self.get_spans())

    def get_spans(self):
        """subclass required property"""
        raise NotImplementedError

    def _build_tree(self, root):
        """helper to build a tree structure for the provided root span"""
        children = []
        for span in self.spans:
            if span.parent_id == root.span_id:
                children.append(self._build_tree(span))

        return TestSpanNode(root, children)

    def get_root_span(self):
        """
        Helper to get the root span from the list of spans in this container

        :returns: The root span if one was found, None if not, and AssertionError if multiple roots were found
        :rtype: :class:`tests.utils.span.TestSpanNode`, None
        :raises: AssertionError
        """
        root = None
        for span in self.spans:
            if span.parent_id is None:
                if root is not None:
                    raise AssertionError('Multiple root spans found {0!r} {1!r}'.format(root, span))
                root = span

        assert root, 'No root span found in {0!r}'.format(self.spans)

        return self._build_tree(root)

    def get_root_spans(self):
        """
        Helper to get all root spans from the list of spans in this container

        :returns: The root spans if any were found, None if not
        :rtype: list of :class:`tests.utils.span.TestSpanNode`, None
        """
        roots = []
        for span in self.spans:
            if span.parent_id is None:
                roots.append(self._build_tree(span))

        return sorted(roots, key=lambda s: s.start)

    def assert_trace_count(self, count):
        """Assert the number of unique trace ids this container has"""
        trace_count = len(self.get_root_spans())
        assert trace_count == count, 'Trace count {0} != {1}'.format(trace_count, count)

    def assert_span_count(self, count):
        """Assert this container has the expected number of spans"""
        assert len(self.spans) == count, 'Span count {0} != {1}'.format(len(self.spans), count)

    def assert_has_spans(self):
        """Assert this container has spans"""
        assert len(self.spans), 'No spans found'

    def assert_has_no_spans(self):
        """Assert this container does not have any spans"""
        assert len(self.spans) == 0, 'Span count {0}'.format(len(self.spans))

    def filter_spans(self, *args, **kwargs):
        """
        Helper to filter current spans by provided parameters.

        This function will yield all spans whose `TestSpan.matches` function return `True`.

        :param args: Positional arguments to pass to :meth:`tests.utils.span.TestSpan.matches`
        :type args: list
        :param kwargs: Keyword arguments to pass to :meth:`tests.utils.span.TestSpan.matches`
        :type kwargs: dict
        :returns: generator for the matched :class:`tests.utils.span.TestSpan`
        :rtype: generator
        """
        for span in self.spans:
            # ensure we have a TestSpan
            if not isinstance(span, TestSpan):
                span = TestSpan(span)

            if span.matches(*args, **kwargs):
                yield span

    def find_span(self, *args, **kwargs):
        """
        Find a single span matches the provided filter parameters.

        This function will find the first span whose `TestSpan.matches` function return `True`.

        :param args: Positional arguments to pass to :meth:`tests.utils.span.TestSpan.matches`
        :type args: list
        :param kwargs: Keyword arguments to pass to :meth:`tests.utils.span.TestSpan.matches`
        :type kwargs: dict
        :returns: The first matching span
        :rtype: :class:`tests.utils.span.TestSpan`
        """
        span = next(self.filter_spans(*args, **kwargs), None)
        assert span is not None, (
            'No span found for filter {0!r} {1!r}, have {2} spans'
            .format(args, kwargs, len(self.spans))
        )
        return span


class TracerSpanContainer(TestSpanContainer):
    """
    A class to wrap a :class:`tests.utils.tracer.DummyTracer` with a
    :class:`tests.utils.span.TestSpanContainer` to use in tests
    """
    def __init__(self, tracer):
        self.tracer = tracer
        super(TracerSpanContainer, self).__init__()

    def get_spans(self):
        """
        Overridden method to return all spans attached to this tracer

        :returns: List of spans attached to this tracer
        :rtype: list
        """
        return self.tracer.writer.spans

    def reset(self):
        """Helper to reset the existing list of spans created"""
        self.tracer.writer.pop()


class TestSpanNode(TestSpan, TestSpanContainer):
    """
    A :class:`tests.utils.span.TestSpan` which is used as part of a span tree.

    Each :class:`tests.utils.span.TestSpanNode` represents the current :class:`ddtrace.span.Span`
    along with any children who have that span as it's parent.

    This class can be used to assert on the parent/child relationships between spans.

    Example::

        class TestCase(BaseTestCase):
            def test_case(self):
                # TODO: Create spans

                self.assert_structure( ... )

                tree = self.get_root_span()

                # Find the first child of the root span with the matching name
                request = tree.find_span(name='requests.request')

                # Assert the parent/child relationship of this `request` span
                request.assert_structure( ... )
    """
    def __init__(self, root, children=None):
        super(TestSpanNode, self).__init__(root)
        object.__setattr__(self, '_children', children or [])

    def get_spans(self):
        """required subclass property, returns this spans children"""
        return self._children

    def assert_structure(self, root, children=NO_CHILDREN):
        """
        Assertion to assert on the structure of this node and it's children.

        This assertion takes a dictionary of properties to assert for this node
        along with a list of assertions to make for it's children.

        Example::

            def test_case(self):
                # Assert the following structure
                #
                # One root_span, with two child_spans, one with a requests.request span
                #
                # |                  root_span                |
                # |       child_span       | |   child_span   |
                # | requests.request |
                self.assert_structure(
                    # Root span with two child_span spans
                    dict(name='root_span'),

                    (
                        # Child span with one child of it's own
                        (
                            dict(name='child_span'),

                            # One requests.request span with no children
                            (
                                dict(name='requests.request'),
                            ),
                        ),

                        # Child span with no children
                        dict(name='child_span'),
                    ),
                )

        :param root: Properties to assert for this root span, these are passed to
            :meth:`tests.utils.span.TestSpan.assert_matches`
        :type root: dict
        :param children: List of child assertions to make, if children is None then do not make any
            assertions about this nodes children. Each list element must be a list with 2 items
            the first is a ``dict`` of property assertions on that child, and the second is a ``list``
            of child assertions to make.
        :type children: list, None
        :raises:
        """
        self.assert_matches(**root)

        # Give them a way to ignore asserting on children
        if children is None:
            return
        elif children is NO_CHILDREN:
            children = ()

        spans = self.spans
        self.assert_span_count(len(children))
        for i, child in enumerate(children):
            if not isinstance(child, (list, tuple)):
                child = (child, NO_CHILDREN)

            root, _children = child
            spans[i].assert_matches(parent_id=self.span_id, trace_id=self.trace_id, _parent=self)
            spans[i].assert_structure(root, _children)

    def pprint(self):
        parts = [super(TestSpanNode, self).pprint()]
        for child in self._children:
            parts.append('-' * 20)
            parts.append(child.pprint())
        return '\r\n'.join(parts)
