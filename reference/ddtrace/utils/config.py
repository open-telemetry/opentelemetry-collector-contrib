import sys
import os


def get_application_name():
    """Attempts to find the application name using system arguments."""
    if hasattr(sys, "argv") and sys.argv[0]:
        app_name = os.path.basename(sys.argv[0])
    else:
        app_name = None
    return app_name
