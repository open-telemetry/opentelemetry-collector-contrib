import logging
import mock

from ddtrace.internal.logger import DDLogger, get_logger

from ..base import BaseTestCase

ALL_LEVEL_NAMES = ('debug', 'info', 'warn', 'warning', 'error', 'exception', 'critical', 'fatal')


class DDLoggerTestCase(BaseTestCase):
    def setUp(self):
        super(DDLoggerTestCase, self).setUp()

        self.root = logging.root
        self.manager = self.root.manager

    def tearDown(self):
        # Weeee, forget all existing loggers
        logging.Logger.manager.loggerDict.clear()
        self.assertEqual(logging.Logger.manager.loggerDict, dict())

        self.root = None
        self.manager = None

        super(DDLoggerTestCase, self).tearDown()

    def _make_record(
            self, logger, msg='test', args=(), level=logging.INFO,
            fn='module.py', lno=5, exc_info=(None, None, None), func=None, extra=None
    ):
        return logger.makeRecord(logger.name, level, fn, lno, msg, args, exc_info, func, extra)

    @mock.patch('ddtrace.internal.logger.DDLogger.handle')
    def assert_log_records(self, log, expected_levels, handle):
        for name in ALL_LEVEL_NAMES:
            method = getattr(log, name)
            method('test')

        records = [args[0][0] for args in handle.call_args_list]
        for record in records:
            self.assertIsInstance(record, logging.LogRecord)
            self.assertEqual(record.name, 'test.logger')
            self.assertEqual(record.msg, 'test')

        levels = [r.levelname for r in records]
        self.assertEqual(levels, expected_levels)

    def test_get_logger(self):
        """
        When using `get_logger` to get a logger
            When the logger does not exist
                We create a new DDLogger
            When the logger exists
                We return the expected logger
            When a different logger is requested
                We return a new DDLogger
        """
        # Assert the logger doesn't already exist
        self.assertNotIn('test.logger', self.manager.loggerDict)

        # Fetch a new logger
        log = get_logger('test.logger')
        self.assertEqual(log.name, 'test.logger')
        self.assertEqual(log.level, logging.NOTSET)

        # Ensure it is a DDLogger
        self.assertIsInstance(log, DDLogger)
        # Make sure it is stored in all the places we expect
        self.assertEqual(self.manager.getLogger('test.logger'), log)
        self.assertEqual(self.manager.loggerDict['test.logger'], log)

        # Fetch the same logger
        same_log = get_logger('test.logger')
        # Assert we got the same logger
        self.assertEqual(log, same_log)

        # Fetch a different logger
        new_log = get_logger('new.test.logger')
        # Make sure we didn't get the same one
        self.assertNotEqual(log, new_log)

    def test_get_logger_parents(self):
        """
        When using `get_logger` to get a logger
            We appropriately assign parent loggers

        DEV: This test case is to ensure we are calling `manager._fixupParents(logger)`
        """
        # Fetch a new logger
        test_log = get_logger('test')
        self.assertEqual(test_log.parent, self.root)

        # Fetch a new child log
        # Auto-associate with parent `test` logger
        child_log = get_logger('test.child')
        self.assertEqual(child_log.parent, test_log)

        # Deep child
        deep_log = get_logger('test.child.logger.from.test.case')
        self.assertEqual(deep_log.parent, child_log)

    def test_logger_init(self):
        """
        When creating a new DDLogger
            Has the same interface as logging.Logger
            Configures a defaultdict for buckets
            Properly configures the rate limit
        """
        # Create a logger
        log = DDLogger('test.logger')

        # Ensure we set the name and use default log level
        self.assertEqual(log.name, 'test.logger')
        self.assertEqual(log.level, logging.NOTSET)

        # Assert DDLogger default properties
        self.assertIsInstance(log.buckets, dict)
        self.assertEqual(log.rate_limit, 60)

        # Assert manager and parent
        # DEV: Parent is `None` because `manager._findParents()` doesn't get called
        #      unless we use `get_logger` (this is the same behavior as `logging.getLogger` and `Logger('name')`)
        self.assertEqual(log.manager, self.manager)
        self.assertIsNone(log.parent)

        # Override rate limit from environment variable
        with self.override_env(dict(DD_LOGGING_RATE_LIMIT='10')):
            log = DDLogger('test.logger')
            self.assertEqual(log.rate_limit, 10)

        # Set specific log level
        log = DDLogger('test.logger', level=logging.DEBUG)
        self.assertEqual(log.level, logging.DEBUG)

    def test_logger_log(self):
        """
        When calling `DDLogger` log methods
            We call `DDLogger.handle` with the expected log record
        """
        log = get_logger('test.logger')

        # -- NOTSET
        # By default no level is set so we only get warn, error, and critical messages
        self.assertEqual(log.level, logging.NOTSET)
        # `log.warn`, `log.warning`, `log.error`, `log.exception`, `log.critical`, `log.fatal`
        self.assert_log_records(log, ['WARNING', 'WARNING', 'ERROR', 'ERROR', 'CRITICAL', 'CRITICAL'])

        # -- CRITICAL
        log.setLevel(logging.CRITICAL)
        # `log.critical`, `log.fatal`
        self.assert_log_records(log, ['CRITICAL', 'CRITICAL'])

        # -- ERROR
        log.setLevel(logging.ERROR)
        # `log.error`, `log.exception`, `log.critical`, `log.fatal`
        self.assert_log_records(log, ['ERROR', 'ERROR', 'CRITICAL', 'CRITICAL'])

        # -- WARN
        log.setLevel(logging.WARN)
        # `log.warn`, `log.warning`, `log.error`, `log.exception`, `log.critical`, `log.fatal`
        self.assert_log_records(log, ['WARNING', 'WARNING', 'ERROR', 'ERROR', 'CRITICAL', 'CRITICAL'])

        # -- INFO
        log.setLevel(logging.INFO)
        # `log.info`, `log.warn`, `log.warning`, `log.error`, `log.exception`, `log.critical`, `log.fatal`
        self.assert_log_records(log, ['INFO', 'WARNING', 'WARNING', 'ERROR', 'ERROR', 'CRITICAL', 'CRITICAL'])

        # -- DEBUG
        log.setLevel(logging.DEBUG)
        # `log.debug`, `log.info`, `log.warn`, `log.warning`, `log.error`, `log.exception`, `log.critical`, `log.fatal`
        self.assert_log_records(log, ['DEBUG', 'INFO', 'WARNING', 'WARNING', 'ERROR', 'ERROR', 'CRITICAL', 'CRITICAL'])

    @mock.patch('logging.Logger.handle')
    def test_logger_handle_no_limit(self, base_handle):
        """
        Calling `DDLogger.handle`
            When no rate limit is set
                Always calls the base `Logger.handle`
        """
        # Configure an INFO logger with no rate limit
        log = get_logger('test.logger')
        log.setLevel(logging.INFO)
        log.rate_limit = 0

        # Log a bunch of times very quickly (this is fast)
        for _ in range(1000):
            log.info('test')

        # Assert that we did not perform any rate limiting
        self.assertEqual(base_handle.call_count, 1000)

        # Our buckets are empty
        self.assertEqual(log.buckets, dict())

    @mock.patch('logging.Logger.handle')
    def test_logger_handle_bucket(self, base_handle):
        """
        When calling `DDLogger.handle`
            With a record
                We pass it to the base `Logger.handle`
                We create a bucket for tracking
        """
        log = get_logger('test.logger')

        # Create log record and handle it
        record = self._make_record(log)
        log.handle(record)

        # We passed to base Logger.handle
        base_handle.assert_called_once_with(record)

        # We added an bucket entry for this record
        key = (record.name, record.levelno, record.pathname, record.lineno)
        logging_bucket = log.buckets.get(key)
        self.assertIsInstance(logging_bucket, DDLogger.LoggingBucket)

        # The bucket entry is correct
        expected_bucket = int(record.created / log.rate_limit)
        self.assertEqual(logging_bucket.bucket, expected_bucket)
        self.assertEqual(logging_bucket.skipped, 0)

    @mock.patch('logging.Logger.handle')
    def test_logger_handle_bucket_limited(self, base_handle):
        """
        When calling `DDLogger.handle`
            With multiple records in a single time frame
                We pass only the first to the base `Logger.handle`
                We keep track of the number skipped
        """
        log = get_logger('test.logger')

        # Create log record and handle it
        first_record = self._make_record(log, msg='first')
        log.handle(first_record)

        for _ in range(100):
            record = self._make_record(log)
            # DEV: Use the same timestamp as `first_record` to ensure we are in the same bucket
            record.created = first_record.created
            log.handle(record)

        # We passed to base Logger.handle
        base_handle.assert_called_once_with(first_record)

        # We added an bucket entry for these records
        key = (record.name, record.levelno, record.pathname, record.lineno)
        logging_bucket = log.buckets.get(key)

        # The bucket entry is correct
        expected_bucket = int(first_record.created / log.rate_limit)
        self.assertEqual(logging_bucket.bucket, expected_bucket)
        self.assertEqual(logging_bucket.skipped, 100)

    @mock.patch('logging.Logger.handle')
    def test_logger_handle_bucket_skipped_msg(self, base_handle):
        """
        When calling `DDLogger.handle`
            When a bucket exists for a previous time frame
                We pass only the record to the base `Logger.handle`
                We update the record message to include the number of skipped messages
        """
        log = get_logger('test.logger')

        # Create log record to handle
        original_msg = 'hello %s'
        original_args = (1, )
        record = self._make_record(log, msg=original_msg, args=(1, ))

        # Create a bucket entry for this record
        key = (record.name, record.levelno, record.pathname, record.lineno)
        bucket = int(record.created / log.rate_limit)
        # We want the time bucket to be for an older bucket
        log.buckets[key] = DDLogger.LoggingBucket(bucket=bucket - 1, skipped=20)

        # Handle our record
        log.handle(record)

        # We passed to base Logger.handle
        base_handle.assert_called_once_with(record)

        self.assertEqual(record.msg, original_msg + ', %s additional messages skipped')
        self.assertEqual(record.args, original_args + (20, ))
        self.assertEqual(record.getMessage(), 'hello 1, 20 additional messages skipped')

    def test_logger_handle_bucket_key(self):
        """
        When calling `DDLogger.handle`
            With different log messages
                We use different buckets to limit them
        """
        log = get_logger('test.logger')

        # DEV: This function is inlined in `logger.py`
        def get_key(record):
            return (record.name, record.levelno, record.pathname, record.lineno)

        # Same record signature but different message
        # DEV: These count against the same bucket
        record1 = self._make_record(log, msg='record 1')
        record2 = self._make_record(log, msg='record 2')

        # Different line number (default is `10`)
        record3 = self._make_record(log, lno=10)

        # Different pathnames (default is `module.py`)
        record4 = self._make_record(log, fn='log.py')

        # Different level (default is `logging.INFO`)
        record5 = self._make_record(log, level=logging.WARN)

        # Different logger name
        record6 = self._make_record(log)
        record6.name = 'test.logger2'

        # Log all of our records
        all_records = (record1, record2, record3, record4, record5, record6)
        [log.handle(record) for record in all_records]

        buckets = log.buckets
        # We have 6 records but only end up with 5 buckets
        self.assertEqual(len(buckets), 5)

        # Assert bucket created for the record1 and record2
        bucket1 = buckets[get_key(record1)]
        self.assertEqual(bucket1.skipped, 1)

        bucket2 = buckets[get_key(record2)]
        self.assertEqual(bucket1, bucket2)

        # Assert bucket for the remaining records
        # None of these other messages should have been grouped together
        for record in (record3, record4, record5, record6):
            bucket = buckets[get_key(record)]
            self.assertEqual(bucket.skipped, 0)
