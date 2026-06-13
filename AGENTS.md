# AstraScheduleServerGo 后端知识库

## 概述

Go 1.26 + Gin + GORM 后端 API，提供课表数据存储、调休计算、WebSocket 广播等功能。

## 项目结构

```
AstraScheduleServerGo/
├── main.go                    # 入口文件，路由定义和启动
├── config.toml                # 实际配置（TOML 格式）
├── config.template.toml       # 配置模板
├── AstraServerGo.openapi.json # OpenAPI 规范文档
├── config/                    # 配置加载层
│   └── load.go                # Viper 读取 TOML 配置
├── model/                     # 数据模型层
│   ├── globalVar.go           # 全局变量
│   ├── server.go              # 服务器配置结构体
│   ├── srvConfig.go           # 服务配置及校验
│   ├── weather.go             # 天气 API 响应结构
│   └── dbTable/               # 数据库表模型（GORM）
│       ├── schedule.go        # Schedule 表（课表）
│       ├── clientConfig.go    # ClientConfig 表
│       ├── timetable.go       # Timetable 表（作息时间表）
│       ├── subject.go         # Subject 表（科目配置）
│       ├── version.go         # DataVersion 表
│       ├── autorunRecord.go   # AutorunRecord 表
│       └── countdownRecord.go # CountdownRecord 表
├── db/                        # 数据访问层（DAL）
│   ├── connect.go             # 数据库连接（单例）
│   ├── getData.go             # 基础 CRUD 查询
│   ├── autorun.go             # 自动任务 CRUD
│   ├── backup.go              # 备份导入/导出
│   └── countdown.go           # 倒数日 CRUD
├── service/                   # 业务逻辑层
│   ├── schedule.go            # 课表规则引擎
│   └── countdown.go           # 倒数日 scope 过滤
├── router/                    # HTTP 处理层
│   ├── client/                # 客户端 API
│   │   ├── getSchedule.go     # GET /:school/:grade/:class
│   │   ├── putSchedule.go     # PUT /:school/:grade/:class
│   │   ├── getWeather.go      # GET /api/weather/...
│   │   └── misc.go            # WebSocket + 广播
│   └── web/                   # Web 管理后台 API
│       ├── menu_handlers.go   # GET /web/menu
│       ├── config_handlers.go # 配置 CRUD
│       ├── autorun_handlers.go
│       ├── countdown_handlers.go
│       ├── backup_handlers.go
│       ├── compensation_handlers.go
│       ├── helpers.go
│       └── types.go
└── startup/                   # 启动初始化
    ├── init.go                # StartInit()
    ├── config.go              # ReadConfig()
    ├── db.go                  # MigrateDb()
    └── log.go                 # SetLog()
```

## 查找位置

| 任务 | 位置 | 说明 |
|------|------|------|
| API 路由定义 | `main.go` | 所有路由注册 |
| 客户端 API | `router/client/` | 课表、天气、WebSocket |
| 管理端 API | `router/web/` | 配置、备份、调休 |
| 数据库模型 | `model/dbTable/` | GORM 表结构 |
| 数据访问 | `db/` | CRUD 操作 |
| 业务逻辑 | `service/` | 规则引擎 |
| 启动配置 | `startup/` | 初始化流程 |
| OpenAPI 文档 | `AstraServerGo.openapi.json` | API 规范 |

## 约定

### API 约定
- 管理端接口前缀：`/web/*`
- 客户端接口：无前缀（如 `/:school/:grade/:class`）
- 认证方式：BasicAuth（用户名 `ElectronClassSchedule` 或 `AstraSchedule`）
- 响应格式：`status`/`message`/`data` 或 `error`/`detail`

### 数据库约定
- 支持 MySQL 和 SQLite（通过配置切换）
- 使用 GORM 模型（`model/dbTable/*`）
- 写入操作使用 upsert（`ON CONFLICT ... UPDATE ALL`）
- 多表操作使用事务，失败原子回滚

### 配置约定
- 配置文件：`config.toml`（从 `config.template.toml` 复制）
- 敏感信息：`secret.token` 字段
- 数据库类型：`db.type` 字段（`"mysql"` 或 `"sqlite"`）

## 反模式

### 安全相关
- **禁止** 提交 `config.toml`（包含敏感配置）
- **禁止** 在代码中硬编码密码或令牌
- **禁止** 禁用 CORS 保护

### 代码质量
- **禁止** 在 `main.go` 中添加新路由（应提取到 `router/` 目录）
- **禁止** 在 `db/` 中添加业务逻辑（应放在 `service/`）
- **禁止** 使用 `any` 类型（应使用具体类型）
- **禁止** 删除失败测试来"通过"

### 数据库相关
- **禁止** 使用原生 SQL（应使用 GORM）
- **禁止** 跳过事务进行多表操作
- **禁止** 忽略数据库迁移（AutoMigrate）

### API 相关
- **禁止** 修改 API 响应格式（破坏性变更）
- **禁止** 跳过 OpenAPI 文档更新
- **禁止** 在 GET 请求中进行写操作

## 独特风格

### 课表规则引擎
- 实现 4 类规则（调休/作息/课表/组合）
- 按优先级叠加应用：COMPENSATION → TIMETABLE → SCHEDULE → ALL
- 支持多级作用域：ALL → school → school/grade → school/grade/class

### Serverless 适配
- 支持 `serverless` 模式（`run.serverless = true`）
- 禁用 WebSocket（返回 501）
- 适合云函数部署（如腾讯云 SCF）
- `/web/statistic` 端点返回 `serverless` 字段，供前端判断是否显示实时数据

### 版本号系统
- 格式：`YYYYMM.D.N`（年月.日.运行序号）
- 用于 API 缓存（`?version=timestamp` 参数）
- 支持 304 增量同步

### 泛型批量导入
- `db/backup.go` 使用 Go 泛型 + reflect
- 支持跨数据库类型迁移（MySQL ↔ SQLite）
- 失败原子回滚

## 命令

```bash
# 构建
go build

# 运行
./astra_server.exe

# 测试
go test ./...

# 格式化
go fmt ./...

# 依赖整理
go mod tidy
```

## 注意事项

### 构建环境
- 需要 Go 1.26（未发布版本）
- 使用 Go Modules 管理依赖
- 支持 Windows/Linux/macOS

### 数据库配置
- SQLite：无需安装，适合开发和小型部署
- MySQL：需要安装 MySQL 服务器
- 配置文件：`config.toml`

### 认证机制
- 用户名：`ElectronClassSchedule`（旧版）或 `AstraSchedule`（新版）
- 密码：从 `config.toml` 的 `secret.token` 获取
- 只有写操作（PUT/POST/DELETE）需要认证

### WebSocket
- 地址：`ws://{host}/ws/{school}/{grade}/{class_number}`
- 消息类型：`SyncConfig`（配置同步广播）
- 心跳：25 秒间隔 ping
- 重连：指数退避算法，初始 1 秒，最大 30 秒

### 日志配置
- 库：Logrus
- 调试模式：`log.debug = true` 时日志级别为 Trace
- 输出：控制台（开发）或文件（生产）

### 天气 API
- 缓存：10 分钟 TTL
- 请求库：Resty
- 数据源：和风天气 API
