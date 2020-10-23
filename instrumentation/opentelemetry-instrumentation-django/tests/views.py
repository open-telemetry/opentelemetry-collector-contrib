from django.http import HttpResponse


def traced(request):  # pylint: disable=unused-argument
    return HttpResponse()


def traced_template(request, year):  # pylint: disable=unused-argument
    return HttpResponse()


def error(request):  # pylint: disable=unused-argument
    raise ValueError("error")


def excluded(request):  # pylint: disable=unused-argument
    return HttpResponse()


def excluded_noarg(request):  # pylint: disable=unused-argument
    return HttpResponse()


def excluded_noarg2(request):  # pylint: disable=unused-argument
    return HttpResponse()


def route_span_name(
    request, *args, **kwargs
):  # pylint: disable=unused-argument
    return HttpResponse()
