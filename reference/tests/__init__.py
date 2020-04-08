# Do *NOT* `import ddtrace` in here
# DEV: Some tests rely on import order of modules
#   in order to properly function. Importing `ddtrace`
#   here would mess with those tests since everyone
#   will load this file by default
