# Plugins

## Defining Plugins

A plugin is a templated set of operators. Plugins are defined by using a file with a name that corresponds to the plugin's `type`.

For example, a plugin for monitoring Apache Tomcat access logs could look like this:
`tomcat.yaml`:
```yaml
pipeline:
  - type: file_input
    include:
      - {{ .path }}

  - type: regex_parser
    regex: '(?P<remote_host>[^\s]+) - (?P<remote_user>[^\s]+) \[(?P<timestamp>[^\]]+)\] "(?P<http_method>[A-Z]+) (?P<path>[^\s]+)[^"]+" (?P<http_status>\d+) (?P<bytes_sent>[^\s]+)'
    output: {{ .output }}
```

## Using Plugins

For stanza to discover a plugin, it needs to be in the `plugins` directory. This can be customized with the `--plugin_dir` argument. For a default installation, the plugin directory is located at `$STANZA_HOME/plugins`.

Plugins can be used in the stanza config file with a `type` matching the filename of the plugin.

In the following example, the `tomcat` plugin used. 

`config.yaml`:
```yaml
pipeline:
  - type: tomcat
    path: /var/log/tomcat/access.log

  - type: stdout
```

When stanza builds the above pipeline the following is happening:
1. stanza looks for an operator or plugin of type `tomcat`. There is no builtin `tomcat` operator, but `tomcat.yaml` was found in the specified plugin directory, so stanza will render this plugin.
2. Parameters are passed into the `tomcat` plugin. In this case, there are actually two parameters - `path` and `output`. `path` is explicitly declared, but the `output` parameter was omitted, so it has taken a default value of `stdout`. If you're not sure why this is, it's highly recommended that you take a moment to read about output parameters [here](/docs/pipeline.md).
3. stanza substitutes the parameters into the plugin template. In the above example, `{{ .path }}` is replaced with `/var/log/tomcat/access.log`, and `{{ .output }}` is replaced with `stdout`.
4. The "rendered" plugin replaces the `tomcat` block. It doesn't overwrite the file - this is just what is logically happening behind the scenes.
5. The resulting pipeline is now the same as the following configuration:
```yaml
pipeline:
  - type: file_input
    include:
      - /var/log/tomcat/access.log

  - type: regex_parser
    regex: '(?P<remote_host>[^\s]+) - (?P<remote_user>[^\s]+) \[(?P<timestamp>[^\]]+)\] "(?P<http_method>[A-Z]+) (?P<path>[^\s]+)[^"]+" (?P<http_status>\d+) (?P<bytes_sent>[^\s]+)'
    output: stdout
  
  - type: stdout
```


## Writing a plugin

Writing a plugin is as easy as pulling out a set of operators in a working configuration file, then templatizing it with any parts of the config that need to be treated as variable. In the example of the Tomcat access log plugin above, that just means adding variables for `path` and `output`.

Plugins use Go's [`text/template`](https://golang.org/pkg/text/template/) package for template rendering. All fields from the plugin configuration are available as variables in the templates except the `type` field.
