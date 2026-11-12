# 本机部署独立验收

结论：用户授权的本机部署已完成，独立读取验收通过。地址：http://127.0.0.1:18080 。这是本机部署结果，不是 main 正式质量发布；整体覆盖率尚未达到项目质量门禁。

本机部署独立验收时间 2026-11-12T16:31:22.510247+08:00

PASS /healthz HTTP200 application/json; charset=utf-8

PASS / HTTP200 text/html; charset=utf-8

PASS /assets/index-B4HXtLM6.js 211499 bytes text/javascript; charset=utf-8

PASS /assets/index-rxN9xOh8.css 22813 bytes text/css; charset=utf-8

PASS actual request login/me buyer1 -> buyer
PASS actual request login/me buyer2 -> buyer
PASS actual request login/me buyer3 -> buyer
PASS actual request login/me ops01 -> ops
PASS actual request login/me cs01 -> cs
PASS actual request login/me warehouse01 -> warehouse
PASS actual request warehouse01 /warehouse/orders
PASS actual request cs01 /cs/refunds
PASS actual request cs01 /cs/pay-exceptions
PASS actual request ops01 /ops/summary
PASS actual request ops01 /channel/config
PASS actual request ops01 /channel/records
PASS actual request unwrap, 401/403 errors and integer-cent formatting


PASS 非破坏读取验收；未创建订单、未变更渠道或业务数据。登录会正常产生认证日志。

## 验收边界

实际验证网关健康、React入口及JS/CSS、六账号登录与服务端角色、买家/仓库/客服/运营读取接口、匿名401与买家越权403。使用实际前端请求模块连接运行中的服务，未输出token或密钥。未实施浏览器点击、订单写入、支付或渠道修改。

局域网候选地址：http://10.80.65.5:18080 。未从局域网其他设备验证可达性；Windows防火墙规则添加遭拒绝访问，因此不能声明局域网访问已经成功。Docker六服务部署状态由部署者核验，本报告只声明上述HTTP独立结果。
