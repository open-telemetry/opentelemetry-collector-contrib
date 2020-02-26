"""
Copyright (c) 2013-2019, Graham Dumpleton
All rights reserved.

Redistribution and use in source and binary forms, with or without
modification, are permitted provided that the following conditions are met:

* Redistributions of source code must retain the above copyright notice, this
  list of conditions and the following disclaimer.

* Redistributions in binary form must reproduce the above copyright notice,
  this list of conditions and the following disclaimer in the documentation
  and/or other materials provided with the distribution.

THIS SOFTWARE IS PROVIDED BY THE COPYRIGHT HOLDERS AND CONTRIBUTORS "AS IS"
AND ANY EXPRESS OR IMPLIED WARRANTIES, INCLUDING, BUT NOT LIMITED TO, THE
IMPLIED WARRANTIES OF MERCHANTABILITY AND FITNESS FOR A PARTICULAR PURPOSE
ARE DISCLAIMED. IN NO EVENT SHALL THE COPYRIGHT HOLDER OR CONTRIBUTORS BE
LIABLE FOR ANY DIRECT, INDIRECT, INCIDENTAL, SPECIAL, EXEMPLARY, OR
CONSEQUENTIAL DAMAGES (INCLUDING, BUT NOT LIMITED TO, PROCUREMENT OF
SUBSTITUTE GOODS OR SERVICES; LOSS OF USE, DATA, OR PROFITS; OR BUSINESS
INTERRUPTION) HOWEVER CAUSED AND ON ANY THEORY OF LIABILITY, WHETHER IN
CONTRACT, STRICT LIABILITY, OR TORT (INCLUDING NEGLIGENCE OR OTHERWISE)
ARISING IN ANY WAY OUT OF THE USE OF THIS SOFTWARE, EVEN IF ADVISED OF THE
POSSIBILITY OF SUCH DAMAGE.
"""
__version_info__ = ('1', '11', '1')
__version__ = '.'.join(__version_info__)

from .wrappers import (ObjectProxy, CallableObjectProxy, FunctionWrapper,
        BoundFunctionWrapper, WeakFunctionProxy, PartialCallableObjectProxy,
        resolve_path, apply_patch, wrap_object, wrap_object_attribute,
        function_wrapper, wrap_function_wrapper, patch_function_wrapper,
        transient_function_wrapper)

from .decorators import (adapter_factory, AdapterFactory, decorator,
        synchronized)

from .importer import (register_post_import_hook, when_imported,
        notify_module_loaded, discover_post_import_hooks)

from inspect import getcallargs
