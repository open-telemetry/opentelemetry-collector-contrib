import os
import django
from django.conf import settings

# We manually designate which settings we will be using in an environment variable
# This is similar to what occurs in the `manage.py`
os.environ.setdefault('DJANGO_SETTINGS_MODULE', 'tests.contrib.django.app.settings')


# `pytest` automatically calls this function once when tests are run.
def pytest_configure():
    settings.DEBUG = False
    if django.VERSION < (1, 7, 0):
        settings.configure()
    else:
        django.setup()
