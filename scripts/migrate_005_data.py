#!/usr/bin/env python3
"""Pindah data elearning -> praktikum DB (satu DB untuk 3 subdomain).

Sumber : backups/elearning_data/*.json (dump read-only, 572 baris)
Tujuan : praktikum DB (Supabase oidwvfcxlxfyojkyurmm) setelah 005 dijalankan

Aturan kunci:
  - Semua yang menunjuk orang pakai users.id (integer), BUKAN nim text.
  - NIM rusak dipetakan manual lewat PETA_NIM.
  - Mahasiswa elearning yang belum ada di users -> di-insert sebagai role 'mahasiswa'.
  - student_lessons TIDAK dipindah (salinan lessons minus solution -> view materi_mahasiswa).
  - activity_logs TIDAK dipindah (log lama, bukan data operasional).
  - Idempoten: semua insert pakai ON CONFLICT DO NOTHING/UPDATE.

Pakai:  python scripts/migrate_005_data.py --dsn "<postgres dsn>" [--dry-run]
"""
import argparse
import json
import os
import sys

import psycopg2
import psycopg2.extras

ROOT = os.path.dirname(os.path.dirname(os.path.abspath(__file__)))
DATA = os.path.join(ROOT, "backups", "elearning_data")

# NIM rusak di elearning -> NIM asli di praktikum.
# 'Faqod' = Ahmad Faqod Kurnia, praktikum users.id 1150, nim 202314020.
PETA_NIM = {"Faqod": "202314020"}

# role elearning -> role praktikum (kosakata 002)
PETA_ROLE = {
    "kordas": "koordinator",
    "admin": "koordinator",
    "asisten": "asisten",
    "praktikan": "mahasiswa",
    "user": "mahasiswa",
}


def muat(nama):
    with open(os.path.join(DATA, nama + ".json"), encoding="utf-8") as f:
        return json.load(f)


def nim_bersih(n):
    if n is None:
        return None
    n = str(n).strip()
    return PETA_NIM.get(n, n)


def main():
    ap = argparse.ArgumentParser()
    ap.add_argument("--dsn", required=True)
    ap.add_argument("--dry-run", action="store_true")
    args = ap.parse_args()

    conn = psycopg2.connect(args.dsn)
    conn.autocommit = False
    cur = conn.cursor()

    # ---- prasyarat: 005 sudah jalan ----
    cur.execute("""select count(*) from information_schema.tables
                   where table_schema='public'
                     and table_name in ('level','modul','materi','profil_belajar',
                                        'progres_belajar','pencapaian','pencapaian_terbuka',
                                        'game_soal','game_riwayat','game_konfigurasi',
                                        'playground_contoh')""")
    n = cur.fetchone()[0]
    if n != 11:
        sys.exit(f"ABORT: 005 belum jalan (tabel target {n}/11). Jalankan docs/sql/005_elearning.sql dulu.")

    # ---- peta nim -> users.id yang sudah ada ----
    cur.execute("select nim, id from public.users where nim is not null")
    nim2id = {r[0].strip(): r[1] for r in cur.fetchall()}
    laporan = {}

    # ---- 1) users elearning yang belum ada -> insert ----
    users_el = muat("users")
    cur.execute("select coalesce(max(id),0) from public.users")
    next_id = cur.fetchone()[0] + 1
    baru = []
    for u in users_el:
        nim = nim_bersih(u.get("nim"))
        if not nim or nim in nim2id:
            continue
        role = PETA_ROLE.get((u.get("role") or "").lower(), "mahasiswa")
        nama = (u.get("nama") or "").strip().title()
        email = u.get("email") or f"{nama.split()[0].lower()}{nim[2:]}@itpln.ac.id"
        baru.append((next_id, nim, nama, role, email))
        nim2id[nim] = next_id
        next_id += 1
    if baru:
        psycopg2.extras.execute_batch(cur, """
            insert into public.users (id, nim, nama, role, email)
            values (%s,%s,%s,%s,%s) on conflict (id) do nothing
        """, baru)
    laporan["users_baru"] = len(baru)

    # ---- 2) level / modul / materi ----
    lv = [(r["id"], r["title"], r.get("description"), r.get("access_mode") or "auto",
           bool(r.get("locked")), r.get("sort_order") or 0) for r in muat("levels")]
    psycopg2.extras.execute_batch(cur, """
        insert into public.level (id,judul,deskripsi,mode_akses,terkunci,urutan)
        values (%s,%s,%s,%s,%s,%s)
        on conflict (id) do update set judul=excluded.judul, deskripsi=excluded.deskripsi,
            mode_akses=excluded.mode_akses, terkunci=excluded.terkunci, urutan=excluded.urutan
    """, lv)
    laporan["level"] = len(lv)

    md = [(r["id"], r["level_id"], r["title"], r.get("sort_order") or 0) for r in muat("modules")]
    psycopg2.extras.execute_batch(cur, """
        insert into public.modul (id,level_id,judul,urutan) values (%s,%s,%s,%s)
        on conflict (id) do update set level_id=excluded.level_id, judul=excluded.judul, urutan=excluded.urutan
    """, md)
    laporan["modul"] = len(md)

    def js(v):
        return json.dumps(v, ensure_ascii=False) if v is not None else None

    ms = [(r["id"], r["module_id"], r["title"], r.get("explanation"), r.get("code_example"),
           r.get("initial_code"), r.get("solution"), r.get("hint"), js(r.get("quiz")),
           js(r.get("test_cases")), js(r.get("validation_rules")),
           r.get("sort_order") or 0, r.get("xp_reward") or 0) for r in muat("lessons")]
    psycopg2.extras.execute_batch(cur, """
        insert into public.materi (id,modul_id,judul,penjelasan,contoh_kode,kode_awal,solusi,
            petunjuk,kuis,kasus_uji,aturan_validasi,urutan,xp_hadiah)
        values (%s,%s,%s,%s,%s,%s,%s,%s,%s,%s,%s,%s,%s)
        on conflict (id) do update set modul_id=excluded.modul_id, judul=excluded.judul,
            penjelasan=excluded.penjelasan, contoh_kode=excluded.contoh_kode,
            kode_awal=excluded.kode_awal, solusi=excluded.solusi, petunjuk=excluded.petunjuk,
            kuis=excluded.kuis, kasus_uji=excluded.kasus_uji,
            aturan_validasi=excluded.aturan_validasi, urutan=excluded.urutan,
            xp_hadiah=excluded.xp_hadiah, updated_at=now()
    """, ms)
    laporan["materi"] = len(ms)

    # ---- 3) profil_belajar (xp/level/streak dari users elearning) ----
    pb, lewat_profil = [], []
    for u in users_el:
        nim = nim_bersih(u.get("nim"))
        uid = nim2id.get(nim)
        if not uid:
            lewat_profil.append(u.get("nim"))
            continue
        pb.append((uid, u.get("xp") or 0, u.get("level") or 1, u.get("streak") or 0,
                   u.get("study_time") or 0, u.get("last_active"),
                   js(u.get("level_access_overrides") or {}), js(u.get("assessment_access") or {})))
    psycopg2.extras.execute_batch(cur, """
        insert into public.profil_belajar (user_id,xp,level_angka,streak,waktu_belajar,
            terakhir_aktif,akses_level,akses_asesmen)
        values (%s,%s,%s,%s,%s,%s,%s,%s)
        on conflict (user_id) do update set xp=excluded.xp, level_angka=excluded.level_angka,
            streak=excluded.streak, waktu_belajar=excluded.waktu_belajar,
            terakhir_aktif=excluded.terakhir_aktif, akses_level=excluded.akses_level,
            akses_asesmen=excluded.akses_asesmen
    """, pb)
    laporan["profil_belajar"] = len(pb)
    laporan["profil_dilewati"] = lewat_profil

    # ---- 4) progres_belajar ----
    materi_ada = {m[0] for m in ms}
    pr, lewat_pr = [], []
    for r in muat("student_progress"):
        uid = nim2id.get(nim_bersih(r.get("nim")))
        if not uid or r["lesson_id"] not in materi_ada:
            lewat_pr.append((r.get("nim"), r.get("lesson_id")))
            continue
        pr.append((uid, r["lesson_id"], bool(r.get("completed")), r.get("completed_at")))
    psycopg2.extras.execute_batch(cur, """
        insert into public.progres_belajar (user_id,materi_id,selesai,selesai_pada)
        values (%s,%s,%s,%s)
        on conflict (user_id,materi_id) do update set selesai=excluded.selesai,
            selesai_pada=excluded.selesai_pada
    """, pr)
    laporan["progres_belajar"] = len(pr)
    laporan["progres_dilewati"] = lewat_pr

    # ---- 5) pencapaian ----
    ac = [(r["id"], r["title"], r.get("description"), r.get("icon"),
           r.get("requirement_type"),
           None if r.get("requirement_value") is None else str(r["requirement_value"]))
          for r in muat("achievements")]
    psycopg2.extras.execute_batch(cur, """
        insert into public.pencapaian (id,judul,deskripsi,ikon,syarat_tipe,syarat_nilai)
        values (%s,%s,%s,%s,%s,%s)
        on conflict (id) do update set judul=excluded.judul, deskripsi=excluded.deskripsi,
            ikon=excluded.ikon, syarat_tipe=excluded.syarat_tipe, syarat_nilai=excluded.syarat_nilai
    """, ac)
    laporan["pencapaian"] = len(ac)

    ach_ada = {a[0] for a in ac}
    ua, lewat_ua = [], []
    for r in muat("unlocked_achievements"):
        uid = nim2id.get(nim_bersih(r.get("nim")))
        if not uid or r["achievement_id"] not in ach_ada:
            lewat_ua.append((r.get("nim"), r.get("achievement_id")))
            continue
        ua.append((uid, r["achievement_id"], r.get("unlocked_at")))
    psycopg2.extras.execute_batch(cur, """
        insert into public.pencapaian_terbuka (user_id,pencapaian_id,dibuka_pada)
        values (%s,%s,%s) on conflict (user_id,pencapaian_id) do nothing
    """, ua)
    laporan["pencapaian_terbuka"] = len(ua)
    laporan["pencapaian_dilewati"] = lewat_ua

    # ---- 6) game ----
    gq = [(r["language"], r["difficulty"], r["title"], r["code"],
           r.get("bug_line"), r.get("explanation")) for r in muat("game_questions")]
    cur.execute("select count(*) from public.game_soal")
    if cur.fetchone()[0] == 0:
        psycopg2.extras.execute_batch(cur, """
            insert into public.game_soal (bahasa,kesulitan,judul,kode,baris_bug,penjelasan)
            values (%s,%s,%s,%s,%s,%s)
        """, gq)
        laporan["game_soal"] = len(gq)
    else:
        laporan["game_soal"] = "dilewati (sudah terisi)"

    gh, lewat_gh = [], []
    for r in muat("game_history"):
        uid = nim2id.get(nim_bersih(r.get("nim")))
        if not uid:
            lewat_gh.append(r.get("nim"))
            continue
        gh.append((uid, r["game_type"], r.get("xp_earned") or 0, r.get("played_at")))
    cur.execute("select count(*) from public.game_riwayat")
    if cur.fetchone()[0] == 0:
        psycopg2.extras.execute_batch(cur, """
            insert into public.game_riwayat (user_id,jenis_game,xp_didapat,dimainkan)
            values (%s,%s,%s,%s)
        """, gh)
        laporan["game_riwayat"] = len(gh)
    else:
        laporan["game_riwayat"] = "dilewati (sudah terisi)"

    for r in muat("game_settings"):
        cur.execute("""
            insert into public.game_konfigurasi (id,bug_hunt_aktif,bug_hunt_c_aktif,
                bug_hunt_py_aktif,batas_mingguan) values (%s,%s,%s,%s,%s)
            on conflict (id) do update set bug_hunt_aktif=excluded.bug_hunt_aktif,
                bug_hunt_c_aktif=excluded.bug_hunt_c_aktif,
                bug_hunt_py_aktif=excluded.bug_hunt_py_aktif,
                batas_mingguan=excluded.batas_mingguan
        """, (r.get("id") or "default", bool(r.get("bug_hunt_active")),
              bool(r.get("bug_hunt_c_active")), bool(r.get("bug_hunt_python_active")),
              r.get("bug_hunt_weekly_limit") or 1))
    laporan["game_konfigurasi"] = 1

    # ---- 7) playground ----
    pg = [(r["title"], r["language"], r["code"], r.get("description"),
           r.get("sort_order") or 0, r.get("created_at")) for r in muat("playground_examples")]
    cur.execute("select count(*) from public.playground_contoh")
    if cur.fetchone()[0] == 0:
        psycopg2.extras.execute_batch(cur, """
            insert into public.playground_contoh (judul,bahasa,kode,deskripsi,urutan,created_at)
            values (%s,%s,%s,%s,%s,%s)
        """, pg)
        laporan["playground_contoh"] = len(pg)
    else:
        laporan["playground_contoh"] = "dilewati (sudah terisi)"

    if args.dry_run:
        conn.rollback()
        print("DRY RUN — rollback, nol perubahan")
    else:
        conn.commit()
        print("COMMIT")

    for k, v in laporan.items():
        print(f"  {k:<22} {v}")
    cur.close()
    conn.close()


if __name__ == "__main__":
    main()
