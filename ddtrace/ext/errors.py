"""
tags for common error attributes
"""

import traceback


ERROR_MSG = 'error.msg'  # a string representing the error message
ERROR_TYPE = 'error.type'  # a string representing the type of the error
ERROR_STACK = 'error.stack'  # a human readable version of the stack. beta.

# shorthand for -----^
MSG = ERROR_MSG
TYPE = ERROR_TYPE
STACK = ERROR_STACK


def get_traceback(tb=None, error=None):
    t = None
    if error:
        t = type(error)
    lines = traceback.format_exception(t, error, tb, limit=20)
    return '\n'.join(lines)
