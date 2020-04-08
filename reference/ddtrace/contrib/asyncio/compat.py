import sys

# asyncio.Task.current_task method is deprecated and will be removed in Python
# 3.9. Instead use asyncio.current_task
if sys.version_info >= (3, 7, 0):
    from asyncio import current_task as asyncio_current_task
else:
    import asyncio
    asyncio_current_task = asyncio.Task.current_task
