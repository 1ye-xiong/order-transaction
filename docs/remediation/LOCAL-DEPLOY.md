# 本机部署记录

2026-11-12，用户指定部署到本机与局域网。六服务已运行于独立 Docker 项目 `order-transaction-live`，镜像 `order-transaction:local-release`，没有覆盖其他项目。

- 本机：http://localhost:18080
- 局域网：http://10.80.65.5:18080（需同一网络，地址随本机IP变化）
- 教学账号：buyer1、buyer2、buyer3、warehouse01、cs01、ops01；密码123456。
- 数据卷：order-transaction-live_dbdata。停止服务不得删除此卷。
- 签名密钥已随机生成，保存在Git忽略的deploy/config.local-release.yaml，不要上传。
- 容器配置自动恢复；电脑重启后需启动Docker Desktop。

启动命令（项目根目录）：

```powershell
docker compose --env-file deploy/.env.local-release -p order-transaction-live -f deploy/docker-compose.yml up -d --no-build --wait
```

停止且保留数据：

```powershell
docker compose --env-file deploy/.env.local-release -p order-transaction-live -f deploy/docker-compose.yml stop
```

主机访问自身局域网地址通过，未验证另一台设备访问。防火墙添加规则被系统拒绝；其他设备无法访问时，用管理员PowerShell运行deploy/allow-lan.ps1，仅放行同子网18080。

标准构建依赖下载停滞后停止，本次利用已有Go1.25 Linux缓存编译六个二进制，结合已验证React构建，通过deploy/Dockerfile.local生成镜像。原始输出：local-binaries-build.txt、local-release-image.txt、local-release-start.txt、local-smoke.txt、local-restart.txt。二进制摘要：local-binaries-sha256.json。独立只读验收：local-deploy-acceptance.md。

这是用户授权的本机教学部署。整体覆盖率最近实测45.57%，未达项目85%门禁，未完成main/PR/CI发布链，不能声称全部正式质量发布要求通过。没有公网HTTPS发布，支付仍为教学模拟渠道。
