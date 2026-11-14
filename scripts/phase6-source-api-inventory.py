#!/usr/bin/env python3
"""Read legacy Supabase OpenAPI metadata and HEAD row counts; never rows/secrets."""
import argparse
import hashlib
import json
from pathlib import Path
import re
import urllib.error
import urllib.parse
import urllib.request


class NoRedirect(urllib.request.HTTPRedirectHandler):
    def redirect_request(self, req, fp, code, msg, headers, newurl):
        raise RuntimeError("redirect refused")


def config(path):
    values = {}
    for line in Path(path).read_text(encoding="utf-8-sig").splitlines():
        match = re.match(r"^\s*(?:export\s+)?([A-Za-z_][A-Za-z0-9_]*)\s*=\s*(.*)$", line)
        if not match:
            continue
        key, value = match.groups()
        if key not in {"SUPABASE_URL", "VITE_SUPABASE_URL", "SUPABASE_SERVICE_ROLE_KEY", "SUPABASE_SERVICE_KEY"}:
            continue
        if value.startswith('"'):
            value = json.loads(value)
        elif len(value) >= 2 and value[0] == value[-1] == "'":
            value = value[1:-1]
        values[key] = value
    return values


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--env-file", required=True)
    parser.add_argument("--out", required=True)
    parser.add_argument("--label", required=True)
    args = parser.parse_args()
    result = {"label": args.label, "mode": "read-only-schema-and-head-counts", "status": "failed", "tables": []}
    try:
        values = config(args.env_file)
        url = urllib.parse.urlsplit(values.get("VITE_SUPABASE_URL") or values.get("SUPABASE_URL", ""))
        if url.scheme != "https" or not (url.hostname or "").endswith(".supabase.co") or url.username or url.password or url.port:
            raise ValueError("invalid configured source origin")
        key = values.get("SUPABASE_SERVICE_ROLE_KEY") or values.get("SUPABASE_SERVICE_KEY")
        if not key:
            raise ValueError("source service credential unavailable")
        origin = "https://" + url.hostname
        result["project_fingerprint"] = hashlib.sha256(url.hostname.encode()).hexdigest()
        client = urllib.request.build_opener(NoRedirect())
        headers = {"apikey": key, "Authorization": "Bearer " + key, "Accept": "application/openapi+json"}
        req = urllib.request.Request(origin + "/rest/v1/", headers=headers)
        with client.open(req, timeout=12) as response:
            body = response.read((4 << 20) + 1)
            if len(body) > 4 << 20:
                raise ValueError("metadata size limit")
            schema = json.loads(body)
        definitions = schema.get("definitions", {})
        if len(definitions) > 150:
            raise ValueError("table inventory exceeds bound")
        for name, definition in sorted(definitions.items()):
            if not re.fullmatch(r"[A-Za-z_][A-Za-z0-9_]*", name):
                raise ValueError("unsupported relation identifier")
            columns = [{"name": k, "type": v.get("format", v.get("type"))} for k, v in definition.get("properties", {}).items()]
            table = {"name": name, "columns": columns}
            req = urllib.request.Request(origin + "/rest/v1/" + urllib.parse.quote(name) + "?select=*", method="HEAD", headers={**headers, "Accept": "application/json", "Prefer": "count=exact", "Range": "0-0"})
            try:
                with client.open(req, timeout=8) as response:
                    count = response.headers.get("Content-Range", "").rsplit("/", 1)[-1]
                    table["rows"] = int(count) if count.isdigit() else None
                    table["http_status"] = response.status
            except urllib.error.HTTPError as exc:
                table["http_status"] = exc.code
                table["rows"] = None
            if name in {"users", "attendance_logs", "schedules", "inventory_items", "inventory_rentals", "financial_records"}:
                names = {c["name"] for c in columns}
                field = next((k for k in ("updated_at", "check_in_time", "created_at") if k in names), None)
                if field:
                    query = urllib.parse.urlencode({"select": field, "order": field + ".desc.nullslast", "limit": 1})
                    req = urllib.request.Request(origin + "/rest/v1/" + urllib.parse.quote(name) + "?" + query, headers={**headers, "Accept": "application/json"})
                    with client.open(req, timeout=8) as response:
                        dates = json.loads(response.read(4096))
                    table["latest_timestamp"] = {field: dates[0].get(field) if dates else None}
            result["tables"].append(table)
        result["status"] = "passed"
    except urllib.error.HTTPError as exc:
        result["failure"] = "source API HTTP " + str(exc.code)
    except Exception as exc:
        # Never stringify exception objects that might contain URLs or headers.
        result["failure"] = "source inventory failed: " + type(exc).__name__
    with Path(args.out).open("x", encoding="utf8") as f:
        json.dump(result, f, indent=2)
        f.write("\n")
    print(json.dumps({"label": args.label, "status": result["status"], "tables": len(result["tables"]), "failure": result.get("failure")}))
    return 0 if result["status"] == "passed" else 1


if __name__ == "__main__":
    raise SystemExit(main())
