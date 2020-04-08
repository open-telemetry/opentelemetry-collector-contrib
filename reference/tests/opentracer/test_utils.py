from opentracing.scope_managers import ThreadLocalScopeManager

import ddtrace
from ddtrace.opentracer.utils import (
    get_context_provider_for_scope_manager,
)


class TestOpentracerUtils(object):
    def test_get_context_provider_for_scope_manager_thread(self):
        scope_manager = ThreadLocalScopeManager()
        ctx_prov = get_context_provider_for_scope_manager(scope_manager)
        assert isinstance(ctx_prov, ddtrace.provider.DefaultContextProvider)
