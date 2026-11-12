import assert from 'node:assert/strict';
import {request,money} from '../web/frontend/src/api.ts';
const base=process.argv[2]; const realFetch=globalThis.fetch;
globalThis.fetch=(url,options)=>realFetch(base+url,options);
const tokens={};
for(const [account,role] of [['buyer1','buyer'],['buyer2','buyer'],['buyer3','buyer'],['ops01','ops'],['cs01','cs'],['warehouse01','warehouse']]) {
 const d=await request('/users/login','','POST',{account,password:'123456'}); assert.ok(d.token);
 tokens[account]=d.token; const me=await request('/users/me',d.token); assert.equal(me.role,role);
 console.log('PASS actual request login/me '+account+' -> '+me.role);
}
const products=await request('/products',tokens.buyer1); assert.equal(products.items.length,2);
await request('/orders/mine?page=1&page_size=20',tokens.buyer1);
for(const [who,path] of [['warehouse01','/warehouse/orders'],['cs01','/cs/refunds'],['cs01','/cs/pay-exceptions'],['ops01','/ops/summary'],['ops01','/channel/config'],['ops01','/channel/records']]) { await request(path,tokens[who]); console.log('PASS actual request '+who+' '+path); }
await assert.rejects(()=>request('/ops/summary',tokens.buyer1),/^Error: 403:/);
await assert.rejects(()=>request('/users/me',''),/^Error: 401:/);
assert.equal(money('12345'),'123.45');assert.equal(money('9007199254740992'),'金额异常');
console.log('PASS actual request unwrap, 401/403 errors and integer-cent formatting');
