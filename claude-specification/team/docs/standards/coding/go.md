# 编码规范 · Go 专项

> 本文件是 `coding/base.md` 的 Go 语言扩展，仅记录 Go 特有约定。
> 通用规范以 base.md 为准，本文件优先级高于 base.md 中的冲突条目。

---

## 1. 代码风格

### 1.1 格式化

- 所有代码必须通过 `gofmt` / `goimports` 格式化（CI 强制检查）
- import 分组顺序：标准库 → 第三方库 → 内部包，每组之间空行隔开
- 使用 `golangci-lint` 作为 lint 工具

> [团队填写] golangci-lint 配置文件位置及启用的规则集：
>
> ```
> 示例：配置文件位于 team-standards/.golangci.yml，各项目软链接引用
> ```

### 1.2 命名约定

- 包名：小写单词，不使用下划线或驼峰，简短且有意义（如 `user`，不用 `userService`）
- 接口名：单方法接口以行为命名（`Reader`、`Writer`）；多方法接口以 `-er` 结尾或明确业务名
- 常量：使用 MixedCaps，不用 ALL_CAPS（Go 惯例）
- 错误变量：以 `Err` 前缀（`ErrNotFound`）；错误类型以 `Error` 后缀（`NotFoundError`）

---

## 2. 错误处理

### 2.1 错误返回

- 函数使用多返回值传递错误，最后一个返回值为 `error`
- 禁止使用 `panic` 处理业务逻辑错误（仅用于不可恢复的程序错误）
- 使用 `fmt.Errorf("...: %w", err)` 包装错误，保留错误链
- 使用 `errors.Is()` / `errors.As()` 判断错误类型，不直接比较错误字符串

### 2.2 自定义错误

```go
// 推荐：定义语义化错误
var ErrUserNotFound = errors.New("user not found")

// 推荐：携带上下文的错误类型
type ValidationError struct {
    Field   string
    Message string
}
func (e *ValidationError) Error() string {
    return fmt.Sprintf("validation failed on %s: %s", e.Field, e.Message)
}
```

### 2.3 错误处理位置

- 调用方负责处理错误，不将未处理的错误继续向上传递（除非是 propagation 场景）
- 在 handler/entry point 层统一记录 ERROR 日志，避免中间层重复记录

---

## 3. 并发

### 3.1 Goroutine 管理

- 启动 goroutine 必须确保其生命周期可控（能被取消或等待结束）
- 使用 `context.Context` 控制 goroutine 取消，第一个参数统一为 `ctx`
- goroutine 泄露是严重 bug，必须在 code review 中检查

```go
// 推荐：使用 errgroup 管理 goroutine 组
g, ctx := errgroup.WithContext(ctx)
g.Go(func() error { return doWork(ctx) })
if err := g.Wait(); err != nil { ... }
```

### 3.2 共享状态

- 优先使用 channel 通信，而非共享内存
- 必须共享内存时，使用 `sync.Mutex` 或 `sync.RWMutex` 保护
- 禁止在持有锁的情况下调用外部 IO（可能死锁）
- 使用 `-race` flag 运行测试（CI 中启用）

### 3.3 Channel 使用

- 明确 channel 的 owner（负责关闭），close 只由 owner 执行
- 禁止关闭已关闭的 channel

---

## 4. 接口设计

### 4.1 接口定义位置

- 接口定义在**使用方**，而非实现方（Go 惯例）
- 小接口优先，按需组合（`io.Reader` + `io.Writer` → `io.ReadWriter`）

### 4.2 接口大小

- 单方法接口是最佳实践，便于 mock 和测试
- 超过 5 个方法的接口应重新审视是否职责过重

---

## 5. 项目结构

> [团队填写] 定义标准目录结构，例如：

```
service/
├── cmd/                # main 入口
│   └── server/
├── internal/           # 内部包（不对外暴露）
│   ├── domain/         # 业务实体和接口定义
│   ├── service/        # 业务逻辑
│   ├── repository/     # 数据访问层
│   └── handler/        # HTTP/gRPC handler
├── pkg/                # 可对外暴露的工具包
├── proto/              # protobuf 定义
├── migrations/         # 数据库迁移脚本
└── docs/               # 文档
```

---

## 6. 依赖管理

- 使用 Go Modules，`go.mod` 和 `go.sum` 必须提交到仓库
- 禁止在 `replace` 指令中使用本地路径提交（仅开发时临时使用）
- 定期运行 `go mod tidy` 清理无用依赖

> [团队填写] 是否使用私有 module proxy：
>
> ```
> GOPROXY=<internal-proxy>,direct
> GONOSUMCHECK=<internal-domain>
> ```

---

## 7. 测试

- 单元测试文件与被测文件同目录，命名为 `<file>_test.go`
- 使用 `testify` 断言库（`github.com/stretchr/testify`）
- Table-driven test 是 Go 惯例，复杂场景优先使用
- 使用 `t.Helper()` 在辅助函数中标记，使错误定位准确

> 详细测试策略参见 `docs/standards/testing.md`
