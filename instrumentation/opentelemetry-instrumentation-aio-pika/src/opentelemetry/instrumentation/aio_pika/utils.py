from opentelemetry import context


def is_instrumentation_enabled() -> bool:
    if context.get_value("suppress_instrumentation") or context.get_value(
        context._SUPPRESS_INSTRUMENTATION_KEY
    ):
        return False
    return True
