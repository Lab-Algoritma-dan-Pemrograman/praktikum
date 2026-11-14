#!/usr/bin/env python3
"""Transform approved masked legacy snapshots into a deterministic target plan.
No credentials, raw PII, database access or writes. Unsupported semantics are
rejected with explicit weighted accounting rather than guessed.
"""
import argparse, datetime as dt, hashlib, json, math, os, uuid
from collections import Counter, defaultdict
from pathlib import Path
NS=uuid.UUID("57f77994-dcec-4fa8-a53f-43e6e4376689")
FORMAT="phase6-live-plan-v1";BATCH="phase6-masked-live-20261113-v1"
def hid(prefix,ref):return (prefix+hashlib.sha256((BATCH+"\0"+ref).encode()).hexdigest()[:24])[:128]
def token(system,table,ref):return f"{system}/{table}/{ref}"
def rfc(v,local=False):
 if not v:return None
 s=str(v).replace("Z","+00:00");d=dt.datetime.fromisoformat(s)
 if d.year<1970:return None
 if d.tzinfo is None:d=d.replace(tzinfo=dt.timezone(dt.timedelta(hours=7) if local else dt.timedelta(0)))
 return d.astimezone(dt.timezone.utc).isoformat().replace("+00:00","Z")
def add_minutes(v,n):return rfc((dt.datetime.fromisoformat(rfc(v,True).replace("Z","+00:00"))+dt.timedelta(minutes=n)).isoformat())
def marker_user(t):return {"$user":t}
def marker_id(t):return {"$id":t}
def language(lesson):
 code=" ".join(str(lesson.get(k) or "") for k in ("initial_code","code_example","solution"))
 return "python" if ("print(" in code or "def " in code or "input(" in code) and "#include" not in code else "c"
def canonical(v):return json.dumps(v,sort_keys=True,separators=(",",":"),ensure_ascii=False)
DAY_ALIASES={"senin":"Senin","selasa":"Selasa","rabu":"Rabu","kamis":"Kamis","jumat":"Jumat","jum'at":"Jumat","juma'at":"Jumat","sabtu":"Sabtu"}
def normalize_day(value):
 return DAY_ALIASES.get(str(value or "").strip().lower())
def main():
 p=argparse.ArgumentParser(description=__doc__);p.add_argument("--primary",required=True);p.add_argument("--siakad",required=True);p.add_argument("--siakad-metadata",required=True);p.add_argument("--out",required=True);a=p.parse_args()
 primary=json.loads(Path(a.primary).read_text());siakad=json.loads(Path(a.siakad).read_text());smeta=json.loads(Path(a.siakad_metadata).read_text())
 if primary.get("status")!="passed" or primary.get("masking_policy")!="local-pseudonymized-v1-no-credentials-no-learner-free-text" or siakad.get("status")!="passed":raise SystemExit("unverified masked input")
 R=primary["masked_rows"];domain=[];auth=[];users=[];assignments=[];disp=[];target_tokens=set();source_seen=set()
 def disposition(system,entity,ref,status,reason,target=None,rows=1):
  key=(system,entity,str(ref));
  if rows==1 and key in source_seen:raise ValueError("duplicate source disposition")
  if rows==1:source_seen.add(key)
  disp.append({"source_system":system,"entity":entity,"source_ref":str(ref),"disposition":status,"reason":reason,"target_token":target or "","rows":rows})
 def domainrow(tok,table,id,parent="",owner="",scope=None,status="",unique="",expires=None,data=None,id_template=""):
  if tok in target_tokens:raise ValueError("duplicate target token")
  target_tokens.add(tok);domain.append({"token":tok,"table":table,"id":id,"id_template":id_template,"parent_id":parent,"owner_token":owner,"scope":scope or {},"status":status,"unique_key":unique,"expires_at":expires,"data":data or {}})
 cutoff=rfc(primary["captured_at"])
 # Identity and access
 class_rows={r["id"]:r for r in R.get("kelas",[])};jadwal_by_class={r["kelas_id"]:r for r in R.get("jadwal",[])};users_by_class=defaultdict(list)
 for u in R["users"]:
  if u.get("kelas_id") is not None:users_by_class[u["kelas_id"]].append(u)
 class_ids={};class_shift={}
 for c in R.get("kelas",[]):
  shifts=[int(str(u.get("shift"))) for u in users_by_class[c["id"]] if str(u.get("shift")) in ("1","2")]
  shift=int(jadwal_by_class.get(c["id"],{}).get("shift") or (Counter(shifts).most_common(1)[0][0] if shifts else 0))
  ct=token("primary","kelas",c["id"])
  if shift not in (1,2):disposition("primary","kelas",c["id"],"rejected","missing_class_shift");continue
  class_ids[c["id"]]=ct;class_shift[c["id"]]=shift
  cid=hid("mig-cls-",str(c["id"]));scope={"domain":"siakad","institution":"legacy-masked","class_id":cid,"shift":shift}
  students=[marker_user(token("primary","users",u["id"])) for u in users_by_class[c["id"]] if u.get("role")=="mahasiswa"]
  data={"id":cid,"code":str(c.get("nama_kelas") or f"LEGACY-{c['id']}"),"course_code":"ALPROG","course_name":"Algoritma dan Pemrograman","period_name":"Legacy masked rehearsal","shift":shift,"student_ids":students,"assistant_ids":[],"active":True,"scope":scope,"version":1,"created_at":cutoff,"updated_at":cutoff}
  domainrow(ct,"academic_classes",cid,scope=scope,unique=data["code"],data=data);disposition("primary","kelas",c["id"],"imported","mapped_academic_class",ct)
 acceptance_chosen=False
 for u in R["users"]:
  ut=token("primary","users",u["id"]);au=str(uuid.uuid5(NS,ut));role=u.get("role");division=(u.get("divisi") or "").lower() or None
  if role not in {"koordinator","asisten","mahasiswa","peminjam"}:disposition("primary","users",u["id"],"rejected","unsupported_role");continue
  if role!="asisten":division=None
  if division not in {None,"sekretaris","bendahara","k3","labops","pendidikan","pdd"}:division=None
  created=rfc(u.get("created_at"),True) or cutoff;name=u["nama"];email=str(u["email"]).lower();nim=u.get("nim") or None
  acceptance=role=="mahasiswa" and not acceptance_chosen
  if acceptance:acceptance_chosen=True
  auth.append({"token":ut,"auth_user_id":au,"email":email,"name":name,"created_at":created,"acceptance_login":acceptance})
  users.append({"token":ut,"auth_user_id":au,"nim":nim,"email":email,"name":name,"is_active":bool(u.get("is_active",u.get("is_aktif",True))),"hidden":bool(u.get("tersembunyi",False)),"created_at":created})
  # Role assignments stay global. Academic membership is expressed by the class
  # roster, not by a narrowed assignment scope: a narrowed scope would make the
  # canonical role grants (which carry no class/shift) inactive.
  scope={}
  assignments.append({"token":ut+"/assignment","user_token":ut,"role_code":role,"division_code":division,"scope":scope,"is_active":bool(u.get("is_active",True)),"created_at":created})
  disposition("primary","users",u["id"],"imported","masked_identity_and_assignment",ut)
 # Academic schedules from authoritative primary kelas/jadwal
 for s in R.get("jadwal",[]):
  st=token("primary","jadwal",s["id"]);ct=class_ids.get(s["kelas_id"])
  if not ct:disposition("primary","jadwal",s["id"],"rejected","missing_imported_class");continue
  day=normalize_day(s.get("hari"))
  if not day:disposition("primary","jadwal",s["id"],"rejected","unsupported_schedule_day");continue
  sid=hid("mig-sch-",str(s["id"]));cid=next(x["id"] for x in domain if x["token"]==ct);shift=int(s["shift"]);scope={"domain":"siakad","institution":"legacy-masked","class_id":cid,"shift":shift}
  data={"id":sid,"class_id":cid,"class_code":str(class_rows[s["kelas_id"]].get("nama_kelas") or cid),"course_name":"Algoritma dan Pemrograman","day_of_week":day,"shift":shift,"room":str(s.get("keterangan") or "Legacy Lab"),"starts_at":str(s["jam_mulai"])[:5],"ends_at":str(s["jam_selesai"])[:5],"scope":scope,"version":1}
  domainrow(st,"academic_schedules",sid,parent=ct,scope=scope,data=data);disposition("primary","jadwal",s["id"],"imported","mapped_academic_schedule",st)
 # Learning definitions
 level_tokens={};module_tokens={};lesson_tokens={};lesson_lang={}
 lessons_by_module=defaultdict(list)
 for l in R["lessons"]:lessons_by_module[l["module_id"]].append(l)
 modules_by_level=defaultdict(list)
 for m in R["modules"]:modules_by_level[m["level_id"]].append(m)
 # Target sort orders are 1..1000. Legacy orders start at 0, so renumber
 # deterministically within each parent while preserving legacy order.
 def renumber(items):
  ordered=sorted(items,key=lambda r:(r.get("sort_order") or 0,str(r["id"])))
  return {str(r["id"]):i+1 for i,r in enumerate(ordered)}
 level_order=renumber(R["levels"]);module_order={};lesson_order={}
 for lvl,mems in modules_by_level.items():module_order.update(renumber(mems))
 for mod,less in lessons_by_module.items():lesson_order.update(renumber(less))
 for lv in R["levels"]:
  t=token("primary","levels",lv["id"]);level_tokens[lv["id"]]=t;id=hid("mig-lvl-",str(lv["id"]));scope={"domain":"elearning","institution":"legacy-masked"};total=sum(1 for m in modules_by_level[lv["id"]] for l in lessons_by_module[m["id"]] if l.get("exercise_type")=="code")
  data={"id":id,"title":lv["title"],"description":lv.get("description") or "Migrated legacy level","sort_order":level_order[str(lv["id"])],"access_mode":lv.get("access_mode") if lv.get("access_mode") in ("auto","unlocked","locked") else "auto","required_xp":0,"unlocked":not bool(lv.get("locked")),"completed_lessons":0,"total_lessons":total,"scope":scope,"version":1,"created_at":cutoff,"updated_at":cutoff}
  domainrow(t,"learning_levels",id,scope=scope,data=data);disposition("primary","levels",lv["id"],"imported","mapped_learning_level",t)
 for m in R["modules"]:
  t=token("primary","modules",m["id"]);parent=level_tokens.get(m["level_id"])
  if not parent:disposition("primary","modules",m["id"],"rejected","missing_level");continue
  module_tokens[m["id"]]=t;id=hid("mig-mod-",str(m["id"]));scope={"domain":"elearning","institution":"legacy-masked"};total=sum(l.get("exercise_type")=="code" for l in lessons_by_module[m["id"]]);data={"id":id,"level_id":marker_id(parent),"title":m["title"],"description":"Migrated legacy module","sort_order":module_order[str(m["id"])],"completed_lessons":0,"total_lessons":total,"scope":scope,"version":1}
  domainrow(t,"learning_modules",id,parent=parent,scope=scope,data=data);disposition("primary","modules",m["id"],"imported","mapped_learning_module",t)
 for l in R["lessons"]:
  t=token("primary","lessons",l["id"]);parent=module_tokens.get(l["module_id"])
  if l.get("exercise_type")!="code":disposition("primary","lessons",l["id"],"rejected","unsupported_flowchart_contract");continue
  if not parent:disposition("primary","lessons",l["id"],"rejected","missing_module");continue
  lesson_tokens[l["id"]]=t;lang=language(l);lesson_lang[l["id"]]=lang;id=hid("mig-les-",str(l["id"]));scope={"domain":"elearning","institution":"legacy-masked"};q=json.loads(l["quiz"]);cases=[]
  for case in json.loads(l["test_cases"]):cases.append({"description":case.get("description") or "Legacy testcase","input":case.get("input") or "","expected_output":case.get("expectedOutput") or ""})
  data={"id":id,"module_id":marker_id(parent),"level_id":marker_id(level_tokens[next(m["level_id"] for m in R["modules"] if m["id"]==l["module_id"])]),"title":l["title"],"sort_order":lesson_order[str(l["id"])],"language":lang,"explanation":l.get("explanation") or "Migrated lesson","code_example":l.get("code_example") or "","starter_code":l.get("initial_code") or "","solution":l.get("solution") or "","hint":l.get("hint") or "","xp_reward":int(l["xp_reward"]),"quiz":{"question":q.get("question") or "Legacy quiz","options":q.get("options") or [],"correct_option":int(q.get("correctAnswer",0))},"test_cases":cases,"status":"active","scope":scope,"version":1}
  domainrow(t,"learning_lessons",id,parent=parent,scope=scope,data=data);disposition("primary","lessons",l["id"],"imported","mapped_code_lesson",t)
 # Learning progress/profile; XP is a declared opening balance, not invented per-lesson XP.
 progress_count=Counter();game_count=Counter(r["nim"] for r in R.get("game_history",[]))
 for pgr in R["student_progress"]:
  t=token("primary","student_progress",pgr["id"]);parent=lesson_tokens.get(pgr["lesson_id"]);user=next((u for u in R["users"] if u.get("nim")==pgr["nim"]),None)
  if not parent:disposition("primary","student_progress",pgr["id"],"rejected","lesson_contract_rejected");continue
  if not user:disposition("primary","student_progress",pgr["id"],"rejected","identity_unresolved");continue
  ut=token("primary","users",user["id"]);mod=next(l["module_id"] for l in R["lessons"] if l["id"]==pgr["lesson_id"]);lvl=next(m["level_id"] for m in R["modules"] if m["id"]==mod);status="completed" if pgr.get("completed") else "in_progress";scope={"domain":"elearning","institution":"legacy-masked"};data={"lesson_id":marker_id(parent),"module_id":marker_id(module_tokens[mod]),"level_id":marker_id(level_tokens[lvl]),"user_id":marker_user(ut),"status":status,"draft_code":"","language":lesson_lang[pgr["lesson_id"]],"quiz_option":None,"test_passed":False,"revision":1,"xp_awarded":0,"completed_at":rfc(pgr.get("completed_at")) if status=="completed" else None,"updated_at":rfc(pgr.get("completed_at")) or cutoff}
  domainrow(t,"learning_progress","",id_template="progress",parent=parent,owner=ut,scope=scope,status=status,unique="",data=data);progress_count[ut]+=status=="completed";disposition("primary","student_progress",pgr["id"],"imported","completed_state_with_zero_award_opening_balance",t)
 for u in R["users"]:
  ut=token("primary","users",u["id"]);t=token("derived","learning_profile",u["id"]);xp=max(0,int(u.get("xp") or 0));level=int(math.floor(math.sqrt(xp/50)))+1 if xp>0 else 1;scope={"domain":"elearning","institution":"legacy-masked"};data={"user_id":marker_user(ut),"user_name":u["nama"],"xp":xp,"level":level,"next_level_xp":50*level*level,"streak":max(0,int(u.get("streak") or 0)),"last_active_date":(str(u.get("last_active") or "")[:10]),"completed_lessons":progress_count[ut],"games_played":game_count[u.get("nim")],"perfect_games":0,"unlocked_achievements":[],"timezone":"Asia/Jakarta","timezone_offset":"+07:00","updated_at":rfc(u.get("last_active")) or cutoff}
  domainrow(t,"learning_profiles","",id_template="profile",owner=ut,scope=scope,status="active",data=data)
 # Unsupported achievement/game rows are accounted honestly.
 for table,reason in (("achievements","code_defined_achievement_catalog"),("unlocked_achievements","achievement_semantics_not_proven"),("game_questions","missing_target_reward_policy"),("game_history","whole_game_not_question_attempt")):
  for r in R.get(table,[]):disposition("primary",table,r["id"],"rejected",reason)
 # Practicum definitions and conservative closed historical subset.
 session_tokens={};course_tokens={};question_tokens={};course_rows={r["id"]:r for r in R["course"]};session_rows={r["id"]:r for r in R["sesi_praktikum"]}
 scope0={"domain":"praktikum","institution":"legacy-masked"}
 for s in R["sesi_praktikum"]:
  t=token("primary","sesi_praktikum",s["id"]);session_tokens[s["id"]]=t;id=hid("mig-psn-",str(s["id"]));data={"id":id,"title":s["judul_sesi"],"sequence":int(s["urutan"]),"scope":scope0,"active":False,"version":1,"created_at":rfc(s.get("created_at"),True) or cutoff,"updated_at":rfc(s.get("updated_at"),True) or cutoff};domainrow(t,"practicum_sessions",id,scope=scope0,data=data);disposition("primary","sesi_praktikum",s["id"],"imported","mapped_practicum_session",t)
 for c in R["course"]:
  t=token("primary","course",c["id"]);parent=session_tokens.get(c["sesi_praktikum_id"])
  if not parent or c.get("jenis") not in ("pretest","posttest","keterampilan","ujian_praktik"):disposition("primary","course",c["id"],"rejected","invalid_course_contract");continue
  course_tokens[c["id"]]=t;id=hid("mig-pco-",str(c["id"]));data={"id":id,"session_id":marker_id(parent),"title":c["judul"],"kind":c["jenis"],"duration_minutes":int(c["durasi_menit"]),"token_required":False,"version":1};domainrow(t,"practicum_courses",id,parent=parent,scope=scope0,unique="",data=data);disposition("primary","course",c["id"],"imported","mapped_practicum_course",t)
 for q in R["soal"]:
  t=token("primary","soal",q["id"]);parent=course_tokens.get(q["course_id"])
  if q.get("is_arsip"):disposition("primary","soal",q["id"],"rejected","archived_question");continue
  if not parent or q.get("difficulty") not in ("easy","medium","hard") or q.get("jenis_soal") not in ("essay","coding") or q.get("poin") is None or not float(q["poin"]).is_integer():disposition("primary","soal",q["id"],"rejected","incomplete_question_contract");continue
  question_tokens[q["id"]]=t;id=hid("mig-pqu-",str(q["id"]));points=int(q["poin"]);data={"id":id,"course_id":marker_id(parent),"type":q["jenis_soal"],"difficulty":q["difficulty"],"category":q.get("kategori_ujian") or "legacy","prompt":q["teks_soal"],"points":points,"starter_code":q.get("kode_rumpang") or "","language":"c" if q["jenis_soal"]=="coding" else "","reference_answer":q.get("kunci_jawaban") or "","rubric":[{"id":"legacy-score","description":"Masked legacy scoring criterion","max_score":points}],"version":1};domainrow(t,"practicum_questions",id,parent=parent,scope=scope0,data=data);disposition("primary","soal",q["id"],"imported","mapped_practicum_question",t)
 selected=defaultdict(list)
 for s in R["soal_terpilih"]:selected[(s["aktivasi_sesi_id"],s["course_id"])].append(s)
 act_session={r["id"]:r for r in R["aktivasi_sesi"]};answers_by_selected_user={(r["soal_terpilih_id"],r["mahasiswa_id"]):r for r in R["jawaban_mahasiswa"]};activation_tokens={};imported_selected=set();ac_rows={(r["aktivasi_sesi_id"],r["course_id"]):r for r in R["aktivasi_course"]};attempts_by_ac=defaultdict(list)
 for pgr in R["pengerjaan_course"]:attempts_by_ac[(pgr["aktivasi_sesi_id"],pgr["course_id"])].append(pgr)
 imported_attempt_keys=set()
 for ac in R["aktivasi_course"]:
  t=token("primary","aktivasi_course",ac["id"]);key=(ac["aktivasi_sesi_id"],ac["course_id"]);asel=sorted(selected[key],key=lambda z:z["urutan"]);s=act_session.get(ac["aktivasi_sesi_id"]);ct=course_tokens.get(ac["course_id"]);st=session_tokens.get(s["sesi_praktikum_id"]) if s else None
  if ac.get("is_open") or not ac.get("started_at") or not s or not ct or not st or not asel or len(asel)>20 or any(z["soal_id"] not in question_tokens for z in asel):disposition("primary","aktivasi_course",ac["id"],"rejected","nonterminal_or_incomplete_activation");continue
  activation_tokens[key]=t;id=hid("mig-pac-",str(ac["id"]));cid=class_ids.get(s["kelas_id"]);scope={"domain":"praktikum","institution":"legacy-masked","session_id":marker_id(st),"shift":int(s["shift"])}
  if cid:scope["class_id"]=marker_id(cid)
  students=[marker_user(token("primary","users",p["mahasiswa_id"])) for p in attempts_by_ac[key]];questions=[marker_id(question_tokens[z["soal_id"]]) for z in asel];course_data=next(x["data"] for x in domain if x["token"]==ct);question_data=[next(x["data"] for x in domain if x["token"]==question_tokens[z["soal_id"]]) for z in asel];started=rfc(ac["started_at"],True);deadline=add_minutes(ac["started_at"],course_rows[ac["course_id"]]["durasi_menit"]);closed=rfc(ac.get("closed_at"),True) or deadline;activation={"id":id,"session_id":marker_id(st),"course_id":marker_id(ct),"scope":scope,"timer_mode":"shared","is_open":False,"started_at":started,"deadline_at":deadline,"closes_at":closed,"question_count":len(asel),"version":1,"student_ids":students,"entry_approved_ids":[],"question_ids":questions}
  domainrow(t,"practicum_activations",id,parent=st,scope=scope,expires=closed,data={"activation":activation,"course":course_data,"questions":question_data,"token_hash":""});disposition("primary","aktivasi_course",ac["id"],"imported","closed_complete_activation",t)
  for z in asel:imported_selected.add(z["id"])
 for s in R["aktivasi_sesi"]:disposition("primary","aktivasi_sesi",s["id"],"imported" if any(k[0]==s["id"] for k in activation_tokens) else "rejected","represented_by_imported_course_activations" if any(k[0]==s["id"] for k in activation_tokens) else "no_importable_course_activation")
 for s in R["soal_terpilih"]:disposition("primary","soal_terpilih",s["id"],"imported" if s["id"] in imported_selected else "rejected","embedded_in_activation_snapshot" if s["id"] in imported_selected else "activation_or_question_rejected")
 for pgr in R["pengerjaan_course"]:
  t=token("primary","pengerjaan_course",pgr["id"]);at=activation_tokens.get((pgr["aktivasi_sesi_id"],pgr["course_id"]));ut=token("primary","users",pgr["mahasiswa_id"])
  if not at or pgr.get("status")!="selesai" or not pgr.get("waktu_selesai"):disposition("primary","pengerjaan_course",pgr["id"],"rejected","attempt_not_terminal_or_activation_rejected");continue
  aid=hid("mig-pat-",str(pgr["id"]));asel=sorted(selected[(pgr["aktivasi_sesi_id"],pgr["course_id"])],key=lambda z:z["urutan"]);qdata=[next(x["data"] for x in domain if x["token"]==question_tokens[z["soal_id"]]) for z in asel];started=rfc(pgr.get("waktu_mulai"),True) or next(x["data"]["activation"]["started_at"] for x in domain if x["token"]==at);deadline=next(x["data"]["activation"]["deadline_at"] for x in domain if x["token"]==at);submitted=rfc(pgr["waktu_selesai"],True);ans=[]
  for z,qd in zip(asel,qdata):
   src=answers_by_selected_user.get((z["id"],pgr["mahasiswa_id"]));ans.append({"question_id":qd["id"],"value":src.get("jawaban_teks") if src else "","language":"c" if qd["type"]=="coding" else "","updated_at":rfc(src.get("updated_at"),True) if src else submitted,"score":None,"feedback":""})
  max_score=sum(q["points"] for q in qdata);scope=next(x["scope"] for x in domain if x["token"]==at);attempt={"attempt_id":aid,"activation_id":marker_id(at),"session_id":next(x["data"]["activation"]["session_id"] for x in domain if x["token"]==at),"course_id":next(x["data"]["activation"]["course_id"] for x in domain if x["token"]==at),"user_id":marker_user(ut),"status":"submitted","submission_reason":"legacy_rehearsal","revision":1,"grade_revision":0,"grading_status":"needs_manual","started_at":started,"deadline_at":deadline,"submitted_at":submitted,"server_now":cutoff,"timezone":"Asia/Jakarta","timezone_offset":"+07:00","questions":qdata,"answers":ans,"score":None,"max_score":max_score,"percentage":None};data={"attempt":attempt,"scope":scope,"user_name":next(u["name"] for u in users if u["token"]==ut),"submitted_answers":ans,"manual_grades":[],"grade_reason":""}
  domainrow(t,"practicum_attempts",aid,parent=at,owner=ut,scope=scope,status="submitted",unique="",expires=deadline,data=data);jt=t+"/grading_job";domainrow(jt,"practicum_grading_jobs",aid,parent=t,scope=scope,status="needs_manual",unique=aid,data={"id":aid,"attempt_id":aid,"status":"needs_manual","grade_revision":0,"suggestions":[],"simulated":False,"updated_at":submitted});imported_attempt_keys.add((pgr["mahasiswa_id"],pgr["aktivasi_sesi_id"],pgr["course_id"]));disposition("primary","pengerjaan_course",pgr["id"],"imported","submitted_attempt_requires_manual_review",t)
 # Answer rows are embedded; preserve the source disposition separately.
 viable_answer_ids=set()
 for d in domain:
  if d["table"]!="practicum_attempts":continue
  for answer in d["data"]["attempt"]["answers"]:
   # source IDs cannot be recovered from target answer, use selected/user lookup below.
   pass
 for arow in R["jawaban_mahasiswa"]:
  selrow=next((s for s in R["soal_terpilih"] if s["id"]==arow["soal_terpilih_id"]),None);pkey=(arow["mahasiswa_id"],selrow["aktivasi_sesi_id"],selrow["course_id"]) if selrow else None
  imported=pkey in imported_attempt_keys if pkey else False
  disposition("primary","jawaban_mahasiswa",arow["id"],"imported" if imported else "rejected","embedded_masked_answer" if imported else "attempt_or_question_rejected")
 for p in R["peserta_susulan"]:disposition("primary","peserta_susulan",p["id"],"rejected","late_admission_history_not_lossless")
 # Primary attendance/parallel schema is explicitly rejected; no relation is guessed.
 for table,reason in (("schedules","alternate_schedule_generation"),("group_members","alternate_roster_generation"),("group_assistants","alternate_roster_generation"),("qr_sessions","qr_bearer_and_schedule_link_missing"),("attendance_logs","attendance_schedule_link_missing"),("inventory_items","empty_alternate_inventory_source"),("inventory_rentals","empty_alternate_inventory_source")):
  for r in R.get(table,[]):disposition("primary",table,r["id"],"rejected",reason)
 # Separate SIAKAD: only inventory items are losslessly imported. Other rows remain accounted.
 for item in siakad["rows"].get("inventory_items",[]):
  t=token("siakad","inventory_items",item["id"]);id=hid("mig-itm-",str(item["id"]));qty=int(item["quantity"]);fee=int(item.get("price_per_day") or 0);status="available" if str(item.get("condition","")).lower()=="baik" else "maintenance";scope={"domain":"siakad","institution":"legacy-masked"};data={"id":id,"code":f"MIG-ITEM-{item['id']}","name":item["name"],"category":"legacy","total_quantity":qty,"available_quantity":qty,"rental_fee":fee,"status":status,"scope":scope,"version":1,"updated_at":rfc(item.get("last_checked")) or cutoff};domainrow(t,"academic_inventory_items",id,scope=scope,status=status,unique=data["code"],data=data);disposition("siakad","inventory_items",item["id"],"imported","mapped_inventory_without_accepted_rentals",t)
 for table,rows in siakad["rows"].items():
  if table=="inventory_items":continue
  reason={"schedules":"alternate_older_schedule_source","group_members":"alternate_older_roster_source","group_assistants":"alternate_older_roster_source","qr_sessions":"qr_bearer_excluded","attendance_logs":"attendance_schedule_link_missing","inventory_rentals":"borrower_identity_unresolved","equipment":"no_target_attachment_model","financial_records":"financial_ledger_not_rental_revenue"}.get(table,"unsupported_separate_source")
  for r in rows:disposition("siakad",table,r["id"],"rejected",reason)
 # Aggregate rejected rows omitted from snapshots, preserving complete source-table accounting.
 primary_counts={t["name"]:t["rows"] for t in primary["tables"]};processed=Counter(d["entity"] for d in disp if d["source_system"]=="primary" and d["rows"]==1)
 for table,count in primary_counts.items():
  missing=count-processed[table]
  if missing<0:raise ValueError("source accounting exceeded")
  if missing:disposition("primary",table,"aggregate-unselected","rejected","table_outside_approved_semantic_adapter",rows=missing)
 siakad_counts={t["name"]:t["rows"] for t in smeta["tables"]};processed_s=Counter(d["entity"] for d in disp if d["source_system"]=="siakad" and d["rows"]==1)
 # All separate users are duplicate identity source; unresolved rows are still explicit.
 matched=siakad["dispositions"]["users"]["matched"];unmatched=siakad["dispositions"]["users"]["rejected_missing_primary_identity"]
 if matched:disposition("siakad","users","aggregate-matched","rejected","duplicate_identity_source_crosswalked_to_primary",rows=matched)
 if unmatched:disposition("siakad","users","aggregate-unmatched","rejected","identity_missing_from_primary_authority",rows=unmatched)
 processed_s["users"]=matched+unmatched
 for table,count in siakad_counts.items():
  missing=count-processed_s[table]
  if missing<0:raise ValueError("separate source accounting exceeded")
  if missing:disposition("siakad",table,"aggregate-unselected","rejected","table_outside_approved_semantic_adapter",rows=missing)
 counts=Counter();reasons=Counter();source_total=0
 for d in disp:counts[d["disposition"]]+=d["rows"];reasons[d["reason"]]+=d["rows"];source_total+=d["rows"]
 plan={"format_version":FORMAT,"batch_id":BATCH,"cutoff":cutoff,"source":{"primary_project_fingerprint":primary["project_fingerprint"],"siakad_project_fingerprint":siakad["siakad_project_fingerprint"],"primary_rows":sum(primary_counts.values()),"siakad_rows":sum(siakad_counts.values())},"policy":{"identity":"exact primary identity; separate SIAKAD exact-NIM crosswalk only","learning_xp":"profile XP is opening balance; imported progress awards zero additional XP","practicum":"closed complete activations and submitted attempts only; all grading remains manual","attendance":"rejected when schedule/session linkage is not provable","files":"no object copy: target attachment model absent","unsupported":"explicit rejection; no invented semantics"},"auth_users":auth,"users":users,"assignments":assignments,"domain_rows":domain,"dispositions":disp,"counts":dict(counts),"reason_counts":dict(reasons)}
 plan["digest"]=hashlib.sha256(canonical(plan).encode()).hexdigest()
 if source_total!=plan["source"]["primary_rows"]+plan["source"]["siakad_rows"]:raise ValueError("source accounting mismatch")
 path=Path(a.out);fd=os.open(path,os.O_CREAT|os.O_EXCL|os.O_WRONLY,0o600)
 with os.fdopen(fd,"w",encoding="utf8") as f:json.dump(plan,f,separators=(",",":"),ensure_ascii=False);f.write("\n")
 print(json.dumps({"status":"passed","digest":plan["digest"],"source_rows":source_total,"dispositions":dict(counts),"target":{"auth_users":len(auth),"users":len(users),"assignments":len(assignments),"domain_rows":len(domain)},"top_rejections":reasons.most_common(8)}))
if __name__=="__main__":main()
