def parse_method_path(method_path):
    """ Returns (package, service, method) tuple from parsing method path """
    # unpack method path based on "/{package}.{service}/{method}"
    # first remove leading "/" as unnecessary
    package_service, method_name = method_path.lstrip('/').rsplit('/', 1)

    # {package} is optional
    package_service = package_service.rsplit('.', 1)
    if len(package_service) == 2:
        return package_service[0], package_service[1], method_name

    return None, package_service[0], method_name
