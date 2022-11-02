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


def response_with_custom_header(request):
    response = HttpResponse()
    response["custom-test-header-1"] = "test-header-value-1"
    response["custom-test-header-2"] = "test-header-value-2"
    response[
        "my-custom-regex-header-1"
    ] = "my-custom-regex-value-1,my-custom-regex-value-2"
    response[
        "my-custom-regex-header-2"
    ] = "my-custom-regex-value-3,my-custom-regex-value-4"
    response["my-secret-header"] = "my-secret-value"
    return response


async def async_traced(request):  # pylint: disable=unused-argument
    return HttpResponse()


async def async_traced_template(
    request, year
):  # pylint: disable=unused-argument
    return HttpResponse()


async def async_error(request):  # pylint: disable=unused-argument
    raise ValueError("error")


async def async_excluded(request):  # pylint: disable=unused-argument
    return HttpResponse()


async def async_excluded_noarg(request):  # pylint: disable=unused-argument
    return HttpResponse()


async def async_excluded_noarg2(request):  # pylint: disable=unused-argument
    return HttpResponse()


async def async_route_span_name(
    request, *args, **kwargs
):  # pylint: disable=unused-argument
    return HttpResponse()


async def async_with_custom_header(request):
    response = HttpResponse()
    response.headers["custom-test-header-1"] = "test-header-value-1"
    response.headers["custom-test-header-2"] = "test-header-value-2"
    return response
