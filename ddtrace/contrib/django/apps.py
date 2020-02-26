# 3rd party
from django.apps import AppConfig, apps

# project
from .patch import apply_django_patches


class TracerConfig(AppConfig):
    name = 'ddtrace.contrib.django'
    label = 'datadog_django'

    def ready(self):
        """
        Ready is called as soon as the registry is fully populated.
        Tracing capabilities must be enabled in this function so that
        all Django internals are properly configured.
        """
        rest_framework_is_installed = apps.is_installed('rest_framework')
        apply_django_patches(patch_rest_framework=rest_framework_is_installed)
