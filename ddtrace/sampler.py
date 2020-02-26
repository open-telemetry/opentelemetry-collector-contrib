"""Samplers manage the client-side trace sampling

Any `sampled = False` trace won't be written, and can be ignored by the instrumentation.
"""
import abc

from .compat import iteritems, pattern_type
from .constants import ENV_KEY
from .constants import SAMPLING_AGENT_DECISION, SAMPLING_RULE_DECISION, SAMPLING_LIMIT_DECISION
from .ext.priority import AUTO_KEEP, AUTO_REJECT
from .internal.logger import get_logger
from .internal.rate_limiter import RateLimiter
from .utils.formats import get_env
from .vendor import six

log = get_logger(__name__)

MAX_TRACE_ID = 2 ** 64

# Has to be the same factor and key as the Agent to allow chained sampling
KNUTH_FACTOR = 1111111111111111111


class BaseSampler(six.with_metaclass(abc.ABCMeta)):
    @abc.abstractmethod
    def sample(self, span):
        pass


class BasePrioritySampler(six.with_metaclass(abc.ABCMeta)):
    @abc.abstractmethod
    def update_rate_by_service_sample_rates(self, sample_rates):
        pass


class AllSampler(BaseSampler):
    """Sampler sampling all the traces"""

    def sample(self, span):
        return True


class RateSampler(BaseSampler):
    """Sampler based on a rate

    Keep (100 * `sample_rate`)% of the traces.
    It samples randomly, its main purpose is to reduce the instrumentation footprint.
    """

    def __init__(self, sample_rate=1):
        if sample_rate <= 0:
            log.error('sample_rate is negative or null, disable the Sampler')
            sample_rate = 1
        elif sample_rate > 1:
            sample_rate = 1

        self.set_sample_rate(sample_rate)

        log.debug('initialized RateSampler, sample %s%% of traces', 100 * sample_rate)

    def set_sample_rate(self, sample_rate):
        self.sample_rate = float(sample_rate)
        self.sampling_id_threshold = self.sample_rate * MAX_TRACE_ID

    def sample(self, span):
        return ((span.trace_id * KNUTH_FACTOR) % MAX_TRACE_ID) <= self.sampling_id_threshold


class RateByServiceSampler(BaseSampler, BasePrioritySampler):
    """Sampler based on a rate, by service

    Keep (100 * `sample_rate`)% of the traces.
    The sample rate is kept independently for each service/env tuple.
    """

    @staticmethod
    def _key(service=None, env=None):
        """Compute a key with the same format used by the Datadog agent API."""
        service = service or ''
        env = env or ''
        return 'service:' + service + ',env:' + env

    def __init__(self, sample_rate=1):
        self.sample_rate = sample_rate
        self._by_service_samplers = self._get_new_by_service_sampler()

    def _get_new_by_service_sampler(self):
        return {
            self._default_key: RateSampler(self.sample_rate)
        }

    def set_sample_rate(self, sample_rate, service='', env=''):
        self._by_service_samplers[self._key(service, env)] = RateSampler(sample_rate)

    def sample(self, span):
        tags = span.tracer.tags
        env = tags[ENV_KEY] if ENV_KEY in tags else None
        key = self._key(span.service, env)

        sampler = self._by_service_samplers.get(
            key, self._by_service_samplers[self._default_key]
        )
        span.set_metric(SAMPLING_AGENT_DECISION, sampler.sample_rate)
        return sampler.sample(span)

    def update_rate_by_service_sample_rates(self, rate_by_service):
        new_by_service_samplers = self._get_new_by_service_sampler()
        for key, sample_rate in iteritems(rate_by_service):
            new_by_service_samplers[key] = RateSampler(sample_rate)

        self._by_service_samplers = new_by_service_samplers


# Default key for service with no specific rate
RateByServiceSampler._default_key = RateByServiceSampler._key()


class DatadogSampler(BaseSampler, BasePrioritySampler):
    """
    This sampler is currently in ALPHA and it's API may change at any time, use at your own risk.
    """
    __slots__ = ('default_sampler', 'limiter', 'rules')

    NO_RATE_LIMIT = -1
    DEFAULT_RATE_LIMIT = 100
    DEFAULT_SAMPLE_RATE = None

    def __init__(self, rules=None, default_sample_rate=None, rate_limit=None):
        """
        Constructor for DatadogSampler sampler

        :param rules: List of :class:`SamplingRule` rules to apply to the root span of every trace, default no rules
        :type rules: :obj:`list` of :class:`SamplingRule`
        :param default_sample_rate: The default sample rate to apply if no rules matched (default: ``None`` /
            Use :class:`RateByServiceSampler` only)
        :type default_sample_rate: float 0 <= X <= 1.0
        :param rate_limit: Global rate limit (traces per second) to apply to all traces regardless of the rules
            applied to them, (default: ``100``)
        :type rate_limit: :obj:`int`
        """
        if default_sample_rate is None:
            # If no sample rate was provided explicitly in code, try to load from environment variable
            sample_rate = get_env('trace', 'sample_rate', default=self.DEFAULT_SAMPLE_RATE)

            # If no env variable was found, just use the default
            if sample_rate is None:
                default_sample_rate = self.DEFAULT_SAMPLE_RATE

            # Otherwise, try to convert it to a float
            else:
                default_sample_rate = float(sample_rate)

        if rate_limit is None:
            rate_limit = int(get_env('trace', 'rate_limit', default=self.DEFAULT_RATE_LIMIT))

        # Ensure rules is a list
        if not rules:
            rules = []

        # Validate that the rules is a list of SampleRules
        for rule in rules:
            if not isinstance(rule, SamplingRule):
                raise TypeError('Rule {!r} must be a sub-class of type ddtrace.sampler.SamplingRules'.format(rule))
        self.rules = rules

        # Configure rate limiter
        self.limiter = RateLimiter(rate_limit)

        # Default to previous default behavior of RateByServiceSampler
        self.default_sampler = RateByServiceSampler()
        if default_sample_rate is not None:
            self.default_sampler = SamplingRule(sample_rate=default_sample_rate)

    def update_rate_by_service_sample_rates(self, sample_rates):
        # Pass through the call to our RateByServiceSampler
        if isinstance(self.default_sampler, RateByServiceSampler):
            self.default_sampler.update_rate_by_service_sample_rates(sample_rates)

    def _set_priority(self, span, priority):
        if span._context:
            span._context.sampling_priority = priority
        span.sampled = priority is AUTO_KEEP

    def sample(self, span):
        """
        Decide whether the provided span should be sampled or not

        The span provided should be the root span in the trace.

        :param span: The root span of a trace
        :type span: :class:`ddtrace.span.Span`
        :returns: Whether the span was sampled or not
        :rtype: :obj:`bool`
        """
        # If there are rules defined, then iterate through them and find one that wants to sample
        matching_rule = None
        # Go through all rules and grab the first one that matched
        # DEV: This means rules should be ordered by the user from most specific to least specific
        for rule in self.rules:
            if rule.matches(span):
                matching_rule = rule
                break
        else:
            # If this is the old sampler, sample and return
            if isinstance(self.default_sampler, RateByServiceSampler):
                if self.default_sampler.sample(span):
                    self._set_priority(span, AUTO_KEEP)
                    return True
                else:
                    self._set_priority(span, AUTO_REJECT)
                    return False

            # If no rules match, use our defualt sampler
            matching_rule = self.default_sampler

        # Sample with the matching sampling rule
        span.set_metric(SAMPLING_RULE_DECISION, matching_rule.sample_rate)
        if not matching_rule.sample(span):
            self._set_priority(span, AUTO_REJECT)
            return False
        else:
            # Do not return here, we need to apply rate limit
            self._set_priority(span, AUTO_KEEP)

        # Ensure all allowed traces adhere to the global rate limit
        allowed = self.limiter.is_allowed()
        # Always set the sample rate metric whether it was allowed or not
        # DEV: Setting this allows us to properly compute metrics and debug the
        #      various sample rates that are getting applied to this span
        span.set_metric(SAMPLING_LIMIT_DECISION, self.limiter.effective_rate)
        if not allowed:
            self._set_priority(span, AUTO_REJECT)
            return False

        # We made it by all of checks, sample this trace
        self._set_priority(span, AUTO_KEEP)
        return True


class SamplingRule(BaseSampler):
    """
    Definition of a sampling rule used by :class:`DatadogSampler` for applying a sample rate on a span
    """
    __slots__ = ('_sample_rate', '_sampling_id_threshold', 'service', 'name')

    NO_RULE = object()

    def __init__(self, sample_rate, service=NO_RULE, name=NO_RULE):
        """
        Configure a new :class:`SamplingRule`

        .. code:: python

            DatadogSampler([
                # Sample 100% of any trace
                SamplingRule(sample_rate=1.0),

                # Sample no healthcheck traces
                SamplingRule(sample_rate=0, name='flask.request'),

                # Sample all services ending in `-db` based on a regular expression
                SamplingRule(sample_rate=0.5, service=re.compile('-db$')),

                # Sample based on service name using custom function
                SamplingRule(sample_rate=0.75, service=lambda service: 'my-app' in service),
            ])

        :param sample_rate: The sample rate to apply to any matching spans
        :type sample_rate: :obj:`float` greater than or equal to 0.0 and less than or equal to 1.0
        :param service: Rule to match the `span.service` on, default no rule defined
        :type service: :obj:`object` to directly compare, :obj:`function` to evaluate, or :class:`re.Pattern` to match
        :param name: Rule to match the `span.name` on, default no rule defined
        :type name: :obj:`object` to directly compare, :obj:`function` to evaluate, or :class:`re.Pattern` to match
        """
        # Enforce sample rate constraints
        if not 0.0 <= sample_rate <= 1.0:
            raise ValueError(
                'SamplingRule(sample_rate={!r}) must be greater than or equal to 0.0 and less than or equal to 1.0',
            )

        self.sample_rate = sample_rate
        self.service = service
        self.name = name

    @property
    def sample_rate(self):
        return self._sample_rate

    @sample_rate.setter
    def sample_rate(self, sample_rate):
        self._sample_rate = sample_rate
        self._sampling_id_threshold = sample_rate * MAX_TRACE_ID

    def _pattern_matches(self, prop, pattern):
        # If the rule is not set, then assume it matches
        # DEV: Having no rule and being `None` are different things
        #   e.g. ignoring `span.service` vs `span.service == None`
        if pattern is self.NO_RULE:
            return True

        # If the pattern is callable (e.g. a function) then call it passing the prop
        #   The expected return value is a boolean so cast the response in case it isn't
        if callable(pattern):
            try:
                return bool(pattern(prop))
            except Exception:
                log.warning('%r pattern %r failed with %r', self, pattern, prop, exc_info=True)
                # Their function failed to validate, assume it is a False
                return False

        # The pattern is a regular expression and the prop is a string
        if isinstance(pattern, pattern_type):
            try:
                return bool(pattern.match(str(prop)))
            except (ValueError, TypeError):
                # This is to guard us against the casting to a string (shouldn't happen, but still)
                log.warning('%r pattern %r failed with %r', self, pattern, prop, exc_info=True)
                return False

        # Exact match on the values
        return prop == pattern

    def matches(self, span):
        """
        Return if this span matches this rule

        :param span: The span to match against
        :type span: :class:`ddtrace.span.Span`
        :returns: Whether this span matches or not
        :rtype: :obj:`bool`
        """
        return all(
            self._pattern_matches(prop, pattern)
            for prop, pattern in [
                (span.service, self.service),
                (span.name, self.name),
            ]
        )

    def sample(self, span):
        """
        Return if this rule chooses to sample the span

        :param span: The span to sample against
        :type span: :class:`ddtrace.span.Span`
        :returns: Whether this span was sampled
        :rtype: :obj:`bool`
        """
        if self.sample_rate == 1:
            return True
        elif self.sample_rate == 0:
            return False

        return ((span.trace_id * KNUTH_FACTOR) % MAX_TRACE_ID) <= self._sampling_id_threshold

    def _no_rule_or_self(self, val):
        return 'NO_RULE' if val is self.NO_RULE else val

    def __repr__(self):
        return '{}(sample_rate={!r}, service={!r}, name={!r})'.format(
            self.__class__.__name__,
            self.sample_rate,
            self._no_rule_or_self(self.service),
            self._no_rule_or_self(self.name),
        )

    __str__ = __repr__
