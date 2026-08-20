# STRUCTURE.md — `components/Servers`

## `ServersCard.vue`

- Polls `GetServers()` every 10 seconds.
- Renders only actual server outbounds returned by the active config.
- Derives the selector buttons from that list plus the explicit `auto` policy;
  no NL/FR hardcoded options are shown when absent.
- Displays the exact active selector default and TCP ping.
- Calls `SelectServer(tag)`; Manager rejects a tag outside selector `proxy`.

## `RoutingCard.vue`

- Polls `GetRouteRules()`.
- Backend IDs use the original `route.rules` index (`rule-N`). UI service-rule
  filtering therefore cannot shift a toggle onto `sniff`/`hijack-dns`.
- Toggle requests go through `ToggleRouteRule(index, enabled)` and Manager
  normalization maps protected `auto` policy to selector `proxy`.

Both cards get Wails methods from generated bindings and use the global toast
provided by `App.vue`.
