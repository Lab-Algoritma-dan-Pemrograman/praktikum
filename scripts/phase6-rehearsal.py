#!/usr/bin/env python3
"""Run local SYNTHETIC migration rehearsal. No credentials or remote DB accepted.
Run with python -I scripts/phase6-rehearsal.py. All resources belong to a new
empty directory. Failure logs and backup artifacts are retained; cluster data
is removed only after its own pg_ctl confirms shutdown.
"""
import argparse
import hashlib
import json
import os
from pathlib import Path
import shutil
import socket
import subprocess
import tempfile
import time
import uuid

ROOT = Path(__file__).resolve().parents[1]


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--evidence-parent", default=tempfile.gettempdir())
    args = parser.parse_args()
    out = Path(tempfile.mkdtemp(prefix="algohub-phase6-local-", dir=args.evidence_parent)).resolve()
    data = out / "pgdata"
    run_id = "phase6-" + uuid.uuid4().hex[:12]
    env = {k: v for k, v in os.environ.items() if not k.startswith(("PG", "ALGOHUB_", "SUPABASE_"))}
    env["PGPASSFILE"] = os.devnull
    env["PGCONNECT_TIMEOUT"] = "5"
    env["PGOPTIONS"] = "-c statement_timeout=30000 -c lock_timeout=5000 -c idle_in_transaction_session_timeout=45000 -c timezone=UTC"
    tools = {name: shutil.which(name) for name in ("go", "initdb", "pg_ctl", "psql", "createdb", "pg_dump", "pg_restore")}
    summary = {"run_id": run_id, "mode": "local-synthetic-only", "checks": [], "commands": [], "cleanup": {}, "status": "running"}
    started = False
    owned = False
    counter = 0
    binary = out / ("rehearsal.exe" if os.name == "nt" else "rehearsal")
    targets = {}
    print(json.dumps({"evidence_dir": str(out), "status": "started"}), flush=True)

    def source_checksums():
        files = list((ROOT / "backend").rglob("*.go")) + list((ROOT / "backend" / "migrations").glob("*.sql"))
        files += [ROOT / "backend" / "go.mod", ROOT / "backend" / "go.sum", ROOT / "go.work", Path(__file__).resolve()]
        return {p.relative_to(ROOT).as_posix(): hashlib.sha256(p.read_bytes()).hexdigest() for p in sorted(files)}

    def write(name, value):
        path = out / name
        with path.open("x", encoding="utf8") as f:
            json.dump(value, f, indent=2)
            f.write("\n")
        return path

    def command(label, argv, *, timeout=120, expected=0, extra_env=None):
        nonlocal counter
        counter += 1
        t = time.monotonic()
        # Logs contain only this synthetic environment. Never invoke against a
        # credential file or a legacy source path.
        log = f"{counter:03d}-{label}.log"
        # A Windows daemon can inherit a PIPE even after pg_ctl exits. Using a
        # real log file keeps wait/timeout bounded by the direct child process.
        try:
            with (out / log).open("xb") as logfile:
                completed = subprocess.run([str(x) for x in argv], cwd=ROOT, env=extra_env or env,
                                           stdin=subprocess.DEVNULL, stdout=logfile, stderr=subprocess.STDOUT,
                                           timeout=timeout, check=False)
        except subprocess.TimeoutExpired as exc:
            summary["commands"].append({"label": label, "timeout_seconds": timeout, "log": log})
            raise RuntimeError(f"{label}: deadline exceeded; see {log}") from exc
        text = (out / log).read_text(encoding="utf8", errors="replace")
        summary["commands"].append({"label": label, "exit": completed.returncode,
                                    "expected": expected, "seconds": round(time.monotonic() - t, 3), "log": log})
        if (expected == "nonzero" and completed.returncode == 0) or (expected != "nonzero" and completed.returncode != expected):
            raise RuntimeError(f"{label}: unexpected exit {completed.returncode}; see {log}")
        return text

    def check(name, condition, **details):
        summary["checks"].append({"name": name, "passed": bool(condition), **details})
        if not condition:
            raise RuntimeError("assertion failed: " + name)

    def pg_args(db):
        return ["--host=127.0.0.1", f"--port={port}", "--username=phase6", "--no-password", f"--dbname={db}"]

    def sql(label, db, text):
        return command(label, [tools["psql"], "--no-psqlrc", "--tuples-only", "--no-align", "--set=ON_ERROR_STOP=1", *pg_args(db), "--command", text], timeout=40).strip()

    def database(label):
        name = f"p6_{run_id.split('-')[1]}_{label}_test"
        command("create-" + label, [tools["createdb"], "--host=127.0.0.1", f"--port={port}", "--username=phase6", "--no-password", name])
        sql("own-" + label, name, f"comment on database {name} is 'algohub-phase6:{run_id}'")
        target = {"run_id": run_id, "database": name, "data_directory": data.as_posix(), "system_identifier": system_id, "port": port}
        targets[label] = write("target-" + label + ".json", target)
        return name

    def cli(label, action, db_label=None, *, extra=(), expected=0):
        result_path = out / (label + ".json")
        argv = [binary, action, "--out", result_path]
        if db_label:
            argv += ["--target", targets[db_label]]
            if action != "init":
                argv += ["--baseline", out / "baseline.json"]
        argv += list(extra)
        command(label, argv, expected=expected, timeout=190)
        if expected != 0:
            check(label + "-no-success-artifact", not result_path.exists())
            return None
        return json.loads(result_path.read_text(encoding="utf8"))

    def dump(label, db):
        path = out / (label + ".dump")
        command(label, [tools["pg_dump"], *pg_args(db), "--format=custom", "--no-owner", "--lock-wait-timeout=5000", "--file", path], timeout=90)
        check(label + "-nonempty", path.stat().st_size > 0, bytes=path.stat().st_size,
              sha256=hashlib.sha256(path.read_bytes()).hexdigest())
        return path

    def restore(label, backup):
        db = database(label)
        command("restore-" + label, [tools["pg_restore"], *pg_args(db), "--no-owner", "--exit-on-error", "--single-transaction", backup], timeout=120)
        return db

    try:
        if not all(tools.values()):
            raise RuntimeError("missing native tool: " + ", ".join(k for k, v in tools.items() if not v))
        write("ownership.json", {"run_id": run_id, "data_directory": data.as_posix(), "purpose": "new disposable synthetic cluster"})
        inputs = source_checksums()
        write("source-checksums.json", inputs)
        owned = True
        command("build-tool", [tools["go"], "build", "-o", binary, "./backend/cmd/rehearsal"], timeout=180)
        command("unit-tests", [tools["go"], "test", "-count=1", "./backend/internal/migrationrehearsal"], timeout=180)
        command("fixture", [binary, "fixture", "--out", out / "fixture"])
        manifest = out / "fixture" / "manifest.json"
        plan1 = cli("plan-first", "transform", extra=["--manifest", manifest])
        plan2 = cli("plan-repeat", "transform", extra=["--manifest", manifest])
        check("deterministic-transform", plan1 == plan2, sha256=plan1["digest"])
        batch = plan1["batch_id"]
        apply_args = ["--plan", out / "plan-first.json", "--confirm-batch", batch, "--baseline", out / "baseline.json"]
        with socket.socket() as sock:
            sock.bind(("127.0.0.1", 0))
            port = sock.getsockname()[1]
        command("initdb", [tools["initdb"], "--pgdata", data, "--username=phase6", "--auth-local=trust", "--auth-host=trust", "--encoding=UTF8", "--locale=C"], timeout=90)
        command("start-postgres", [tools["pg_ctl"], "-D", data, "-l", out / "postgres.log", "-w", "-t", "30", "-o",
                                   f"-h 127.0.0.1 -p {port} -c max_connections=24 -c timezone=UTC", "start"], timeout=45)
        started = True
        system_id = sql("cluster-identity", "postgres", "select system_identifier::text from pg_control_system()")
        main_db = database("main")
        baseline = cli("baseline", "init", "main")
        before = cli("read-only-preflight", "preflight", "main")
        check("preflight-read-only", before["snapshot"] == baseline["snapshot"])
        baseline_dump = dump("baseline-backup", main_db)
        restored_db = restore("baseline_restore", baseline_dump)
        metadata_sql = """select jsonb_agg(v order by v::text) from (
          select jsonb_build_object('kind','constraint','table',t.relname,'name',c.conname,'type',c.contype,'validated',c.convalidated,'definition',pg_get_constraintdef(c.oid)) v from pg_constraint c join pg_class t on t.oid=c.conrelid join pg_namespace n on n.oid=t.relnamespace where n.nspname in ('public','auth')
          union all select jsonb_build_object('kind','acl','schema',n.nspname,'table',t.relname,'acl',t.relacl,'effective_acl',array(select x::text from unnest(coalesce(t.relacl,acldefault(case when t.relkind='S' then 's'::"char" else 'r'::"char" end,t.relowner))) x order by x::text)) v from pg_class t join pg_namespace n on n.oid=t.relnamespace where n.nspname in ('public','auth') and t.relkind in ('r','p','S')) objects"""
        original_metadata = json.loads(sql("baseline-schema-metadata", main_db, metadata_sql))
        restored_metadata = json.loads(sql("restored-schema-metadata", restored_db, metadata_sql))
        write("restore-schema-diff.json", {"only_before": [x for x in original_metadata if x not in restored_metadata], "only_after": [x for x in restored_metadata if x not in original_metadata]})
        restored = cli("baseline-restored", "preflight", "baseline_restore")
        check("backup-restore-baseline-exact", restored["snapshot"] == baseline["snapshot"])
        first = cli("apply-first", "apply", "main", extra=apply_args)
        cli("reconcile-first", "reconcile", "main", extra=["--plan", out / "plan-first.json", "--receipt", out / "apply-first.json"])
        repeated = cli("apply-repeat", "apply", "main", extra=apply_args)
        check("repeat-import-noop", repeated["counts"].get("imported", 0) == 0 and repeated["counts"].get("unchanged", 0) == first["counts"].get("imported", 0)
              and repeated["before"] == repeated["after"] == first["after"])
        cli("reconcile-repeat", "reconcile", "main", extra=["--plan", out / "plan-first.json", "--receipt", out / "apply-repeat.json"])
        populated_dump = dump("populated-backup", main_db)
        # Fetch one synthetic record to inject a late conflict on a separate copy.
        item_json = json.loads(sql("synthetic-conflict-record", main_db, "select to_jsonb(i) from academic_inventory_items i"))
        acceptance_db = restore("acceptance", populated_dump)
        acceptance = cli("populated-restored", "preflight", "acceptance")
        check("backup-restore-populated-exact", acceptance["snapshot"] == first["after"])
        test_env = dict(env)
        test_env["ALGOHUB_PHASE6_IMPORTED_FLOWS"] = "true"
        test_env["ALGOHUB_PHASE6_TARGET_FILE"] = str(targets["acceptance"])
        test_env["ALGOHUB_TEST_DATABASE_URL"] = f"postgres://phase6@127.0.0.1:{port}/{acceptance_db}?sslmode=disable"
        flow_output = command("imported-flows", [tools["go"], "test", "-json", "-count=1", "-run", "^TestMigrationRehearsalImportedFlows$", "./backend/internal/repository"], timeout=180, extra_env=test_env)
        events = [json.loads(line) for line in flow_output.splitlines() if line.startswith("{")]
        passes = [e["Test"] for e in events if e.get("Action") == "pass" and "Test" in e]
        skips = [e["Test"] for e in events if e.get("Action") == "skip" and "Test" in e]
        check("imported-flows-executed", "TestMigrationRehearsalImportedFlows" in passes and not skips, passed_tests=passes, skipped=skips,
              limitation="local HTTP/services with fake Auth provider, not real Supabase or browser SSO")
        # An unchanged receipt cannot undo the first import.
        noop = cli("rollback-noop-receipt", "rollback", "main", extra=["--receipt", out / "apply-repeat.json", "--confirm-batch", batch])
        check("noop-receipt-does-not-delete", noop["snapshot"] == first["after"])
        rolled = cli("rollback-first", "rollback", "main", extra=["--receipt", out / "apply-first.json", "--confirm-batch", batch])
        check("rollback-baseline-rows", rolled["snapshot"]["tables"] == baseline["snapshot"]["tables"] and rolled["snapshot"]["schema_sha256"] == baseline["snapshot"]["schema_sha256"],
              sequence_policy="monotonic sequences intentionally not reset")
        restore("recovery", baseline_dump)
        recovered = cli("apply-after-restore", "apply", "recovery", extra=apply_args)
        check("restore-reimport-identical", recovered["after"] == first["after"])
        cli("wrong-target-receipt", "rollback", "recovery", extra=["--receipt", out / "apply-first.json", "--confirm-batch", batch], expected="nonzero")
        check("wrong-target-no-mutation", cli("wrong-target-after", "preflight", "recovery")["snapshot"] == recovered["after"])
        conflict_db = restore("conflict", baseline_dump)
        item_json["data"]["name"] = "Conflicting synthetic baseline item"
        value = json.dumps(item_json, separators=(",", ":")).replace("'", "''")
        sql("inject-late-conflict", conflict_db, "insert into public.academic_inventory_items select * from jsonb_populate_record(null::public.academic_inventory_items, '" + value + "'::jsonb)")
        conflict_before = cli("conflict-before", "preflight", "conflict")
        cli("late-conflict-apply", "apply", "conflict", extra=apply_args, expected="nonzero")
        conflict_after = cli("conflict-after", "preflight", "conflict")
        check("late-conflict-rows-atomic", conflict_before["snapshot"]["tables"] == conflict_after["snapshot"]["tables"], sequence_policy="sequence allocations may survive failed transaction")
        dependent_db = restore("dependent", baseline_dump)
        cli("dependent-apply", "apply", "dependent", extra=apply_args)
        sql("inject-unowned-dependent", dependent_db, "insert into public.audit_logs(actor_id,action,app,subject_type,subject_id,scope,detail,request_id,operation_id) select u.id,'synthetic.post-import','portal','user',u.id::text,'{}','{}','synthetic-dependent-request','synthetic-dependent-operation' from public.users u where u.email<>'phase6-baseline@example.test' order by u.id limit 1")
        dep_before = cli("dependent-before", "preflight", "dependent")
        cli("dependent-rollback-refused", "rollback", "dependent", extra=["--receipt", out / "dependent-apply.json", "--confirm-batch", batch], expected="nonzero")
        check("dependent-refusal-atomic", cli("dependent-after", "preflight", "dependent")["snapshot"] == dep_before["snapshot"])
        mutated_db = restore("usermutation", baseline_dump)
        cli("usermutation-apply", "apply", "usermutation", extra=apply_args)
        sql("mutate-user-version", mutated_db, "update public.users set access_version=access_version+1 where email<>'phase6-baseline@example.test'")
        mutation_before = cli("usermutation-before", "preflight", "usermutation")
        cli("usermutation-rollback-refused", "rollback", "usermutation", extra=["--receipt", out / "usermutation-apply.json", "--confirm-batch", batch], expected="nonzero")
        check("user-mutation-refusal-atomic", cli("usermutation-after", "preflight", "usermutation")["snapshot"] == mutation_before["snapshot"])
        # Existing schema drift must never trigger reapplication.
        sql("inject-rls-drift", mutated_db, "alter table public.learning_levels disable row level security")
        cli("drift-preflight-refused", "preflight", "usermutation", expected="nonzero")
        regression_db = database("regression")
        regression_env = dict(env)
        regression_env["ALGOHUB_TEST_DATABASE_URL"] = f"postgres://phase6@127.0.0.1:{port}/{regression_db}?sslmode=disable"
        regression_output = command("backend-regression", [tools["go"], "test", "-json", "-count=1", "./backend/..."], timeout=240, extra_env=regression_env)
        regression_events = [json.loads(line) for line in regression_output.splitlines() if line.startswith("{")]
        regression_skips = [e["Test"] for e in regression_events if e.get("Action") == "skip" and "Test" in e]
        regression_passes = [e["Test"] for e in regression_events if e.get("Action") == "pass" and "Test" in e]
        check("backend-regression-postgres", regression_passes and all(name == "TestMigrationRehearsalImportedFlows" for name in regression_skips),
              passing_test_events=len(regression_passes), skipped=regression_skips,
              skip_reason="imported-data test ran separately against restored acceptance DB")
        command("backend-vet", [tools["go"], "vet", "./backend/..."], timeout=180)
        command("backend-build", [tools["go"], "build", "./backend/..."], timeout=180)
        check("source-unchanged-through-verification", inputs == source_checksums(), files=len(inputs))
        summary["status"] = "passed"
        summary["source_counts"] = plan1["counts"]
        summary["apply_counts"] = first["counts"]
        summary["plan_digest"] = plan1["digest"]
        summary["scope_limitations"] = ["synthetic contract only; no real legacy adapter or PII authorization", "no remote write or imported-data browser acceptance", "file-copy plan only", "row rollback does not rewind sequences; exact restore tested separately", "receipt-loss recovery uses isolated baseline restore; no process-crash durability claim"]
    except Exception as exc:
        summary["status"] = "failed"
        summary["failure"] = str(exc)
    finally:
        # Even a timed-out pg_ctl start may have started THIS cluster. Consult
        # only its newly created data directory, never a global process kill.
        if owned and (data / "PG_VERSION").exists():
            try:
                status = subprocess.run([tools["pg_ctl"], "-D", str(data), "status"], cwd=ROOT, env=env,
                                        stdout=subprocess.PIPE, stderr=subprocess.STDOUT, timeout=10, check=False)
                if status.returncode == 0:
                    command("stop-owned-postgres", [tools["pg_ctl"], "-D", data, "-w", "-t", "30", "-m", "fast", "stop"], timeout=40)
                status = subprocess.run([tools["pg_ctl"], "-D", str(data), "status"], cwd=ROOT, env=env,
                                        stdout=subprocess.PIPE, stderr=subprocess.STDOUT, timeout=10, check=False)
                if status.returncode != 3:
                    raise RuntimeError("owned cluster shutdown not confirmed; preserving pgdata")
                shutil.rmtree(data)
                summary["cleanup"] = {"cluster_stopped": True, "data_directory_removed": not data.exists(), "temporary_databases_removed_with_cluster": True,
                                      "containers_created": 0, "networks_created": 0, "backups_and_reports_retained": True}
            except Exception as exc:
                summary["cleanup"] = {"failed": str(exc), "data_preserved": data.exists()}
                summary["status"] = "failed"
        else:
            summary["cleanup"] = {"cluster_started": False, "data_directory_present": data.exists()}
        (out / "summary.json").write_text(json.dumps(summary, indent=2) + "\n", encoding="utf8")
        print(json.dumps({"evidence_dir": str(out), "status": summary["status"], "checks": len(summary["checks"]), "cleanup": summary["cleanup"], "failure": summary.get("failure")}))
    return 0 if summary["status"] == "passed" else 1


if __name__ == "__main__":
    raise SystemExit(main())
