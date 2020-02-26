"""
Class based views used for Django tests.
"""

from functools import partial

from django.http import HttpResponse
from django.conf.urls import url

from django.views.generic import ListView, TemplateView
from django.views.decorators.cache import cache_page

from django.contrib.auth.models import User
from django.contrib.syndication.views import Feed


class UserList(ListView):
    model = User
    template_name = 'users_list.html'


class TemplateCachedUserList(ListView):
    model = User
    template_name = 'cached_list.html'


class ForbiddenView(TemplateView):
    def get(self, request, *args, **kwargs):
        return HttpResponse(status=403)


def function_view(request):
    return HttpResponse(status=200)


def error_500(request):
    raise Exception('Error 500')


class FeedView(Feed):
    """
    A callable view that is part of the Django framework
    """
    title = 'Police beat site news'
    link = '/sitenews/'
    description = 'Updates on changes and additions to police beat central.'

    def items(self):
        return []

    def item_title(self, item):
        return 'empty'

    def item_description(self, item):
        return 'empty'


partial_view = partial(function_view)

# disabling flake8 test below, yes, declaring a func like this is bad, we know
lambda_view = lambda request: function_view(request)  # NOQA

# use this url patterns for tests
urlpatterns = [
    url(r'^users/$', UserList.as_view(), name='users-list'),
    url(r'^cached-template/$', TemplateCachedUserList.as_view(), name='cached-template-list'),
    url(r'^cached-users/$', cache_page(60)(UserList.as_view()), name='cached-users-list'),
    url(r'^fail-view/$', ForbiddenView.as_view(), name='forbidden-view'),
    url(r'^fn-view/$', function_view, name='fn-view'),
    url(r'^feed-view/$', FeedView(), name='feed-view'),
    url(r'^partial-view/$', partial_view, name='partial-view'),
    url(r'^lambda-view/$', lambda_view, name='lambda-view'),
    url(r'^error-500/$', error_500, name='error-500'),
]
