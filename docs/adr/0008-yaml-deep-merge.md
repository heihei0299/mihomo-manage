# YAML deep merge for config templates

The config-template is now a YAML overlay that supplements subscription data via deep merge, replacing the previous simple string substitution (`{{placeholder}}`).

## Status

superseded by ADR-0007; retained as the historical predecessor to the override-file semantics

## Context

The original config-pipeline used simple string substitution: the template contained `{{subscription}}` and `{{routing_rules}}` placeholders that were replaced with raw subscription data and routing rules. This assumed the template was the "skeleton" and subscription data was the "filler".

In practice, most users get complete mihomo configurations from their subscription provider — including proxies, proxy-groups, rules, and top-level settings like port and mode. These users want to keep the subscription config as-is and only supplement it with custom settings (DNS, tun, additional rules, additional proxy-groups). The placeholder model cannot naturally express this "subscription-first, template-supplement" relationship.

## Considered Options

- **Simple string substitution (current)** — zero dependencies, but forces the template to be the outer container, which does not match the real use case where subscription data is the complete config
- **YAML deep merge** — requires a YAML library (`gopkg.in/yaml.v3`), but naturally expresses the "subscription as base, template as overlay" relationship
- **Dual mode** — support both placeholder and merge modes — adds complexity and user confusion

## Decision

Adopt YAML deep merge. Subscription data is the base config; the template is an overlay that only supplements missing keys and appends to specific allow-listed arrays (`proxies`, `proxy-groups`, `rules`, `proxy-providers`, `rule-providers`). Template values never override existing subscription keys.

## Consequences

- Old templates containing `{{subscription}}` or `{{routing_rules}}` placeholders must be migrated; the tool detects them and emits deprecation warnings
- New dependency: `gopkg.in/yaml.v3`
- The `SetRoutingRules` API and `rules.txt` file are no longer used (rules are embedded in the template)
- Config preview and generated config now respect YAML structure rather than treating everything as raw text
