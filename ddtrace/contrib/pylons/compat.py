try:
    from pylons.templating import render_mako  # noqa

    # Pylons > 0.9.7
    legacy_pylons = False
except ImportError:
    # Pylons <= 0.9.7
    legacy_pylons = True
