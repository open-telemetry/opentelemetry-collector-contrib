### Overrides

The configschema library produces metadata about component configurations. It does so by introspecting component config types and by looking at godoc
present in the source code. In some cases, components have one or more config fields whose compile-time types are vague (e.g. `any`) but they require
the field to be of a certain type at runtime. For these components, this directory provides a way to override a component's configschema with a manually
generated version.

The yaml in each file must be un-marshalable to a `configschema.Field{}`, and must have a name of `<component-group>-<component-type>.yaml`
(e.g. `receiver-hostmetrics.yaml`).

Note: as of this writing, overrides are only used by the Otto web application.
