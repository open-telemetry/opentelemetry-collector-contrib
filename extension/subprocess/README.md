# Subprocess Extension

The `subprocess` extension facilitates running a subprocess of the collector.

You can provide the configuration to the subprocess via the command line arguments. 

You can also specify the working directory of the command.

## Configuration

The executable path is required:

- `executable_path`: The path of the command to run. If `executable_path` is relative, then it is evaluated relative to the working directory.

The following settings can be optionally configured:

- `args`: The command line arguments to use.
- `working_directory`: Defines the working directory of the command. If it is not configured, the `subprocess` extension runs the command in the calling process's current directory.

## Example Config

```yaml
extensions:
  health_check:
  sub_process:
    executable_path: bin/cmd
    args:
      - --port
      - 5989
    working_directory: /usr/local/

receivers:
# {...}
service:

  extensions: [sub_process]
```
