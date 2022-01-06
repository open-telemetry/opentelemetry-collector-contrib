---
name: New component proposal
about: Suggest a new component for the project
title: ''
labels: new component
assignees: ''

---

**The purpose and use-cases of the new component**

*This information can be used later on to populate the README for the component.*

**Example configuration for the component**

*This will be used later on when creating `config.go` and added to README as well.*

**Telemetry data types supported**

*Any combination of traces, metrics and/or logs is valid here.*

**Sponsor**

*A sponsor is an approver who will be in charge of being the official reviewer of the code. For vendor-specific components, it's good to have a volunteer sponsor. If you can't find one, we'll assign one in a round-robin fashion. For non-vendor components, having a sponsor means that your use case has been validated.*

**Checklist**

The following is a checklist for tracking the work required when adding a new component:

- [ ] Identify a sponsor
- [ ] Document README.md with some basic information about your component. It doesn't have to be complete, but should contain at least:
  - [ ] The purpose of the component
  - [ ] Use-cases
- [ ] A complete config.go, which will show how your component will look like to users
- [ ] Add component to `versions.yaml`
- [ ] `CODEOWNERS` with at least the sponsor and the component's author
- [ ] internal/components tests
- [ ] Add component to `go.mod` at the root of the repository and at `cmd/configschema`
- [ ] Enable Dependabot for your component by running `make gendependabot`.
- [ ] The skeleton of the component, potentially using the helpers
- [ ] Add Makefile
