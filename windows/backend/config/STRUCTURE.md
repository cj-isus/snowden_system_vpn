# STRUCTURE.md — `windows/backend/config`

Package `config` is the pre-runtime safety boundary. It does not build a VPN
config from hardcoded country lists; it normalizes and validates JSON supplied by
templates, import, UI or recovery.

## Files

| File | Role |
|---|---|
| `validator.go` | JSON graph checks, placeholder scan, protected selector normalization and default selection |
| `validator_test.go` | missing references, placeholders, direct fallback and urltest normalization |
| `builder.go` | legacy `VPSConfig` reference type; not called by the live path |

## Runtime pipeline

```text
raw JSON
  → NormalizeProtectedRoute
      legacy urltest/auto → selector proxy
      direct removed from protected candidates
      protected route rules → proxy
  → Validate(RequireFailClosed: true)
      tags, route, DNS and inbound references
      no placeholders
      no direct in protected selector
      explicit selector default
  → embedded box.New
```

Protocol-specific schema validation is still performed by the exact embedded
sing-box build. This package only validates the generic reference graph and the
application safety policy.

## Safety invariants

- `route.final` cannot be `direct` in runtime mode.
- selector `proxy` may contain only validated protected tags and must have a
  candidate default.
- explicit `direct` rules remain possible for private/RU split-tunnel policy.
- public examples may contain placeholders only when validation is called with
  `AllowPlaceholders: true`; runtime mode never does that.
