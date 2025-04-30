name: 💡 Feature Request
description: Suggest a new idea or enhancement
title: "[Feature] <concise summary of suggestion>"
labels: [enhancement, needs-triage]
body:
  - type: markdown
    attributes:
      value: |
        Thanks for taking the time to suggest a feature! Please complete the form below so we can evaluate your idea.
  - type: textarea
    id: description
    attributes:
      label: What problem would this feature solve?
      description: Explain what you're trying to accomplish and why the current behavior is insufficient.
    validations:
      required: true
  - type: textarea
    id: solution
    attributes:
      label: What do you suggest as a solution?
      description: Describe the feature, command, flag, or enhancement you'd like to see.
  - type: textarea
    id: alternatives
    attributes:
      label: Alternatives considered
      description: Have you considered any other approaches?
  - type: dropdown
    id: scope
    attributes:
      label: Is this feature specific to a particular stage of the workflow?
      options:
        - Planning
        - Init
        - Apply
        - Destroy
        - All stages
        - Other (explain in detail)
