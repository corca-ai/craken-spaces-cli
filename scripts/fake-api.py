#!/usr/bin/env python3
"""Minimal fake control-plane API for specdown specs.

Serves canned JSON responses that satisfy the OpenAPI contract.
Maintains just enough in-memory state for multi-step CLI flows.

Usage:
    python3 scripts/fake-api.py          # prints http://127.0.0.1:PORT
    FAKE_API_PORT=9999 python3 ...       # use fixed port
"""

import json
import os
import re
import sys
from http.server import HTTPServer, BaseHTTPRequestHandler

# ---------------------------------------------------------------------------
# In-memory state
# ---------------------------------------------------------------------------
sessions = {}          # token -> email
ssh_keys = {}          # fingerprint -> record
spaces = {}            # id -> record
member_auth_keys = {}  # (space_id, key_id) -> record
next_ssh_key_id = 1
next_member_key_id = 1
DEFAULT_AUTH_KEY = "test-key"

SPACE_TEMPLATE = {
    "id": "", "name": "", "role": "admin", "owner_user_id": 1,
    "created_at": "2026-01-01T00:00:00Z",
    "cpu_millis": 4000, "memory_mib": 8192, "disk_mb": 10240,
    "network_egress_mb": 1024, "llm_tokens_limit": 100000, "llm_tokens_used": 0,
    "actor_cpu_millis": 0, "actor_memory_mib": 0, "actor_disk_mb": 0,
    "actor_network_mb": 0, "actor_llm_tokens": 0, "byok_bytes_used": 0,
    "runtime_driver": "mock", "runtime_state": "stopped", "runtime_meta": "",
}


def space_record(space_id, name, **overrides):
    rec = dict(SPACE_TEMPLATE)
    rec["id"] = space_id
    rec["name"] = name
    rec.update(overrides)
    return rec


def validate_login(email, key):
    email = str(email or "").strip()
    key = str(key or "").strip()
    if not email or not key:
        return None, "email and key are required"
    if key == DEFAULT_AUTH_KEY:
        return True, None
    for rec in member_auth_keys.values():
        if key != "wmauth_fake_" + str(rec["id"]):
            continue
        if rec["invitee_email"] != email:
            return None, "invalid auth key"
        if rec["revoked_at"]:
            return None, "auth key revoked"
        if rec["redeemed_at"]:
            return None, "auth key already redeemed"
        rec["redeemed_at"] = "2026-01-02T00:00:00Z"
        return True, None
    return None, "invalid auth key"


# ---------------------------------------------------------------------------
# Request handler
# ---------------------------------------------------------------------------
class Handler(BaseHTTPRequestHandler):
    def log_message(self, fmt, *args):
        pass  # silence request logging

    def _send_json(self, status, body):
        payload = json.dumps(body).encode()
        self.send_response(status)
        self.send_header("Content-Type", "application/json")
        self.send_header("Content-Length", str(len(payload)))
        self.end_headers()
        self.wfile.write(payload)

    def _read_json(self):
        length = int(self.headers.get("Content-Length", 0))
        if length == 0:
            return {}
        return json.loads(self.rfile.read(length))

    def _require_auth(self):
        auth = self.headers.get("Authorization", "")
        if not auth.startswith("Bearer "):
            self._send_json(401, {"ok": False, "error": "unauthorized"})
            return None
        token = auth[len("Bearer "):]
        email = sessions.get(token)
        if not email:
            self._send_json(401, {"ok": False, "error": "invalid session"})
            return None
        return email

    # -- routing helpers --
    def _match(self, method, pattern):
        if self.command != method:
            return None
        return re.fullmatch(pattern, self.path)

    def do_GET(self):
        self._dispatch("GET")

    def do_POST(self):
        self._dispatch("POST")

    def do_DELETE(self):
        self._dispatch("DELETE")

    def _dispatch(self, method):
        global next_ssh_key_id, next_member_key_id
        path = self.path

        # --- healthz ---
        if method == "GET" and path == "/healthz":
            self._send_json(200, {"ok": True})
            return

        # --- auth/login (no auth required) ---
        if method == "POST" and path == "/api/v1/auth/login":
            body = self._read_json()
            email = body.get("email", "")
            ok, error = validate_login(email, body.get("key", ""))
            if not ok:
                self._send_json(400, {"ok": False, "error": error})
                return
            token = "sess_" + email.split("@")[0]
            sessions[token] = email
            self._send_json(200, {
                "ok": True,
                "email": email,
                "session_token": token,
            })
            return

        # --- auth/logout ---
        if method == "POST" and path == "/api/v1/auth/logout":
            email = self._require_auth()
            if email is None:
                return
            auth = self.headers.get("Authorization", "")
            token = auth[len("Bearer "):]
            sessions.pop(token, None)
            self._send_json(200, {"ok": True})
            return

        # --- whoami ---
        if method == "GET" and path == "/api/v1/whoami":
            email = self._require_auth()
            if email is None:
                return
            self._send_json(200, {
                "ok": True,
                "user": {"id": 1, "email": email, "name": email.split("@")[0]},
            })
            return

        # --- spaces ---
        if method == "GET" and path == "/api/v1/spaces":
            email = self._require_auth()
            if email is None:
                return
            self._send_json(200, {
                "ok": True,
                "spaces": list(spaces.values()),
            })
            return

        if method == "POST" and path == "/api/v1/spaces":
            email = self._require_auth()
            if email is None:
                return
            body = self._read_json()
            space_id = "sp_" + str(len(spaces) + 1)
            rec = space_record(space_id, body.get("name", ""),
                               runtime_driver=body.get("runtime_driver", "mock"),
                               cpu_millis=body.get("cpu_millis", 4000),
                               memory_mib=body.get("memory_mib", 8192),
                               disk_mb=body.get("disk_mb", 10240),
                               network_egress_mb=body.get("network_egress_mb", 1024),
                               llm_tokens_limit=body.get("llm_tokens_limit", 100000))
            spaces[space_id] = rec
            self._send_json(200, {"ok": True, "space": rec})
            return

        # space up
        m = re.fullmatch(r"/api/v1/spaces/([^/]+)/up", path)
        if method == "POST" and m:
            email = self._require_auth()
            if email is None:
                return
            space_id = m.group(1)
            rec = spaces.get(space_id, space_record(space_id, "unknown"))
            rec["runtime_state"] = "running"
            spaces[space_id] = rec
            self._send_json(200, {"ok": True, "space": rec})
            return

        # space down
        m = re.fullmatch(r"/api/v1/spaces/([^/]+)/down", path)
        if method == "POST" and m:
            email = self._require_auth()
            if email is None:
                return
            space_id = m.group(1)
            rec = spaces.get(space_id, space_record(space_id, "unknown"))
            rec["runtime_state"] = "stopped"
            spaces[space_id] = rec
            self._send_json(200, {"ok": True, "space": rec})
            return

        # space delete
        m = re.fullmatch(r"/api/v1/spaces/([^/]+)/delete", path)
        if method == "DELETE" and m:
            email = self._require_auth()
            if email is None:
                return
            space_id = m.group(1)
            spaces.pop(space_id, None)
            self._send_json(200, {"ok": True})
            return

        # member auth keys - list
        m = re.fullmatch(r"/api/v1/spaces/([^/]+)/member-auth-keys", path)
        if method == "GET" and m:
            email = self._require_auth()
            if email is None:
                return
            space_id = m.group(1)
            keys = [v for (sid, _), v in member_auth_keys.items() if sid == space_id]
            self._send_json(200, {"ok": True, "auth_keys": keys})
            return

        # member auth keys - issue
        if method == "POST" and m:
            email = self._require_auth()
            if email is None:
                return
            space_id = m.group(1)
            body = self._read_json()
            key_id = next_member_key_id
            next_member_key_id += 1
            rec = {
                "id": key_id,
                "space_id": space_id,
                "space_name": spaces.get(space_id, {}).get("name", "unknown"),
                "issued_by_user_id": 1,
                "issued_by_email": email,
                "invitee_email": body.get("email", ""),
                "issued_at": "2026-01-01T00:00:00Z",
                "expires_at": "2026-01-08T00:00:00Z",
                "redeemed_at": "",
                "revoked_at": "",
                "cpu_millis": body.get("cpu_millis", 1000),
                "memory_mib": body.get("memory_mib", 1024),
                "disk_mb": body.get("disk_mb", 1024),
                "network_egress_mb": body.get("network_egress_mb", 256),
                "llm_tokens_limit": body.get("llm_tokens_limit", 10000),
            }
            member_auth_keys[(space_id, key_id)] = rec
            self._send_json(200, {
                "ok": True,
                "auth_key": rec,
                "key": "wmauth_fake_" + str(key_id),
            })
            return

        # member auth keys - revoke
        m = re.fullmatch(r"/api/v1/spaces/([^/]+)/member-auth-keys/(\d+)", path)
        if method == "DELETE" and m:
            email = self._require_auth()
            if email is None:
                return
            space_id = m.group(1)
            key_id = int(m.group(2))
            rec = member_auth_keys.get((space_id, key_id))
            if rec:
                rec["revoked_at"] = "2026-01-02T00:00:00Z"
            self._send_json(200, {"ok": True})
            return

        # --- SSH keys ---
        if method == "GET" and path == "/api/v1/ssh/keys":
            email = self._require_auth()
            if email is None:
                return
            self._send_json(200, {"ok": True, "keys": list(ssh_keys.values())})
            return

        if method == "POST" and path == "/api/v1/ssh/keys":
            email = self._require_auth()
            if email is None:
                return
            body = self._read_json()
            key_id = next_ssh_key_id
            next_ssh_key_id += 1
            fingerprint = "SHA256:fake" + str(key_id)
            rec = {
                "id": key_id,
                "user_id": 1,
                "user_email": email,
                "name": body.get("name", ""),
                "public_key": body.get("public_key", ""),
                "fingerprint": fingerprint,
                "created_at": "2026-01-01T00:00:00Z",
            }
            ssh_keys[fingerprint] = rec
            self._send_json(200, {"ok": True, "key": rec})
            return

        m = re.fullmatch(r"/api/v1/ssh/keys/(.+)", path)
        if method == "DELETE" and m:
            email = self._require_auth()
            if email is None:
                return
            fingerprint = m.group(1)
            ssh_keys.pop(fingerprint, None)
            self._send_json(200, {"ok": True})
            return

        # --- SSH issue-cert ---
        if method == "POST" and path == "/api/v1/ssh/issue-cert":
            email = self._require_auth()
            if email is None:
                return
            body = self._read_json()
            self._send_json(200, {
                "ok": True,
                "fingerprint": "SHA256:fakecert",
                "principal": body.get("principal", "spaces-room"),
                "expires_at": "2026-01-01T00:05:00Z",
                "certificate": "ssh-ed25519-cert-v01@openssh.com AAAA_FAKE_CERT\n",
            })
            return

        # --- fallback ---
        self._send_json(404, {"ok": False, "error": "not found: " + path})


def main():
    port = int(os.environ.get("FAKE_API_PORT", "0"))
    server = HTTPServer(("127.0.0.1", port), Handler)
    actual_port = server.server_address[1]
    # Print the URL so callers can capture it
    print(f"http://127.0.0.1:{actual_port}", flush=True)
    server.serve_forever()


if __name__ == "__main__":
    main()
