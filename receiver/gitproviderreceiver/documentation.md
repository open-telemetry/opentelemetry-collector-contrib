[comment]: <> (Code generated by mdatagen. DO NOT EDIT.)

# gitprovider

## Default Metrics

The following metrics are emitted by default. Each of them can be disabled by applying the following configuration:

```yaml
metrics:
  <metric_name>:
    enabled: false
```

### vcs.repository.change.count

The number of changes (pull requests) in a repository, categorized by their state (either open or merged).

| Unit | Metric Type | Value Type |
| ---- | ----------- | ---------- |
| {change} | Gauge | Int |

#### Attributes

| Name | Description | Values |
| ---- | ----------- | ------ |
| change.state | The state of a change (pull request) | Str: ``open``, ``merged`` |
| repository.name | The name of a VCS repository | Any Str |

### vcs.repository.change.time_open

The amount of time a change (pull request) has been open.

| Unit | Metric Type | Value Type |
| ---- | ----------- | ---------- |
| s | Gauge | Int |

#### Attributes

| Name | Description | Values |
| ---- | ----------- | ------ |
| repository.name | The name of a VCS repository | Any Str |
| ref.name | The name of a VCS branch | Any Str |

### vcs.repository.change.time_to_approval

The amount of time it took a change (pull request) to go from open to approved.

| Unit | Metric Type | Value Type |
| ---- | ----------- | ---------- |
| s | Gauge | Int |

#### Attributes

| Name | Description | Values |
| ---- | ----------- | ------ |
| repository.name | The name of a VCS repository | Any Str |
| ref.name | The name of a VCS branch | Any Str |

### vcs.repository.change.time_to_merge

The amount of time it took a change (pull request) to go from open to merged.

| Unit | Metric Type | Value Type |
| ---- | ----------- | ---------- |
| s | Gauge | Int |

#### Attributes

| Name | Description | Values |
| ---- | ----------- | ------ |
| repository.name | The name of a VCS repository | Any Str |
| ref.name | The name of a VCS branch | Any Str |

### vcs.repository.count

The number of repositories in an organization.

| Unit | Metric Type | Value Type |
| ---- | ----------- | ---------- |
| {repository} | Gauge | Int |

### vcs.repository.ref.count

The number of refs of type branch in a repository.

| Unit | Metric Type | Value Type |
| ---- | ----------- | ---------- |
| {ref} | Gauge | Int |

#### Attributes

| Name | Description | Values |
| ---- | ----------- | ------ |
| repository.name | The name of a VCS repository | Any Str |
| ref.type | The type of ref (branch, tag). | Str: ``branch``, ``tag`` |

### vcs.repository.ref.lines_added

The number of lines added in a ref (branch) relative to the default branch (trunk).

| Unit | Metric Type | Value Type |
| ---- | ----------- | ---------- |
| {line} | Gauge | Int |

#### Attributes

| Name | Description | Values |
| ---- | ----------- | ------ |
| repository.name | The name of a VCS repository | Any Str |
| ref.name | The name of a VCS branch | Any Str |
| ref.type | The type of ref (branch, tag). | Str: ``branch``, ``tag`` |

### vcs.repository.ref.lines_deleted

The number of lines deleted in a ref (branch) relative to the default branch (trunk).

| Unit | Metric Type | Value Type |
| ---- | ----------- | ---------- |
| {line} | Gauge | Int |

#### Attributes

| Name | Description | Values |
| ---- | ----------- | ------ |
| repository.name | The name of a VCS repository | Any Str |
| ref.name | The name of a VCS branch | Any Str |
| ref.type | The type of ref (branch, tag). | Str: ``branch``, ``tag`` |

### vcs.repository.ref.revisions_ahead

The number of revisions (commits) a ref (branch) is ahead of the default branch (trunk).

| Unit | Metric Type | Value Type |
| ---- | ----------- | ---------- |
| {revision} | Gauge | Int |

#### Attributes

| Name | Description | Values |
| ---- | ----------- | ------ |
| repository.name | The name of a VCS repository | Any Str |
| ref.name | The name of a VCS branch | Any Str |
| ref.type | The type of ref (branch, tag). | Str: ``branch``, ``tag`` |

### vcs.repository.ref.revisions_behind

The number of revisions (commits) a ref (branch) is behind the default branch (trunk).

| Unit | Metric Type | Value Type |
| ---- | ----------- | ---------- |
| {revision} | Gauge | Int |

#### Attributes

| Name | Description | Values |
| ---- | ----------- | ------ |
| repository.name | The name of a VCS repository | Any Str |
| ref.name | The name of a VCS branch | Any Str |
| ref.type | The type of ref (branch, tag). | Str: ``branch``, ``tag`` |

### vcs.repository.ref.time

Time a ref (branch) created from the default branch (trunk) has existed. The `ref.type` attribute will always be `branch`.

| Unit | Metric Type | Value Type |
| ---- | ----------- | ---------- |
| s | Gauge | Int |

#### Attributes

| Name | Description | Values |
| ---- | ----------- | ------ |
| repository.name | The name of a VCS repository | Any Str |
| ref.name | The name of a VCS branch | Any Str |
| ref.type | The type of ref (branch, tag). | Str: ``branch``, ``tag`` |

## Optional Metrics

The following metrics are not emitted by default. Each of them can be enabled by applying the following configuration:

```yaml
metrics:
  <metric_name>:
    enabled: true
```

### vcs.repository.contributor.count

The number of unique contributors to a repository.

| Unit | Metric Type | Value Type |
| ---- | ----------- | ---------- |
| {contributor} | Gauge | Int |

#### Attributes

| Name | Description | Values |
| ---- | ----------- | ------ |
| repository.name | The name of a VCS repository | Any Str |

## Resource Attributes

| Name | Description | Values | Enabled |
| ---- | ----------- | ------ | ------- |
| organization.name | VCS Organization | Any Str | true |
| vcs.vendor.name | The name of the VCS vendor/provider (ie. GitHub) | Any Str | true |