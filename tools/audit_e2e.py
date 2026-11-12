"""Isolated six-process HTTP acceptance. Uses real SQLite and the teaching channel.
Run: python tools/audit_e2e.py . Results are evidence, not a deployment claim.
"""
import concurrent.futures, datetime, hashlib, json, pathlib, shutil, socket, subprocess, sys, tempfile, time, traceback, urllib.request, urllib.error
ROOT = pathlib.Path(__file__).resolve().parents[1]
OUT = ROOT / 'docs/remediation'
OUT.mkdir(parents=True, exist_ok=True)
LINES = []
def log(text):
    text = str(text); print(text, flush=True); LINES.append(text)
def freeport():
    with socket.socket() as s:
        s.bind(('127.0.0.1',0)); return s.getsockname()[1]
class Harness:
    def __init__(self):
        self.work = pathlib.Path(tempfile.mkdtemp(prefix='trade-acceptance-'))
        self.procs = []; self.handles=[]; self.tokens={}; self.ports={n:freeport() for n in ['gateway','user','product','order','payment','warehouse','note']}
        self.base='http://127.0.0.1:'+str(self.ports['gateway'])+'/api/v1'
    def prepare(self):
        (self.work/'configs').mkdir(); (self.work/'bin').mkdir(); (self.work/'data').mkdir()
        shutil.copytree(ROOT/'web',self.work/'web')
        cfg=(ROOT/'configs/config.yaml').read_text(encoding='utf-8').replace('host: 0.0.0.0','host: 127.0.0.1').replace('port: 8080','port: '+str(self.ports['gateway'])).replace('order_pay_timeout_seconds: 600','order_pay_timeout_seconds: 3')
        for name,old in [('user',9001),('note',9002),('order',9100),('product',9101),('payment',9103),('warehouse',9104)]:
            cfg=cfg.replace('port: '+str(old),'port: '+str(self.ports[name])).replace('127.0.0.1:'+str(old),'127.0.0.1:'+str(self.ports[name]))
        (self.work/'configs/config.yaml').write_text(cfg,encoding='utf-8')
        source = {}
        for folder in ['cmd','internal','pkg','api','web','configs']:
            for path in (ROOT/folder).rglob('*'):
                if path.is_file() and path.suffix in ['.go','.html','.yaml','.yml']:
                    source[path.relative_to(ROOT).as_posix()] = hashlib.sha256(path.read_bytes()).hexdigest()
        (OUT/'e2e-source-manifest.json').write_text(json.dumps({'time':datetime.datetime.now(datetime.timezone(datetime.timedelta(hours=8))).isoformat(),'head':subprocess.check_output(['git','rev-parse','HEAD'],cwd=ROOT,text=True).strip(),'scope':'HEAD plus file hashes, uncommitted worktree','files':source},ensure_ascii=False,indent=2),encoding='utf-8')
        for name in ['user','product','order','payment','warehouse','gateway']:
            exe=name+('.exe' if sys.platform=='win32' else '')
            subprocess.run(['go','build','-o',str(self.work/'bin'/exe),'./cmd/'+name],cwd=ROOT,check=True)
        log('Built six services. Isolated runtime: '+str(self.work))
    def start(self):
        for name in ['user','product','order','payment','warehouse','gateway']:
            handle=open(self.work/(name+'.log'),'ab'); self.handles.append(handle)
            exe=name+('.exe' if sys.platform=='win32' else '')
            p=subprocess.Popen([str(self.work/'bin'/exe)],cwd=self.work,stdout=handle,stderr=handle); self.procs.append(p)
            for _ in range(150):
                if p.poll() is not None: raise RuntimeError(name+' exited: '+(self.work/(name+'.log')).read_text(encoding='utf-8',errors='replace')[-3000:])
                try:
                    with socket.create_connection(('127.0.0.1',self.ports[name]),.1): break
                except OSError: time.sleep(.1)
            else: raise RuntimeError(name+' startup timeout')
        for account in ['buyer1','buyer2','buyer3','ops01','cs01','warehouse01']:
            self.tokens[account]=self.api('POST','/users/login',body={'account':account,'password':'123456'})['token']
        log('All six services started; six teaching accounts authenticated.')
    def stop(self):
        for p in reversed(self.procs):
            p.terminate()
        for p in self.procs:
            try:p.wait(timeout=8)
            except subprocess.TimeoutExpired:p.kill(); p.wait()
        self.procs=[]
        for h in self.handles:h.close()
        self.handles=[]
    def api(self,method,path,who=None,body=None,allow_error=False):
        headers={'Content-Type':'application/json'}
        if who:headers['Authorization']='Bearer '+self.tokens[who]
        req=urllib.request.Request(self.base+path,data=json.dumps(body).encode() if body is not None else None,method=method,headers=headers)
        try:
            with urllib.request.urlopen(req,timeout=15) as r: code=r.status; data=json.load(r)
        except urllib.error.HTTPError as e:
            code=e.code; data=json.load(e)
        if allow_error:return code,data.get('data',data)
        assert code<300,(method,path,code,data)
        return data.get('data',{})
    def product(self,name,qty=5):
        return self.api('POST','/products','ops01',{'name':name,'price_cents':10000,'stock_total':qty})['id']
    def stock(self,p):
        return next(x for x in self.api('GET','/products?include_off_sale=true','ops01')['items'] if x['id']==p)
    def order(self,p,who='buyer1'):
        return self.api('POST','/orders',who,{'product_id':p,'quantity':1})['order_no']
    def get(self,o,who='buyer1'):return self.api('GET','/orders/'+o,who)
    def config(self,**extra):
        cfg={'pay_behavior':'success','notify_count':1,'notify_delay_seconds':0,'pay_late_seconds':0,'refund_success_from_attempt':1}; cfg.update(extra)
        self.api('PUT','/channel/config','ops01',cfg)
    def wait(self,fn,timeout=12):
        until=time.monotonic()+timeout
        while time.monotonic()<until:
            value=fn()
            if value:return value
            time.sleep(.1)
        raise AssertionError('condition timed out')
    def pay(self,o):
        self.api('POST','/orders/'+o+'/pay','buyer1')
        self.wait(lambda:self.get(o)['status']=='PAID')
    def ship(self,o,allow_error=False):
        result=self.api('POST','/warehouse/orders/'+o+'/ship','warehouse01',{'carrier':'教学物流','tracking_no':'TRACK-'+o},allow_error)
        if not allow_error:assert result.get('success'),result
        return result
    def refund_id(self,o):
        return next(x['id'] for x in self.api('GET','/cs/refunds','cs01')['items'] if x['order_no']==o)
    def run(self):
        initial=self.api('GET','/products','buyer1')['items']; assert len(initial)==2,initial
        log('Seed: exactly two products; three buyers and three staff authenticated. Teaching expiry explicitly shortened to 3 seconds.')
        self.config(notify_count=3)
        p=self.product('S1 normal'); o=self.order(p); self.pay(o); self.ship(o)
        got=self.get(o); assert got['status']=='SHIPPED' and got['tracking_no']=='TRACK-'+o,got
        assert int(self.stock(p).get('available',0))==4,self.stock(p)
        records=self.api('GET','/ops/orders/'+o+'/records','ops01')['items']; assert any(x['type']=='SHIP' for x in records)
        log('PASS S1: real HTTP create/pay (3 repeated notifications)/ship/buyer tracking/ops stock and records. order='+o)
        persisted=o
        p=self.product('S2 concurrent'); self.config(pay_behavior='no_notify')
        def create(i):
            who='buyer'+str(i%3+1); status,data=self.api('POST','/orders',who,{'product_id':p,'quantity':1},True)
            return who,status,data
        with concurrent.futures.ThreadPoolExecutor(max_workers=20) as pool: results=list(pool.map(create,range(20)))
        won=[(w,d['order_no']) for w,c,d in results if c<300]
        assert len(won)==5,results
        assert int(self.stock(p).get('available',0))==0,self.stock(p)
        for who,order in won[:2]:
            self.api('POST','/orders/'+order+'/cancel',who); self.api('POST','/orders/'+order+'/cancel',who)
        assert int(self.stock(p).get('available',0))==2,self.stock(p)
        self.order(p,'buyer3'); self.order(p,'buyer2')
        assert int(self.stock(p).get('available',0))==0,self.stock(p)
        log('PASS S2: 20 HTTP requests / 5 successful / 15 rejected; cancel 2 twice, then 2 other purchases; no oversell.')
        self.config(refund_success_from_attempt=2)
        p=self.product('S3 refund'); o=self.order(p); self.pay(o)
        self.api('POST','/orders/'+o+'/refund','buyer1',{'reason':'不需要'})
        code,data=self.ship(o,True); assert code>=400 or not data.get('success',False),(code,data)
        rid=self.refund_id(o)
        code,_=self.api('POST','/cs/refunds/'+rid+'/review','cs01',{'approve':True,'reason':' '},True); assert code>=400
        self.api('POST','/cs/refunds/'+rid+'/review','cs01',{'approve':True,'reason':'已核实同意'})
        assert self.get(o)['status']=='REFUND_FAILED'
        self.api('POST','/cs/orders/'+o+'/refund-retry','cs01')
        assert self.get(o)['status']=='REFUNDED' and int(self.stock(p).get('available',0))==5
        log('PASS S3: paid/refund apply/shipping blocked/blank review rejected/first channel refund fails/retry succeeds/5 stock restored.')
        self.config(notify_count=3,pay_late_seconds=5)
        p=self.product('S4 late'); o=self.order(p); self.api('POST','/orders/'+o+'/pay','buyer1')
        self.wait(lambda:self.get(o)['status']=='CLOSED_TIMEOUT')
        self.wait(lambda:any(x['order_no']==o for x in self.api('GET','/cs/pay-exceptions','cs01').get('items',[])))
        ex=next(x for x in self.api('GET','/cs/pay-exceptions','cs01')['items'] if x['order_no']==o)
        self.api('POST','/cs/pay-exceptions/'+ex['id']+'/refund','cs01')
        self.api('POST','/cs/pay-exceptions/'+ex['id']+'/refund','cs01')
        assert self.get(o)['status']=='CLOSED_TIMEOUT' and int(self.stock(p).get('available',0))==5
        entries=self.api('GET','/channel/records','ops01')['items']
        refunds=[x for x in entries if x.get('refund_no')=='RFE'+ex['id'] and x['result']=='success']; assert len(refunds)==1,refunds
        summary=self.api('GET','/ops/summary','ops01'); assert int(summary['total_paid_cents'])==30000 and int(summary['total_refunded_cents'])==20000,summary
        log('PASS S4: timeout before 3 delayed success notifications; exception refunded twice requests, only 1 channel success; stock 5; summary paid=30000/refunded=20000.')
        self.config(pay_behavior='fail')
        failed=self.order(p); self.api('POST','/orders/'+failed+'/pay','buyer1'); time.sleep(.3)
        assert self.get(failed)['status']=='PENDING_PAYMENT'
        self.api('POST','/orders/'+failed+'/cancel','buyer1')
        self.config(pay_behavior='no_notify')
        silent=self.order(p); self.api('POST','/orders/'+silent+'/pay','buyer1')
        self.wait(lambda:self.get(silent)['status']=='CLOSED_TIMEOUT')
        log('PASS channel branches: explicit FAILED leaves order unpaid; no_notify times out server-side and releases stock.')
        self.config()
        p=self.product('S5 competition'); o=self.order(p); self.pay(o)
        with concurrent.futures.ThreadPoolExecutor(max_workers=2) as pool:
            a=pool.submit(self.api,'POST','/orders/'+o+'/refund','buyer1',{'reason':'竞争退款'},True)
            b=pool.submit(self.ship,o,True)
            ar,br=a.result(),b.result()
        state=self.get(o)['status']; assert state in ['SHIPPED','REFUND_REQUESTED'],(state,ar,br)
        if state=='SHIPPED':assert ar[0]>=400,(state,ar,br)
        else: assert br[0]>=400 or not br[1].get('success',False),(state,ar,br)
        assert self.api('GET','/orders/'+o,'buyer2',allow_error=True)[0]==403
        assert self.api('POST','/orders/'+o+'/cancel','buyer2',allow_error=True)[0]==403
        assert self.api('GET','/ops/summary','buyer1',allow_error=True)[0]==403
        assert self.api('GET','/channel/config',allow_error=True)[0]==401
        assert self.api('GET','/channel/config','buyer1',allow_error=True)[0]==403
        log('PASS S5: simultaneous refund/ship -> '+state+'; other buyer GET/cancel denied; staff operation and channel auth boundaries denied.')
        before=self.api('GET','/ops/summary','ops01'); self.stop(); self.start()
        assert self.get(persisted)['status']=='SHIPPED'
        after=self.api('GET','/ops/summary','ops01')
        assert int(before['total_paid_cents'])==int(after['total_paid_cents']) and int(before['total_refunded_cents'])==int(after['total_refunded_cents']),(before,after)
        assert len(self.api('GET','/products?include_off_sale=true','ops01')['items'])==7
        log('PASS restart: all six processes stopped/restarted against same file; shipment, amounts and product count persisted; no duplicate seeding.')

def main():
    h=Harness(); ok=False
    log('Acceptance time '+datetime.datetime.now(datetime.timezone(datetime.timedelta(hours=8))).isoformat())
    log('Source HEAD '+subprocess.check_output(['git','rev-parse','HEAD'],cwd=ROOT,text=True).strip()+' plus uncommitted worktree changes (not an exact commit release).')
    try:h.prepare(); h.start(); h.run(); ok=True
    except Exception:log(traceback.format_exc())
    finally:
        h.stop(); log('Runtime logs retained at '+str(h.work)); log('RESULT '+('PASS (HTTP integration scope only)' if ok else 'FAIL'))
        (OUT/'e2e.txt').write_text('\n'.join(LINES)+'\n',encoding='utf-8')
    return 0 if ok else 1
if __name__=='__main__':sys.exit(main())
