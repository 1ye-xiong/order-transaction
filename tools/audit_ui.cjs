const fs = require('node:fs');
const vm = require('node:vm');
const assert = require('node:assert/strict');
const source = fs.readFileSync('web/index.html','utf8').split('<script>')[1].split('</script>')[0];
const values = {'account':'ops01','password':'123456','np-name':'test','np-price':'10000','np-stock':'5','ch-behavior':'success','ch-count':'2','ch-delay':'0','ch-late':'0','ch-refund':'2'};
const elements = new Map();
const el=id=> { if(!elements.has(id)) elements.set(id,{value:values[id]||'',style:{},innerHTML:'',textContent:'',classList:{add(){},remove(){}}});return elements.get(id); };
const calls=[];const local = new Map();
const context = vm.createContext({ console, setTimeout(){}, localStorage:{getItem:k=>local.get(k),setItem:(k,v)=>local.set(k,v),clear:()=>local.clear()},location:{reload(){}},window:{addEventListener(){}}, document:{getElementById:el,querySelectorAll:()=>[],querySelector:()=>null},fetch:async(url,opts)=>{calls.push({url,...opts});let data={};if(url.endsWith('/login')) data={token:'verified-token',user:{display_name:'operator',account:'ops01',role:'ops'}};else if(url.endsWith('/channel/config'))data={pay_behavior:'success',notify_count:3,notify_delay_seconds:1,pay_late_seconds:0,refund_success_from_attempt:2};else if(url.includes('/products'))data={items:[]};return {ok:true,json:async()=>({data})};}});
vm.runInContext(source,context);
(async()=>{
 await vm.runInContext('login()',context);assert.equal(local.get('token'),'verified-token');assert.equal(local.get('role'),'ops');
 await vm.runInContext('createProduct()',context);let request=calls.find(c=>c.method==='POST'&&c.url.endsWith('/products'));assert.deepEqual(JSON.parse(request.body),{name:'test',price_cents:10000,stock_total:5});
 await vm.runInContext('loadChannel()',context);assert.equal(el('ch-count').value,3);
 await vm.runInContext('saveChannel()',context);request=calls.find(c=>c.method==='PUT'&&c.url.endsWith('/channel/config'));assert.equal(JSON.parse(request.body).notify_count,3);
 console.log('PASS: login envelope, authenticated product load/create, channel DOM load/save');
})().catch(e=>{console.error(e);process.exitCode=1});
