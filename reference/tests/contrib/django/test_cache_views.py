# testing
from .compat import reverse
from .utils import DjangoTraceTestCase


class DjangoCacheViewTest(DjangoTraceTestCase):
    """
    Ensures that the cache system is properly traced
    """
    def test_cached_view(self):
        # make the first request so that the view is cached
        url = reverse('cached-users-list')
        response = self.client.get(url)
        assert response.status_code == 200

        # check the first call for a non-cached view
        spans = self.tracer.writer.pop()
        assert len(spans) == 6
        # the cache miss
        assert spans[1].resource == 'get'
        # store the result in the cache
        assert spans[4].resource == 'set'
        assert spans[5].resource == 'set'

        # check if the cache hit is traced
        response = self.client.get(url)
        spans = self.tracer.writer.pop()
        assert len(spans) == 3

        span_header = spans[1]
        span_view = spans[2]
        assert span_view.service == 'django'
        assert span_view.resource == 'get'
        assert span_view.name == 'django.cache'
        assert span_view.span_type == 'cache'
        assert span_view.error == 0
        assert span_header.service == 'django'
        assert span_header.resource == 'get'
        assert span_header.name == 'django.cache'
        assert span_header.span_type == 'cache'
        assert span_header.error == 0

        expected_meta_view = {
            'django.cache.backend': 'django.core.cache.backends.locmem.LocMemCache',
            'django.cache.key': (
                'views.decorators.cache.cache_page..'
                'GET.03cdc1cc4aab71b038a6764e5fcabb82.d41d8cd98f00b204e9800998ecf8427e.en-us'
            ),
            'env': 'test',
        }

        expected_meta_header = {
            'django.cache.backend': 'django.core.cache.backends.locmem.LocMemCache',
            'django.cache.key': 'views.decorators.cache.cache_header..03cdc1cc4aab71b038a6764e5fcabb82.en-us',
            'env': 'test',
        }

        assert span_view.meta == expected_meta_view
        assert span_header.meta == expected_meta_header

    def test_cached_template(self):
        # make the first request so that the view is cached
        url = reverse('cached-template-list')
        response = self.client.get(url)
        assert response.status_code == 200

        # check the first call for a non-cached view
        spans = self.tracer.writer.pop()
        assert len(spans) == 5
        # the cache miss
        assert spans[2].resource == 'get'
        # store the result in the cache
        assert spans[4].resource == 'set'

        # check if the cache hit is traced
        response = self.client.get(url)
        spans = self.tracer.writer.pop()
        assert len(spans) == 3

        span_template_cache = spans[2]
        assert span_template_cache.service == 'django'
        assert span_template_cache.resource == 'get'
        assert span_template_cache.name == 'django.cache'
        assert span_template_cache.span_type == 'cache'
        assert span_template_cache.error == 0

        expected_meta = {
            'django.cache.backend': 'django.core.cache.backends.locmem.LocMemCache',
            'django.cache.key': 'template.cache.users_list.d41d8cd98f00b204e9800998ecf8427e',
            'env': 'test',
        }

        assert span_template_cache.meta == expected_meta
