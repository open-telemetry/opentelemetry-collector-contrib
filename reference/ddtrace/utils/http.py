def normalize_header_name(header_name):
    """
    Normalizes an header name to lower case, stripping all its leading and trailing white spaces.
    :param header_name: the header name to normalize
    :type header_name: str
    :return: the normalized header name
    :rtype: str
    """
    return header_name.strip().lower() if header_name is not None else None
