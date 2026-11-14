#!/usr/bin/env python3
"""Reconcile row-count metrics between legacy prod and v2 staging DBs.

Dry-run import validation per plan/database-migration.md sections 4-5
(row count / identity validation). Read-only: every query is a SELECT
count, run through `psql -At -F tab`, matching how the audit queries in
scripts/prod-audit/ were executed. psycopg is NOT required.

Usage (no credentials embedded; pass connection URLs as CLI args):

    python scripts/prod-audit/reconcile-import.py \
        --source "$LEGACY_DATABASE_URL" \
        --target "$TARGET_DATABASE_URL"

Exit codes:
    0  all checklist metrics match (target >= source, additive import)
    1  one or more metrics differ or error
    2  usage error (missing/invalid CLI args)

Output contains no connection strings; passwords are masked in any
error text.
"""

import argparse
import re
import subprocess
import sys

# (metric, source SQL, target SQL, note)  source=legacy prod, target=v2.
# Target table names verified against backend/migrations/0002-0007
# (practicum_* dynamic DO-block array: sessions, courses, questions,
# activations, attempts, grading_jobs, operations, runner_leases).
METRICS = [
    (
        "users_total",
        "SELECT count(*) FROM public.users",
        "SELECT count(*) FROM public.users",
        "source roles: asisten/koordinator/mahasiswa",
    ),
    (
        "progress_total",
        "SELECT count(*) FROM public.student_progress",
        "SELECT count(*) FROM public.learning_progress",
        "",
    ),
    (
        "attendance_total",
        "SELECT count(*) FROM public.attendance_logs",
        "SELECT count(*) FROM public.academic_attendance_records",
        "",
    ),
    (
        "schedule_total",
        "SELECT count(*) FROM public.schedules",
        "SELECT count(*) FROM public.academic_schedules",
        "source schedule_assignments not included",
    ),
    (
        "attempts_total",
        "SELECT count(*) FROM public.pengerjaan_course",
        "SELECT count(*) FROM public.practicum_attempts",
        "",
    ),
    (
        "answers_total",
        "SELECT count(*) FROM public.jawaban_mahasiswa",
        "SELECT count(*) FROM public.practicum_questions",
        "approximate pairing: legacy answers vs v2 questions",
    ),
    (
        "storage_public_assets",
        "SELECT count(*) FROM storage.objects WHERE bucket_id='public-assets'",
        "SELECT count(*) FROM storage.objects WHERE bucket_id='public-assets'",
        "",
    ),
    (
        "storage_laporan_besar",
        "SELECT count(*) FROM storage.objects WHERE bucket_id='laporan-besar'",
        "SELECT count(*) FROM storage.objects WHERE bucket_id='laporan-besar'",
        "",
    ),
]

_PASSWORD_RE = re.compile(r"(?::)([^@/:]+)(?=@)")


def mask_url(text):
    """Mask the password component of any URL embedded in text."""
    return _PASSWORD_RE.sub(":***@", text)


def run_psql(url, sql):
    """Run one SQL statement via psql, return list of single-column rows.

    Read-only contract: callers only pass SELECT count(*) statements.
    Raises RuntimeError with masked message on failure.
    """
    try:
        proc = subprocess.run(
            ["psql", url, "-At", "-F", "\t", "-c", sql],
            capture_output=True,
            text=True,
            timeout=60,
        )
    except FileNotFoundError:
        raise RuntimeError("psql executable not found on PATH")
    except subprocess.TimeoutExpired:
        raise RuntimeError("psql timed out after 60s")
    if proc.returncode != 0:
        raise RuntimeError(
            "psql failed (exit %d): %s"
            % (proc.returncode, mask_url(proc.stderr.strip()))
        )
    return [line.strip() for line in proc.stdout.splitlines() if line.strip()]


def main():
    parser = argparse.ArgumentParser(
        description=(
            "Reconcile legacy prod vs v2 staging row counts (read-only). "
            "Connection URLs are required CLI args; never embed credentials."
        )
    )
    parser.add_argument("--source", required=True, help="legacy prod database URL")
    parser.add_argument("--target", required=True, help="v2 staging/target database URL")
    args = parser.parse_args()

    print("metric\tsource\ttarget\tstatus")
    diffs = []
    for name, src_sql, tgt_sql, note in METRICS:
        try:
            src_rows = run_psql(args.source, src_sql)
            tgt_rows = run_psql(args.target, tgt_sql)
            src_val = src_rows[0] if src_rows else "(no rows)"
            tgt_val = tgt_rows[0] if tgt_rows else "(no rows)"
            status = "MATCH" if src_val == tgt_val else "DIFF"
        except RuntimeError as exc:
            src_val = "ERROR"
            tgt_val = "ERROR"
            status = "ERROR: %s" % exc
        line = "%s\t%s\t%s\t%s" % (name, src_val, tgt_val, status)
        if note:
            line += " (%s)" % note
        print(line)
        if status != "MATCH":
            diffs.append(name)

    if diffs:
        print("diff summary: %d metric(s) not matching: %s" % (len(diffs), ", ".join(diffs)))
        return 1
    print("all %d checklist metrics match" % len(METRICS))
    return 0


if __name__ == "__main__":
    sys.exit(main())
