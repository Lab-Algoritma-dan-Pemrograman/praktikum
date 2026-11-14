"""Unggah cadangan storage legacy ke project Supabase tujuan, lalu verifikasi sha256.

Dipakai untuk staging maupun produksi:

    python scripts/upload-storage.py --env .env.staging.local
    python scripts/upload-storage.py --env .env.production.local
    python scripts/upload-storage.py --env .env.production.local --dry-run

Baket dibuat dengan visibilitas dan limit yang sama seperti project legacy.
Setiap objek diunggah dengan upsert, lalu diunduh ulang dan dibandingkan sha256,
sehingga skrip aman dijalankan berulang.
"""

from __future__ import annotations

import argparse
import hashlib
import json
import mimetypes
import os
import sys
import time
import urllib.error
import urllib.parse
import urllib.request

DEFAULT_BACKUP = r"E:/tmp/legacy-storage-backup-20261115"
BUCKETS = {
    "public-assets": {"public": True, "file_size_limit": None},
    "laporan-besar": {"public": False, "file_size_limit": 10485760},
}


def env(path: str, key: str) -> str:
    with open(path, encoding="utf-8", errors="ignore") as handle:
        for line in handle:
            line = line.strip()
            if line.startswith(f"{key}="):
                return line.split("=", 1)[1].strip().strip("'\"")
    raise SystemExit(f"{path}: {key} tidak ditemukan")


def request(url: str, key: str, method: str = "GET", data: bytes | None = None,
            headers: dict[str, str] | None = None, timeout: int = 120):
    all_headers = {"Authorization": f"Bearer {key}", "apikey": key}
    all_headers.update(headers or {})
    req = urllib.request.Request(url, data=data, method=method, headers=all_headers)
    try:
        with urllib.request.urlopen(req, timeout=timeout) as response:
            return response.status, response.read()
    except urllib.error.HTTPError as error:
        return error.code, error.read()


def object_url(base: str, bucket: str, path: str) -> str:
    return f"{base}/storage/v1/object/{bucket}/{urllib.parse.quote(path)}"


def main() -> int:
    parser = argparse.ArgumentParser()
    parser.add_argument("--env", required=True, help="file env berisi SUPABASE_URL + SUPABASE_SERVICE_ROLE_KEY")
    parser.add_argument("--backup", default=DEFAULT_BACKUP, help="folder cadangan legacy")
    parser.add_argument("--dry-run", action="store_true", help="hanya tampilkan rencana")
    args = parser.parse_args()

    base = env(args.env, "SUPABASE_URL").rstrip("/")
    key = env(args.env, "SUPABASE_SERVICE_ROLE_KEY")
    manifest = json.load(open(os.path.join(args.backup, "MANIFEST.json"), encoding="utf-8"))

    print(f"target {base} — {len(manifest)} objek, {sum(i['size'] for i in manifest)} byte")
    if args.dry_run:
        for item in manifest:
            print(f"  {item['bucket']}/{item['path']} ({item['size']} byte)")
        return 0

    status, body = request(f"{base}/storage/v1/bucket", key)
    if status != 200:
        raise SystemExit(f"gagal membaca baket: {status} {body[:200]!r}")
    existing = {bucket["name"]: bucket for bucket in json.loads(body or b"[]")}

    for name, spec in BUCKETS.items():
        if name in existing:
            print(f"baket {name}: sudah ada (public={existing[name].get('public')})")
            continue
        payload = json.dumps({"id": name, "name": name, "public": spec["public"],
                              "file_size_limit": spec["file_size_limit"]}).encode()
        status, body = request(f"{base}/storage/v1/bucket", key, "POST", payload,
                               {"Content-Type": "application/json"})
        print(f"baket {name}: dibuat -> {status}")
        if status not in (200, 201):
            raise SystemExit(f"gagal membuat baket {name}: {body[:200]!r}")

    failures: list[str] = []
    uploaded = 0
    for item in manifest:
        local = os.path.join(args.backup, item["bucket"], item["path"])
        if not os.path.exists(local):
            failures.append(f"file lokal hilang: {local}")
            continue
        data = open(local, "rb").read()
        if len(data) != item["size"] or hashlib.sha256(data).hexdigest() != item["sha256"]:
            failures.append(f"checksum lokal berbeda: {item['bucket']}/{item['path']}")
            continue
        mime = item.get("mime") or mimetypes.guess_type(item["path"])[0] or "application/octet-stream"
        for attempt in range(3):
            status, body = request(object_url(base, item["bucket"], item["path"]), key, "POST", data,
                                   {"Content-Type": mime, "x-upsert": "true"})
            if status in (200, 201):
                uploaded += 1
                break
            time.sleep(1 + attempt)
        else:
            failures.append(f"unggah gagal: {item['bucket']}/{item['path']} -> {status} {body[:160]!r}")
    print(f"terunggah {uploaded}/{len(manifest)}")

    verified = 0
    for item in manifest:
        status, body = request(object_url(base, item["bucket"], item["path"]), key)
        if status != 200:
            failures.append(f"unduh gagal: {item['bucket']}/{item['path']} -> {status}")
        elif hashlib.sha256(body).hexdigest() != item["sha256"]:
            failures.append(f"checksum remote berbeda: {item['bucket']}/{item['path']}")
        else:
            verified += 1
    print(f"terverifikasi {verified}/{len(manifest)}")

    if failures:
        print("KEGAGALAN:")
        for failure in failures[:20]:
            print(" -", failure)
        return 1
    print("migrasi storage ok")
    return 0


if __name__ == "__main__":
    sys.exit(main())
