# Simple string substitution for config templates

The config-template uses simple string substitution (`{{placeholder}}`) rather than a full template engine like Jinja2 or Go templates.

## Context

The manager merges subscription data and user-defined routing rules into a final mihomo config file. The template is user-editable and stored alongside the instance config.

## Considered Options

- **Jinja2 / Go template** — powerful and widely used, but introduces a dependency and a learning curve for users editing the template by hand
- **Simple string substitution** — zero dependencies, trivially understandable by any user, but cannot express conditionals or loops

## Decision

Use simple string substitution. The mihomo config format is YAML with a relatively fixed structure; subscription data fills predetermined sections. The extra power of a full template engine would rarely be used and would make the template harder for users to understand and edit directly.

## Consequences

- If users need conditional sections or loops in the future, the template engine will need to be upgraded to a more capable system, breaking existing templates
