import audit_e2e as a
import pathlib,subprocess,re,urllib.request,hashlib,json,datetime,traceback
root=a.ROOT
# Redirect this run's manifest; preserve previous integration evidence.
a.OUT=root/'docs/remediation/frontend-runtime';a.OUT.mkdir(exist_ok=True)
h=a.Harness();ok=False
try:
 h.prepare();h.start(); origin=h.base.removesuffix('/api/v1')
 def get(p):
  with urllib.request.urlopen(origin+p) as r:return r.status,r.headers.get('Content-Type'),r.read()
 status,ct,body=get('/'); assert b'/assets/' in body and b'<div id="root"></div>' in body,(ct,body[:500])
 for asset in re.findall(rb'(?:src|href)="(/assets/[^\"]+)"',body):
  p=asset.decode();status,ct,b=get(p);assert len(b)>1000 and 'text/html' not in ct
  a.log('PASS compiled asset '+p+' '+str(len(b))+' bytes '+ct)
 assert get('/buyer/orders')[2]==body
 assert b'/assets/' not in get('/legacy')[2]
 try:get('/api/unknown');raise AssertionError('API fallback unexpectedly successful')
 except urllib.error.HTTPError as e:assert e.code==404
 a.log('PASS real gateway root compiled React, SPA route, legacy page and API404')
 result=subprocess.run(['node',str(root/'tools/audit_react_api.mjs'),origin],cwd=root,text=True,encoding='utf-8',capture_output=True)
 a.log(result.stdout);a.log(result.stderr);assert result.returncode==0
 h.run();ok=True
except Exception:a.log(traceback.format_exc())
finally:
 h.stop();a.log('Runtime retained '+str(h.work));a.log('RESULT '+('PASS' if ok else 'FAIL'))
 (a.OUT/'results.txt').write_text('\n'.join(a.LINES),encoding='utf-8')
 files={str(p.relative_to(root)):hashlib.sha256(p.read_bytes()).hexdigest() for p in list((root/'web/frontend/src').glob('*'))+list((root/'web/dist').rglob('*')) if p.is_file()}
 (a.OUT/'frontend-source-manifest.json').write_text(json.dumps({'time':datetime.datetime.now().isoformat(),'files':files},indent=2),encoding='utf-8')
raise SystemExit(0 if ok else 1)
