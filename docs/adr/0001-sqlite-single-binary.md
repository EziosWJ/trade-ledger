# SQLite 单二进制部署

V1 使用 SQLite 作为唯一数据库，前端 `web/dist` 经 `go:embed` 打进 Go 单二进制，`./trade-ledger` 一键启动，无外部依赖。多门店 / 多用户并发成为瓶颈时再迁移到 Postgres。
