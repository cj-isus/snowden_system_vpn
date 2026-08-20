# Operator script environment

Python tools load the first available local profile from:

1. `SNOWDEN_ENV_FILE` (explicit path);
2. `configs/env/.env`;
3. project `.env`;
4. current-directory `.env`.

Existing process environment variables always win. Values are never printed by
the loader.

Minimum diagnostic variables:

```text
SNOWDEN_VPS_IP
SNOWDEN_VPS_SSH_USER=root
SNOWDEN_VPS_SSH_PASSWORD
SNOWDEN_VPS_SSH_PORT=22
```

Provisioning/build tools additionally require only the values they use, such as
`SNOWDEN_HY2_PASSWORD`, `SNOWDEN_VPN_DOMAIN`,
`SNOWDEN_CLIENT_CONFIG_PATH` and `SNOWDEN_RUNTIME_CONFIG_PATH`.

Never put `.env`, generated configs, private keys or Telegram tokens in a
public artifact. Run scripts explicitly and review their target paths before
any SSH/restart/deploy operation.
