#!/usr/bin/env python3
"""Merge a separate legacy SIAKAD source into an already-masked primary snapshot.
Raw rows and credentials stay in memory. Output contains only masked records and
complete disposition counts; it is not an importer and performs no writes.
"""
import argparse, hashlib, json, os, re, urllib.error, urllib.parse, urllib.request
from pathlib import Path

TABLE_FIELDS={
"users":"id,nim,role,division,is_active,class_code,major,shift",
"schedules":"id,title,type,day_of_week,start_time,end_time,room,status,major,class_code,updated_at",
"group_members":"id,schedule_id,student_id,assistant_id",
"group_assistants":"id,schedule_id,assistant_id",
"qr_sessions":"id,title,is_active,created_at,created_by",
"attendance_logs":"id,custom_user_id,check_in_time,status,is_verified,session_id,verification_status,reschedule_schedule_id,reschedule_status,type,recorded_by",
"inventory_items":"id,name,condition,quantity,location,last_checked,is_rentable,price_per_day",
"inventory_rentals":"id,user_id,item_id,quantity,start_date,end_date,type,total_price,status,created_at,updated_at",
"equipment":"id,name,type,description,uploaded_at",
"financial_records":"id,title,type,amount,category,date",
}
USER_FIELDS={"student_id","assistant_id","custom_user_id","recorded_by","created_by","user_id"}
class NoRedirect(urllib.request.HTTPRedirectHandler):
 def redirect_request(self,req,fp,code,msg,headers,newurl): raise RuntimeError("redirect refused")
def env(path):
 out={}
 for line in Path(path).read_text(encoding="utf-8-sig").splitlines():
  m=re.match(r"^\s*(?:export\s+)?([A-Za-z_][A-Za-z0-9_]*)\s*=\s*(.*)$",line)
  if not m:continue
  k,v=m.groups()
  if k not in {"SUPABASE_URL","SUPABASE_SERVICE_ROLE_KEY","SUPABASE_SERVICE_KEY"}:continue
  if v.startswith('"'):
   try:v=json.loads(v)
   except Exception:continue
  elif len(v)>=2 and v[0]==v[-1]=="'":v=v[1:-1]
  out[k]=v
 return out
def api(cfg):
 u=urllib.parse.urlsplit(cfg.get("SUPABASE_URL",""));key=cfg.get("SUPABASE_SERVICE_ROLE_KEY") or cfg.get("SUPABASE_SERVICE_KEY")
 if u.scheme!="https" or not (u.hostname or "").endswith(".supabase.co") or not key:raise ValueError("invalid source API configuration")
 return "https://"+u.hostname,key,hashlib.sha256(u.hostname.encode()).hexdigest()
def fetch_all(origin,key,table,fields):
 client=urllib.request.build_opener(NoRedirect());rows=[];offset=0
 while True:
  query=urllib.parse.urlencode({"select":fields,"order":"id.asc","limit":1000,"offset":offset})
  req=urllib.request.Request(origin+"/rest/v1/"+table+"?"+query,headers={"apikey":key,"Authorization":"Bearer "+key,"Accept":"application/json"})
  with client.open(req,timeout=15) as r:body=r.read((8<<20)+1)
  if len(body)>8<<20:raise ValueError("source response exceeds bound")
  page=json.loads(body);rows.extend(page)
  if len(page)<1000:return rows
  offset+=len(page)
  if offset>100000:raise ValueError("source row bound exceeded")
def main():
 p=argparse.ArgumentParser(description=__doc__);p.add_argument("--primary-env",required=True);p.add_argument("--siakad-env",required=True);p.add_argument("--primary-masked",required=True);p.add_argument("--expected-primary",required=True);p.add_argument("--expected-siakad",required=True);p.add_argument("--out",required=True);a=p.parse_args()
 primary=json.loads(Path(a.primary_masked).read_text());masked_users=primary.get("masked_rows",{}).get("users",[])
 po,pk,pf=api(env(a.primary_env));so,sk,sf=api(env(a.siakad_env))
 if pf!=a.expected_primary or sf!=a.expected_siakad or pf==sf:raise SystemExit("source identity mismatch")
 raw_primary=fetch_all(po,pk,"users","id,nim,role")
 if len(raw_primary)!=len(masked_users):raise SystemExit("primary identity snapshot drift")
 nim_to_masked={};source_ids=set()
 for source,masked in zip(raw_primary,masked_users):
  if source.get("role")!=masked.get("role"):raise SystemExit("primary snapshot ordering mismatch")
  nim=(source.get("nim") or "").strip()
  if not nim or nim in nim_to_masked:raise SystemExit("primary NIM mapping ambiguous")
  nim_to_masked[nim]=masked["id"];source_ids.add(masked["id"])
 separate={t:fetch_all(so,sk,t,f) for t,f in TABLE_FIELDS.items()}
 separate_id_to_masked={};unmatched_users=[]
 for u in separate["users"]:
  nim=(u.get("nim") or "").strip();mapped=nim_to_masked.get(nim)
  if mapped is None:unmatched_users.append(u["id"])
  else:separate_id_to_masked[u["id"]]=mapped
 out={"mode":"read-only-in-memory-cross-source-mask","status":"passed","primary_project_fingerprint":pf,"siakad_project_fingerprint":sf,"masking_policy":"reuse-primary-opaque-identities-no-credentials-no-free-text","rows":{},"dispositions":{"users":{"matched":len(separate_id_to_masked),"rejected_missing_primary_identity":len(unmatched_users)}}}
 for table,rows in separate.items():
  if table=="users":continue
  accepted=[];rejected=0
  for row in rows:
   row=dict(row);ok=True
   for field in USER_FIELDS & row.keys():
    value=row.get(field)
    if value is None:continue
    mapped=separate_id_to_masked.get(value)
    if mapped is None:ok=False;break
    row[field]=mapped
   if not ok:rejected+=1;continue
   # Free text that can contain people/health/financial descriptions is redacted.
   for field in ("title","description","location"):
    if row.get(field):row[field]="[Masked legacy "+field+"]"
   accepted.append(row)
  out["rows"][table]=accepted;out["dispositions"][table]={"import_candidate":len(accepted),"rejected_unresolved_identity":rejected}
 path=Path(a.out);fd=os.open(path,os.O_CREAT|os.O_EXCL|os.O_WRONLY,0o600)
 with os.fdopen(fd,"w",encoding="utf8") as f:json.dump(out,f,separators=(",",":"));f.write("\n")
 print(json.dumps({"status":"passed","matched_users":len(separate_id_to_masked),"unmatched_users":len(unmatched_users),"tables":{k:len(v) for k,v in out["rows"].items()}}))
if __name__=="__main__":main()
