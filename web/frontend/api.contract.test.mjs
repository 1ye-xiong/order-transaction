import {test} from 'node:test';
import assert from 'node:assert/strict';
import {request,money} from './src/api.ts';

test('actual JSON envelope is unwrapped; JWT and paging reach server',async()=>{
 let seen;
 globalThis.fetch=async(url,options)=>{seen={url,options};return Response.json({request_id:'r',data:{items:[{order_no:'O1'}],total:'21'}})};
 const data=await request('/orders/mine?page=2&page_size=20','signed-token');
 assert.equal(data.total,'21');assert.equal(data.items[0].order_no,'O1');
 assert.equal(seen.url,'/api/v1/orders/mine?page=2&page_size=20');
 assert.equal(seen.options.headers.Authorization,'Bearer signed-token');
});
test('login and current-user preserve server role and token',async()=>{
 globalThis.fetch=async()=>Response.json({data:{token:'signed',user:{account:'cs01',role:'cs'}}});
 const login=await request('/users/login','','POST',{account:'cs01',password:'test'});
 assert.equal(login.token,'signed');assert.equal(login.user.role,'cs');
 globalThis.fetch=async()=>Response.json({data:{account:'cs01',role:'cs'}});
 assert.equal((await request('/users/me','signed')).role,'cs');
});
test('HTTP errors expose backend explanation',async()=>{
 globalThis.fetch=async()=>Response.json({error:{message:'不能访问他人订单'}},{status:403});
 await assert.rejects(request('/orders/O1','x'),/403: 不能访问他人订单/);
});
test('business rejected HTTP 200 operations cannot display success',async()=>{
 for(const path of ['/warehouse/orders/O1/ship','/orders/O1/pay','/cs/pay-exceptions/1/refund']){
  globalThis.fetch=async()=>Response.json({data:{success:false,accepted:false,message:'业务拒绝'}});
  await assert.rejects(request(path,'x','POST'),/业务拒绝/);
 }
});
test('approved request returning REFUND_FAILED remains failed server state',async()=>{
 globalThis.fetch=async()=>Response.json({data:{status:'REFUND_FAILED',refund_fail_reason:'渠道忙'}});
 const data=await request('/cs/refunds/1/review','x','POST',{approve:true,reason:'核实'});
 assert.equal(data.status,'REFUND_FAILED');assert.equal(data.refund_fail_reason,'渠道忙');
});
test('money uses integer cents and flags unsafe values',()=>{
 assert.equal(money(10001),'100.01');assert.equal(money(-1),'-0.01');assert.equal(money('10000'),'100.00');
 assert.equal(money(1.1),'金额异常');assert.equal(money(Number.MAX_SAFE_INTEGER+1),'金额异常');
});
