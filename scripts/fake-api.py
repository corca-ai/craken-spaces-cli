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
import urllib.parse
from http.server import HTTPServer, BaseHTTPRequestHandler

# ---------------------------------------------------------------------------
# In-memory state
# ---------------------------------------------------------------------------
sessions = {}               # token -> session dict
ssh_keys = {}               # fingerprint -> record
ssh_key_owners = {}         # fingerprint -> email
spaces = {}                 # id -> record
space_owners = {}           # space_id -> owner email
space_members = {}          # space_id -> set(email)
member_auth_keys = {}       # (space_id, key_id) -> record
member_auth_key_values = {} # (space_id, key_id) -> auth key
issued_auth_keys = {}       # auth key -> record
next_ssh_key_id = 1
next_member_key_id = 1
next_auth_key_id = 1
next_session_id = 1

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


def issue_test_auth_key(email, role="admin", space_id="", member_key_ref=None):
    global next_auth_key_id
    email = str(email or "").strip()
    role = str(role or "").strip()
    space_id = str(space_id or "").strip()
    if not email:
        return None, "email is required"
    if role not in {"admin", "member"}:
        return None, "role must be admin or member"
    if role == "member" and not space_id:
        return None, "space_id is required for member auth keys"
    key = "auth_fake_" + str(next_auth_key_id)
    next_auth_key_id += 1
    issued_auth_keys[key] = {
        "email": email,
        "role": role,
        "space_id": space_id,
        "redeemed_at": "",
        "revoked_at": "",
        "member_key_ref": member_key_ref,
    }
    return key, None


def validate_login(email, key):
    email = str(email or "").strip()
    key = str(key or "").strip()
    if not email or not key:
        return None, "email and key are required"
    rec = issued_auth_keys.get(key)
    if rec is None:
        return None, "invalid auth key"
    if rec["email"] != email:
        return None, "invalid auth key"
    if rec["revoked_at"]:
        return None, "auth key revoked"
    if rec["redeemed_at"]:
        return None, "auth key already redeemed"
    rec["redeemed_at"] = "2026-01-02T00:00:00Z"
    if rec["member_key_ref"] is not None:
        member_auth_keys[rec["member_key_ref"]]["redeemed_at"] = rec["redeemed_at"]
        space_members.setdefault(rec["space_id"], set()).add(email)
    return {"role": rec["role"], "space_id": rec["space_id"]}, None


def space_role_for_session(session, space_id):
    if space_owners.get(space_id) == session["email"]:
        return "admin"
    if (
        session.get("role") == "member"
        and session.get("space_id") == space_id
        and session["email"] in space_members.get(space_id, set())
    ):
        return "member"
    return ""


def visible_space_records(session):
    visible = []
    for space_id, rec in spaces.items():
        role = space_role_for_session(session, space_id)
        if not role:
            continue
        row = dict(rec)
        row["role"] = role
        visible.append(row)
    return visible


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

    def _send_text(self, status, body):
        payload = str(body).encode()
        self.send_response(status)
        self.send_header("Content-Type", "text/plain; charset=utf-8")
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
        session = sessions.get(token)
        if not session:
            self._send_json(401, {"ok": False, "error": "invalid session"})
            return None
        return session

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
        global next_ssh_key_id, next_member_key_id, next_session_id
        parsed = urllib.parse.urlparse(self.path)
        path = parsed.path
        query = urllib.parse.parse_qs(parsed.query)

        # --- healthz ---
        if method == "GET" and path == "/healthz":
            self._send_json(200, {"ok": True})
            return

        if method == "GET" and path == "/__test/issue-auth-key":
            email = query.get("email", [""])[0]
            role = query.get("role", ["admin"])[0]
            space_id = query.get("space_id", [""])[0]
            key, error = issue_test_auth_key(email, role, space_id)
            if error is not None:
                self._send_json(400, {"ok": False, "error": error})
                return
            self._send_text(200, key)
            return

        # --- auth/login (no auth required) ---
        if method == "POST" and path == "/api/v1/auth/login":
            body = self._read_json()
            email = body.get("email", "")
            auth_context, error = validate_login(email, body.get("key", ""))
            if auth_context is None:
                self._send_json(400, {"ok": False, "error": error})
                return
            token = "sess_" + str(next_session_id)
            next_session_id += 1
            sessions[token] = {
                "email": email,
                "role": auth_context["role"],
                "space_id": auth_context["space_id"],
            }
            self._send_json(200, {
                "ok": True,
                "email": email,
                "session_token": token,
            })
            return

        # --- auth/logout ---
        if method == "POST" and path == "/api/v1/auth/logout":
            session = self._require_auth()
            if session is None:
                return
            auth = self.headers.get("Authorization", "")
            token = auth[len("Bearer "):]
            sessions.pop(token, None)
            self._send_json(200, {"ok": True})
            return

        # --- whoami ---
        if method == "GET" and path == "/api/v1/whoami":
            session = self._require_auth()
            if session is None:
                return
            self._send_json(200, {
                "ok": True,
                "user": {"id": 1, "email": session["email"], "name": session["email"].split("@")[0]},
            })
            return

        # --- spaces ---
        if method == "GET" and path == "/api/v1/spaces":
            session = self._require_auth()
            if session is None:
                return
            self._send_json(200, {
                "ok": True,
                "spaces": visible_space_records(session),
            })
            return

        if method == "POST" and path == "/api/v1/spaces":
            session = self._require_auth()
            if session is None:
                return
            if session["role"] != "admin":
                self._send_json(403, {"ok": False, "error": "forbidden"})
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
            rec["runtime_state"] = "running"
            spaces[space_id] = rec
            space_owners[space_id] = session["email"]
            space_members.setdefault(space_id, set())
            self._send_json(200, {"ok": True, "space": rec})
            return

        # space up
        m = re.fullmatch(r"/api/v1/spaces/([^/]+)/up", path)
        if method == "POST" and m:
            session = self._require_auth()
            if session is None:
                return
            space_id = m.group(1)
            role = space_role_for_session(session, space_id)
            if not role:
                self._send_json(403, {"ok": False, "error": "forbidden"})
                return
            rec = dict(spaces.get(space_id, space_record(space_id, "unknown")))
            rec["runtime_state"] = "running"
            spaces[space_id] = rec
            rec["role"] = role
            self._send_json(200, {"ok": True, "space": rec})
            return

        # space down
        m = re.fullmatch(r"/api/v1/spaces/([^/]+)/down", path)
        if method == "POST" and m:
            session = self._require_auth()
            if session is None:
                return
            space_id = m.group(1)
            role = space_role_for_session(session, space_id)
            if not role:
                self._send_json(403, {"ok": False, "error": "forbidden"})
                return
            rec = dict(spaces.get(space_id, space_record(space_id, "unknown")))
            rec["runtime_state"] = "stopped"
            spaces[space_id] = rec
            rec["role"] = role
            self._send_json(200, {"ok": True, "space": rec})
            return

        # space delete
        m = re.fullmatch(r"/api/v1/spaces/([^/]+)/delete", path)
        if method == "DELETE" and m:
            session = self._require_auth()
            if session is None:
                return
            space_id = m.group(1)
            if space_role_for_session(session, space_id) != "admin":
                self._send_json(403, {"ok": False, "error": "forbidden"})
                return
            spaces.pop(space_id, None)
            space_owners.pop(space_id, None)
            space_members.pop(space_id, None)
            self._send_json(200, {"ok": True})
            return

        # member auth keys - list
        m = re.fullmatch(r"/api/v1/spaces/([^/]+)/member-auth-keys", path)
        if method == "GET" and m:
            session = self._require_auth()
            if session is None:
                return
            space_id = m.group(1)
            if space_role_for_session(session, space_id) != "admin":
                self._send_json(403, {"ok": False, "error": "forbidden"})
                return
            keys = [v for (sid, _), v in member_auth_keys.items() if sid == space_id]
            self._send_json(200, {"ok": True, "auth_keys": keys})
            return

        # member auth keys - issue
        if method == "POST" and m:
            session = self._require_auth()
            if session is None:
                return
            space_id = m.group(1)
            if space_role_for_session(session, space_id) != "admin":
                self._send_json(403, {"ok": False, "error": "forbidden"})
                return
            body = self._read_json()
            key_id = next_member_key_id
            next_member_key_id += 1
            rec = {
                "id": key_id,
                "space_id": space_id,
                "space_name": spaces.get(space_id, {}).get("name", "unknown"),
                "issued_by_user_id": 1,
                "issued_by_email": session["email"],
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
            key, error = issue_test_auth_key(rec["invitee_email"], "member", space_id, (space_id, key_id))
            if error is not None:
                self._send_json(400, {"ok": False, "error": error})
                return
            member_auth_keys[(space_id, key_id)] = rec
            member_auth_key_values[(space_id, key_id)] = key
            self._send_json(200, {
                "ok": True,
                "auth_key": rec,
                "key": key,
            })
            return

        # member auth keys - revoke
        m = re.fullmatch(r"/api/v1/spaces/([^/]+)/member-auth-keys/(\d+)", path)
        if method == "DELETE" and m:
            session = self._require_auth()
            if session is None:
                return
            space_id = m.group(1)
            if space_role_for_session(session, space_id) != "admin":
                self._send_json(403, {"ok": False, "error": "forbidden"})
                return
            key_id = int(m.group(2))
            rec = member_auth_keys.get((space_id, key_id))
            if rec:
                rec["revoked_at"] = "2026-01-02T00:00:00Z"
                auth_key = member_auth_key_values.get((space_id, key_id))
                if auth_key in issued_auth_keys:
                    issued_auth_keys[auth_key]["revoked_at"] = rec["revoked_at"]
            self._send_json(200, {"ok": True})
            return

        # --- SSH keys ---
        if method == "GET" and path == "/api/v1/ssh/keys":
            session = self._require_auth()
            if session is None:
                return
            keys = [rec for fingerprint, rec in ssh_keys.items() if ssh_key_owners.get(fingerprint) == session["email"]]
            self._send_json(200, {"ok": True, "keys": keys})
            return

        if method == "POST" and path == "/api/v1/ssh/keys":
            session = self._require_auth()
            if session is None:
                return
            body = self._read_json()
            key_id = next_ssh_key_id
            next_ssh_key_id += 1
            fingerprint = "SHA256:fake" + str(key_id)
            rec = {
                "id": key_id,
                "user_id": 1,
                "user_email": session["email"],
                "name": body.get("name", ""),
                "public_key": body.get("public_key", ""),
                "fingerprint": fingerprint,
                "created_at": "2026-01-01T00:00:00Z",
            }
            ssh_keys[fingerprint] = rec
            ssh_key_owners[fingerprint] = session["email"]
            self._send_json(200, {"ok": True, "key": rec})
            return

        m = re.fullmatch(r"/api/v1/ssh/keys/(.+)", path)
        if method == "DELETE" and m:
            session = self._require_auth()
            if session is None:
                return
            fingerprint = m.group(1)
            if ssh_key_owners.get(fingerprint) != session["email"]:
                self._send_json(404, {"ok": False, "error": "not found"})
                return
            ssh_keys.pop(fingerprint, None)
            ssh_key_owners.pop(fingerprint, None)
            self._send_json(200, {"ok": True})
            return

        # --- SSH issue-cert ---
        if method == "POST" and path == "/api/v1/ssh/issue-cert":
            session = self._require_auth()
            if session is None:
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

        if method == "GET" and path == "/api/v1/ssh/known-hosts":
            session = self._require_auth()
            if session is None:
                return
            host = self.query.get("host", ["spaces.borca.ai"])[0]
            port = int(self.query.get("port", ["22"])[0])
            if port == 22:
                host_pattern = host
            else:
                host_pattern = f"[{host}]:{port}"
            public_key = "ssh-ed25519 AAAA_FAKE_HOST_KEY"
            self._send_json(200, {
                "ok": True,
                "host": host,
                "port": port,
                "public_key": public_key,
                "known_hosts_line": f"{host_pattern} {public_key}",
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
