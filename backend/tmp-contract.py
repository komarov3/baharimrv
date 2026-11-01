"""Summarise the BahariMRV HTTP contract from the running dev backend.

Prints the JSON shape (keys, types, sample scalars) for every endpoint the
Android client consumes, so Kotlin DTOs are written against live evidence.
"""
import json
import urllib.error
import urllib.request

BASE = "http://localhost:8099/api/v1"
TOKEN = ""


def call(method, path, body=None, headers=None):
    data = json.dumps(body).encode() if body is not None else None
    req = urllib.request.Request(BASE + path, data=data, method=method)
    req.add_header("Content-Type", "application/json")
    if TOKEN:
        req.add_header("Authorization", "Bearer " + TOKEN)
    for key, value in (headers or {}).items():
        req.add_header(key, value)
    try:
        with urllib.request.urlopen(req, timeout=30) as response:
            raw = response.read()
            return response.status, json.loads(raw) if raw else None
    except urllib.error.HTTPError as error:
        raw = error.read()
        try:
            return error.code, json.loads(raw)
        except Exception:
            return error.code, raw.decode(errors="replace")


def shape(value, depth=0):
    """Render a compact type skeleton for a JSON value."""
    pad = "  " * depth
    if isinstance(value, dict):
        if not value:
            return "{}"
        lines = []
        for key, item in value.items():
            if isinstance(item, (dict, list)):
                lines.append(f"{pad}{key}:")
                lines.append(f"{_inline(item, depth + 1)}")
            else:
                lines.append(f"{pad}{key}: { _scalar(item) }")
        return "\n".join(lines)
    return _inline(value, depth)


def _inline(value, depth):
    pad = "  " * depth
    if isinstance(value, list):
        if not value:
            return f"{pad}[] (empty)"
        first = value[0]
        if isinstance(first, (dict, list)):
            return f"{pad}[{len(value)} items] first:\n{shape(first, depth + 1)}"
        return f"{pad}[{len(value)} items] first: {_scalar(first)}"
    if isinstance(value, dict):
        return f"{pad}{{...}}\n{shape(value, depth + 1)}"
    return pad + _scalar(value)


def _scalar(value):
    if isinstance(value, str):
        text = value if len(value) <= 46 else value[:46] + "\u2026"
        return json.dumps(text)
    if value is None:
        return "null"
    if isinstance(value, bool):
        return "bool"
    if isinstance(value, (int, float)):
        return "number"
    return type(value).__name__


def show(label, method, path, body=None, headers=None):
    status, payload = call(method, path, body, headers)
    print(f"\n### {label}  [{method} {path}] -> {status}")
    if isinstance(payload, (dict, list)):
        print(shape(payload))
    else:
        print("   ", str(payload)[:300])


status, login = call("POST", "/auth/login", {
    "email": "manager@demo.baharimrv.local", "password": "Demo-Passw0rd-123"})
if status != 200:
    raise SystemExit(f"login failed: {status} {login}")
TOKEN = login["sessionToken"]
print("logged in as", login["user"]["role"], "identity state", login["identity"]["state"])

show("projects", "GET", "/projects")
_, projects = call("GET", "/projects")
rows = projects.get("rows") if isinstance(projects, dict) else None
project_id = rows[0]["id"] if rows else ""
activity_id = "55b6baf9-fc3b-41f5-8436-155bd3861a53"
show("dashboard", "GET", f"/projects/{project_id}/dashboard")
show("wallet", "GET", f"/projects/{project_id}/wallet")
show("documents", "GET", f"/projects/{project_id}/documents")
show("reconciliation", "GET", f"/projects/{project_id}/reconciliation")
show("identity", "GET", "/users/me/identity")
show("links", "GET", "/auth/links")
show("listings", "GET", "/marketplace/listings")
show("sync-delta", "GET", "/sync/delta?since=0")
show("provenance", "GET", f"/activities/{activity_id}/provenance")
show("verification-requests", "GET", "/verification-requests")
show("proofs", "GET", f"/achievements/{activity_id}/proofs")
show("proof-generate", "POST", f"/achievements/{activity_id}/proofs", {}, {"Idempotency-Key": "contract-probe-1"})
print("\nPROJECT_ID=" + project_id)
