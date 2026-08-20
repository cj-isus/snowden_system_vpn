"""Shared local environment loading for operator scripts.

The loader reads only local ignored files and never prints values. Existing
process environment variables win over file values, so CI and explicit shell
configuration remain authoritative.
"""
from __future__ import annotations

import os
from pathlib import Path

PROJECT_ROOT = Path(__file__).resolve().parent.parent


def _candidate_paths() -> list[Path]:
    paths: list[Path] = []
    explicit = os.environ.get("SNOWDEN_ENV_FILE")
    if explicit:
        paths.append(Path(explicit).expanduser())
    paths.extend(
        [
            PROJECT_ROOT / "configs" / "env" / ".env",
            PROJECT_ROOT / ".env",
            Path.cwd() / ".env",
        ]
    )
    seen: set[Path] = set()
    result: list[Path] = []
    for path in paths:
        resolved = path.expanduser().resolve()
        if resolved not in seen:
            seen.add(resolved)
            result.append(resolved)
    return result


def load_project_env() -> None:
    """Load the first available local env file without overriding env vars."""
    for path in _candidate_paths():
        try:
            lines = path.read_text(encoding="utf-8").splitlines()
        except OSError:
            continue
        for raw in lines:
            line = raw.strip()
            if not line or line.startswith("#") or "=" not in line:
                continue
            key, value = line.split("=", 1)
            key = key.strip()
            value = value.strip()
            if not key or key.startswith("#"):
                continue
            if len(value) >= 2 and value[0] == value[-1] and value[0] in "\"'":
                value = value[1:-1]
            os.environ.setdefault(key, value)
        return


def require_env(name: str) -> str:
    """Return a required value with an actionable error, never its contents."""
    value = os.environ.get(name, "")
    if not value:
        raise RuntimeError(f"required environment variable is missing: {name}")
    return value
