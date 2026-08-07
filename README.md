# KLDNS

KLDNS 是一个二级域名分发和 DNS 解析管理系统，支持用户注册二级域名、积分扣费、域名审核、解析记录管理和后台运维管理。

当前版本：`1.0.7`

## 技术栈

- 后端：Go、Gin、GORM、SQLite3
- 前端：Vite、Vue 3、TypeScript、Element Plus
- DNS 平台：Cloudflare、DNSPod、阿里云、DNS.com、DNSLA、DnsDun、西部数码、华为云、百度云、Route53、Google Cloud DNS

## 目录

```text
cmd/server/main.go                     # 服务端入口
internal/handler/                      # HTTP 处理器与路由注册
internal/service/                      # 业务逻辑
internal/repository/                   # 数据访问
internal/repository/migrations/        # 内嵌 SQLite 迁移
internal/model/                        # 数据模型与响应结构
internal/config/                       # 配置定义与加载
internal/middleware/                   # Gin 中间件
internal/runtime/                      # 进程运行时依赖
pkg/dns/                               # DNS 平台适配器
pkg/utils/                             # 通用工具包
configs/config.yaml                    # 默认运行配置
scripts/                               # 开发、测试与构建脚本
web/                                   # Vue 前端工程
```

## 配置

默认读取 `configs/config.yaml`，也可以通过 `KLDNS_CONFIG` 指定配置文件。

```yaml
app:
  name: kldns
  port: 8004
  mode: dev

database:
  path: data/kldns.db
  busy_timeout_ms: 5000
  wal: true

security:
  secret_key: change-me-before-production-kldns-secret
```

生产环境必须修改 `security.secret_key`。不要提交数据库文件、私钥、Token 或 DNS 平台密钥。

## 开发

后端：

```powershell
go test ./...
go run ./cmd/server
# 或执行：./scripts/dev.ps1
```

前端：

```powershell
cd web
npm run typecheck
npm run lint
npm run build
npm run dev
```

前端开发服务器会代理到 `127.0.0.1:8004`。

## 打包

```powershell
./scripts/build.ps1
# 或执行：make build
```

`build.ps1` 会在 `bin/` 下生成 `kldns-windows-amd64.exe` 和
`kldns-linux-amd64`。二进制会内嵌迁移文件和前端构建产物，
`configs/config.yaml` 和 `data/kldns.db` 仍为外部运行文件。

## 数据库迁移

程序启动时会自动执行内嵌的 `internal/repository/migrations/` 迁移，并记录到 `schema_migrations`。修改表结构时必须新增迁移文件。
