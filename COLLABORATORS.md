# Collaborator onboarding — snowden_system_vpn

> Direct push to `master` is allowed on this repository. Read this file end
> to end before the first commit. It is the contract between you and the
> Windows/embedded sing-box adapter that already lives in this branch.

## 1. Access setup

1. Make sure the maintainer has invited your GitHub account to
   `github.com/cj-isus/snowden_system_vpn` as a **Collaborator** (push role).
   Accept the invite from `https://github.com/notifications`.
2. Enable 2FA on your GitHub account before cloning.
3. Generate a personal SSH key (ed25519 is fine) and add the public half to
   GitHub → Settings → SSH and GPG keys.
4. Clone with SSH so credential prompts stay out of the codebase:

   ```bash
   git clone git@github.com:cj-isus/snowden_system_vpn.git
   cd snowden_system_vpn
   git config user.name  "Your Name"
   git config user.email "you@example.com"
   ```

   HTTPS works too once you create a fine-scoped personal access token; do
   not commit the token to disk in cleartext.

## 2. Toolchain you must install locally

The repository ships a Windows-first Go backend, a Vue3 / TypeScript
frontend, Flutter Android client, optional Swift/Cocoa iOS client and a
set of Python diagnostic scripts.

Mandatory on Windows 10/11:

| Tool | Version | Why |
|---|---|---|
| Go | 1.25+ (the project uses `go 1.25.0`) | Backend, sing-box-lx, adaptive engine |
| Node | 20.x | Vue3 frontend build |
| Python | 3.11+ | `scripts/*.py` (paramiko, json, subprocess) |
| JDK | 17 (Eclipse Adoptium) | Flutter / Android |
| Git for Windows | latest | shell quoting inside `build_android.bat` |

Optional, only if you work on the mobile clients:

- Flutter SDK + Android SDK (build-tools, platform-tools)
- `adb devices` must show your phone as `device`, not `unauthorized`.
- For iOS: macOS host with Xcode 15+, `gomobile`, `xcodegen`.

## 3. Secrets — read this twice

There are NO real secrets in the repository:

- Public docs use `YOUR_VPS_IP`, `YOUR_UUID`, `YOUR_HY2_PASSWORD`, etc.
- Public examples use `YOUR_CLASH_API_SECRET`, `YOUR_VPS_IP_FR`, etc.
- The Telegram token, UUIDs, HY2 password, WARP private key and SSH
  credentials are NEVER to be committed. Whoever has them now must
  treat them as **already leaked** and rotate them.

For your local checkout, populate `configs/env/.env` (git-ignored) using
the template in `configs/env/.env.example`. Minimum required variables:

```text
SNOWDEN_VPS_IP=...
SNOWDEN_VPS_SSH_USER=root
SNOWDEN_VPS_SSH_PASSWORD=...
SNOWDEN_VPS_SSH_PORT=22
SNOWDEN_VPS_UUID=...
SNOWDEN_HY2_PASSWORD=...
SNOWDEN_VPN_DOMAIN=...
SNOWDEN_CLIENT_CONFIG_PATH=...
SNOWDEN_RUNTIME_CONFIG_PATH=...
SNOWDEN_BUILD_CONFIG_PATH=...
SNOWDEN_PORTABLE_CONFIG_PATH=...
SNOWDEN_TG_TOKEN=...
SNOWDEN_TG_CHAT_ID=...
SNOWDEN_REALITY_PRIVATE_KEY=...
SNOWDEN_REALITY_SHORT_ID=...
```

`scripts/env.py` honours `SNOWDEN_ENV_FILE`, then `configs/env/.env`,
then the project root `.env`, then the cwd `.env`. Process env always
wins over file values.

## 4. Verify the checkout before touching the code

```powershell
cd D:\snowden-v2
"C:\Program Files\Go\bin\go.exe" test  -race=false ./backend/...
"C:\Program Files\Go\bin\go.exe" vet  ./...
"C:\Program Files\Go\bin\go.exe" build -tags "with_awg,with_wireguard,with_utls,with_gvisor" ./...
cd .\windows\frontend ; npm ci ; npm run build ; cd ..\..
node --check configs\cloudflare\worker.js
python -m py_compile scripts\*.py
```

All five sections must pass before you push. `go test -race` requires a
C compiler on Windows; if you do not have MSYS2, fall back to plain
`go test`.

## 5. Branch and commit policy

- Default branch: `master`. Direct push is allowed for both owners.
- For risky multi-file work, cut a feature branch:

  ```bash
  git checkout -b feat/<slug> master
  …
  git push -u origin feat/<slug>
  ```

  and merge with a fast-forward once tests are green. Open a PR if you
  want a second pair of eyes.
- Commit messages: imperative mood, ≤ 72 chars for the title. Add a body
  that describes **why** the change exists, not **what** (the diff shows
  the what).
- Never rewrite history already on `master` (`git reset --hard`,
  `git push --force`, `git rebase --interactive master`).

## 6. Component landscape

Read these before touching a directory:

| File | Purpose |
|---|---|
| `PLAN.md` | Single source of truth for what the system should do |
| `STRUCTURE.md` | Top-level architecture map |
| `windows/STRUCTURE.md` | Wails app + facade |
| `windows/backend/core/STRUCTURE.md` | lifecycle, selector, circuit breaker, memory |
| `windows/backend/config/STRUCTURE.md` | validator + normalization contract |
| `windows/frontend/src/STRUCTURE.md` | Vue3 frontend |
| `windows/frontend/src/components/{Servers,Dashboard,Settings}/STRUCTURE.md` | card contracts |
| `android/STRUCTURE.md`, `ios/STRUCTURE.md` | mobile clients |
| `scripts/STRUCTURE.md`, `scripts/ENVIRONMENT.md` | operator tools |
| `configs/{singbox,cloudflare,landing}/STRUCTURE.md` + `configs/README.md` | config layout |

If a `STRUCTURE.md` disagrees with the code, the code wins. Open a PR
that fixes the doc.

## 7. What you should NOT change

- `windows/backend/core/registry.go` — sing-box protocol registration
  is deliberate; it avoids the `protocol/naive → cronet-go/all` import
  graph that fails under RU-restricted proxies.
- `windows/app.go:loadEnvFile` — order of `.env` candidates is
  intentional; do not add `os.Getenv("HOME")` style paths.
- `configs/cloudflare/worker.js` — security gates (`publicConfig` strips
  UUID/password/private_key/token/secret fields) are mandatory.
- `scripts/{deploy,setup-vps,fix-keypair}.py` — they intentionally mask
  or never log credentials. Adding `print(f"password: {PASS}")` will be
  reverted.

## 8. Verifying a feature end-to-end

After a code change, in this order:

1. `go test ./backend/...` — unit-level correctness.
2. `go vet ./...` — static analysis.
3. `go build -tags "with_awg,with_wireguard,with_utls,with_gvisor" ./...`
   — produces a Wails build candidate.
4. `npm run build` — frontend must type-check (`vue-tsc`).
5. `python -m py_compile scripts/*.py` — operator tools must still parse.
6. Live check on Windows: launch the binary, hit Start, open the
   Dashboard, confirm:
   - Status flips to `running` and uses an actual `GetServers()` channel.
   - Diagnostics are not hardcoded `"VPS Netherlands"` / `"VLESS+TLS"`.
   - TrafficCard says "нет данных" instead of 0 B/s when Clash is off.
   - No system proxy is left enabled after a failed Start.

## 9. Asking for help

- Bugs / questions: open an issue titled `[phase-N] <summary>` and tag
  the file path.
- Security issues: do NOT post the leak in public. DM the maintainer.
- Plan changes: file a PR against `PLAN.md` and discuss in the issue
  before writing code.

## 10. Quick map of identity

| Where identity is set | Value |
|---|---|
| Repo URL | `https://github.com/cj-isus/snowden_system_vpn` |
| Maintainer GitHub login | `cj-isus` |
| Default branch | `master` |
| Visibility | public (secrets are sanitised) |
| Local Android AAR | NOT in git; `build_android.bat` downloads it |
| Local env file | `configs/env/.env` (git-ignored) |
