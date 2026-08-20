# STRUCTURE.md — `components/Settings`

`SettingsCard.vue` owns user-facing local settings:

- `IsAutostartEnabled` / `SetAutostart` manage the Windows startup toggle.
- `ImportConfig` reads a file through Wails, normalizes and fail-closed validates
  it, then stages it in App memory for the next Start.
- `ExportConfig` writes the active normalized runtime snapshot.

Import never writes into source templates or the repository and never accepts a
config that contains placeholders, missing references or a protected `direct`
fallback. Wails dialog APIs remain dynamically typed because they are runtime
bindings rather than application business logic.
