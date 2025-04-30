name: 🐛 Bug Report
description: File a bug report for an unexpected or broken behavior
title: "[Bug] <describe the issue briefly>"
labels: [bug, needs-triage]
body:
  - type: markdown
    attributes:
      value: |
        Thanks for reporting a bug! Please provide enough information for us to understand, reproduce, and fix it.
  - type: input
    id: version
    attributes:
      label: quartzctl version
      placeholder: e.g. v0.4.1 or commit hash
    validations:
      required: true
  - type: textarea
    id: describe
    attributes:
      label: What happened?
      description: Describe the bug, expected behavior, and what actually happened.
    validations:
      required: true
  - type: textarea
    id: reproduce
    attributes:
      label: Steps to Reproduce
      description: Provide a minimal, complete set of steps to reproduce the bug.
      placeholder: |
        1. Run 'quartzctl apply ...'
        2. Observe error: ...
    validations:
      required: true
  - type: textarea
    id: logs
    attributes:
      label: Relevant logs or output
      description: Share any error messages, logs, or stack traces.
      render: shell
  - type: dropdown
    id: os
    attributes:
      label: OS
      options:
        - macOS
        - Linux
        - Windows
        - Other (explain in bug description)
    validations:
      required: true
  - type: input
    id: go-version
    attributes:
      label: Go Version
      placeholder: e.g. go1.22
