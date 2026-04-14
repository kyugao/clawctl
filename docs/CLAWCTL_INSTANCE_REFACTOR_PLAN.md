# clawctl Instance 模型与 Backend 配置职责重构方案

## 1. 背景

当前 `clawctl` 的实例模型和 backend 启动逻辑已经能支持基础的 create/start/stop/status 流程，但随着 backend 类型变多，现有结构开始暴露出几个系统性问题：

1. 实例配置模型过于扁平。`internal/config/config.go` 中的 `Instance` 既承担通用配置，又开始容纳 backend 私有信息，扩展性不足。
2. 端口分配逻辑放在全局静态表中。`defaultPorts` 适合单端口服务，但不适合 `picoclaw` 这种一个实例含多个端口的模型。
3. manager 层直接读取实例字段。这样会把底层模型绑定死，后续很难引入 backend-specific runtime data 或多态实例。
4. 运行时信息回写不统一。token、派生端口、迁移状态等信息没有稳定的持久化模型和 merge 规则。
5. 旧实例迁移路径不明确。新结构一旦引入，如果没有 migration 方案，很容易让已有实例停在半新半旧状态。

其中最典型的问题是 `picoclaw` 端口冲突：

1. `picoclaw-launcher` 需要一个 launcher port。
2. `picoclaw gateway` 还需要一个独立的 gateway port。
3. 当前 `clawctl` 只持久化了一个 `Port` 字段，并把它同时用于 launcher 启动参数和内部 gateway 默认端口推导，导致冲突。

这说明我们需要的不是一次局部修补，而是一次实例模型、端口职责、配置持久化和启动编排的系统性重构。

---

## 2. 目标

本方案的目标不是只修 `picoclaw`，而是建立一套可长期演进的实例与 backend 配置框架。

### 2.1 核心目标

1. 引入可扩展的实例模型，支持 backend-specific 扩展信息。
2. 将端口分配和实例初始化职责下沉到 backend。
3. 统一 `create/start/restart/info/list/status` 对实例的访问方式。
4. 为运行时信息采集和回写建立稳定模型。
5. 为旧实例提供可控、渐进的迁移机制。

### 2.2 非目标

本阶段不追求：

1. 一次性重写所有 backend 行为实现。
2. 引入复杂的数据库或状态服务，配置仍然基于 JSON 文件。
3. 让每个 backend 都暴露完全不同的配置文件格式，外层 `clawctl` 仍保持统一视图。

---

## 3. 现状问题分析

### 3.1 实例结构与运行时模型耦合

当前实例定义是一个扁平结构体：

```go
type Instance struct {
    ClawType  string         `json:"claw_type"`
    WorkDir   string         `json:"work_dir"`
    Port      int            `json:"port"`
    Version   string         `json:"version"`
    CreatedAt string         `json:"created_at"`
    Info      map[string]any `json:"info,omitempty"`
}
```

它的问题在于：

1. 结构体既承担持久化格式，又被 manager/backend 当运行时对象直接使用。
2. 所有调用方都直接访问字段，导致底层模型很难替换。
3. `Info` 没有结构约束，随着 backend 特性增多会越来越散。

### 3.2 全局默认端口模型不适合多端口 backend

当前 `defaultPorts` 是按 `claw_type -> port` 的一对一映射。这适合 `zero`、`hermes` 这样的单主服务端口模型，但不适合：

1. 一个实例有 launcher port 和 gateway port 的 backend。
2. 后续可能存在 HTTP port、admin port、debug port 的 backend。
3. 端口需要按实例状态修复或补齐的 backend。

### 3.3 manager 层承担了过多 backend 细节

manager 目前广泛依赖实例结构体字段，例如：

1. 直接读取 `inst.ClawType` 决定 backend。
2. 直接读取 `inst.Port` 展示和启动。
3. 直接覆盖 `inst.Info`，而不是按规则 merge。

这会导致：

1. manager 层对持久化结构形成强耦合。
2. 一旦引入接口或 backend-specific 扩展，改动面很大。
3. `start` 和 `restart` 等命令容易各自长出一套后处理逻辑。

### 3.4 缺少统一的 reconcile 机制

当前流程里没有“启动前修复实例”的标准步骤。对于以下场景不够友好：

1. 实例创建于旧版本，缺少新字段。
2. `config.json` 存在，但内容和配置记录不一致。
3. backend 需要在启动前补写衍生配置。

---

## 4. 重构原则

### 4.1 持久化层与运行时层分离

JSON 落盘格式要稳定，运行时对象要可扩展，两者不再共享同一个裸结构体。

### 4.2 backend 对自身配置负责

每个 backend 最清楚自己需要哪些端口、目录、附属配置和迁移规则，因此：

1. 端口分配应由 backend 决定。
2. 实例创建时的扩展字段应由 backend 初始化。
3. 启动前修复逻辑应由 backend 定义。

### 4.3 manager 只做编排，不持有 backend 私有规则

manager 应负责：

1. 解析命令。
2. 选择 backend。
3. 加载和保存配置。
4. 统一执行 create/start/restart 的编排流程。

manager 不应负责解释 `picoclaw` 的 `gateway_port`、`dashboard_token` 等细节。

### 4.4 迁移必须可渐进、可观测

旧实例迁移不能依赖一次性离线脚本，也不能让读命令产生写副作用。迁移应尽量在写路径或启动前 reconcile 阶段完成，并保留版本痕迹。

---

## 5. 总体架构

### 5.1 统一的持久化模型：`InstanceRecord`

`InstanceRecord` 是 JSON 文件的稳定表示，用于 `Load/Save`。

```go
type InstanceRecord struct {
    Name      string         `json:"name"`
    ClawType  string         `json:"claw_type"`
    WorkDir   string         `json:"work_dir"`
    Port      int            `json:"port"`
    Version   string         `json:"version"`
    CreatedAt string         `json:"created_at"`
    Info      map[string]any `json:"info,omitempty"`
}
```

约束：

1. `Name` 显式落盘，不再只依赖 map key。
2. `Port` 仍保留为主服务端口。
3. backend-specific 数据进入 `Info`。

### 5.2 统一的运行时模型：`Instance`

`Instance` 是 manager/backend 在内存中使用的接口。

```go
type Instance interface {
    GetName() string
    GetClawType() string
    GetWorkDir() string
    GetPort() int
    GetVersion() string
    GetInfo() map[string]any
    AsRecord() InstanceRecord
}
```

设计原则：

1. manager 层只依赖接口。
2. backend 可通过 helper 访问扩展字段。
3. 持久化时统一走 `AsRecord()`，不直接 marshal 接口具体实现。

### 5.3 config 包内部保留具体实例类型

具体实现类型只在 `config` 包内部暴露其构造和转换逻辑，例如：

1. `baseInstance`
2. `picoclawInstance`
3. `zeroclawInstance`
4. `openclawInstance`
5. `hermesInstance`

这些类型负责：

1. 实现 `Instance` 接口。
2. 管理 backend-specific info helper。
3. 承担 record 与 runtime object 的互转。

### 5.4 `Config` 容器以接口形式暴露实例

建议调整为：

```go
type Config struct {
    Instances map[string]Instance `json:"-"`
    Default   string              `json:"default"`
}
```

但 `Load/Save` 在内部以 `InstanceRecord` 进行编解码，避免直接对接口做 JSON 反序列化。

---

## 6. 配置与扩展字段规范

### 6.1 `Port` 的语义

`Instance.Port` 始终代表实例的主服务端口。

各 backend 语义约定：

1. `zero`：daemon 主端口。
2. `hermes`：gateway 主端口。
3. `openclaw`：预留主端口。
4. `picoclaw`：launcher port。

### 6.2 `Info` 的命名空间

为了避免 `Info` 变成无组织字典，约定分成三个一级命名空间：

1. `ports.*`
2. `runtime.*`
3. `meta.*`

推荐的 JSON 结构：

```json
{
  "info": {
    "ports": {
      "launcher": 18800,
      "gateway": 18791
    },
    "runtime": {
      "dashboard_token": "xxx",
      "dashboard_token_saved_at": "2026-04-14T12:00:00Z"
    },
    "meta": {
      "schema_version": 2
    }
  }
}
```

### 6.3 通用 helper

在 `config` 包中提供通用 helper，降低业务代码中到处做 `map[string]any` 断言的噪音，例如：

1. `GetInfoInt(inst, "ports.gateway")`
2. `GetInfoString(inst, "runtime.dashboard_token")`
3. `SetInfoValue(inst, "runtime.dashboard_token", value)`
4. `MergeInfo(inst, map[string]any)`

---

## 7. Backend 职责模型

### 7.1 保留进程控制接口：`Backend`

现有 `Backend` 继续负责进程操作：

1. `Repo`
2. `BinaryNames`
3. `GatewayBinary`
4. `IsRunning`
5. `StatusDetail`
6. `Start`
7. `Stop`
8. `ResetWorkspace`

### 7.2 新增实例配置接口：`InstanceConfigurator`

新增一个独立接口，负责实例创建、修复、runtime 信息采集：

```go
type InstanceConfigurator interface {
    AllocateInstance(ctx context.Context, cfg *config.Config, name string, explicit map[string]int, version, workDir string) (config.Instance, error)
    PrepareWorkDir(inst config.Instance) error
    ReconcileInstance(ctx context.Context, cfg *config.Config, inst config.Instance) (config.Instance, bool, error)
    GatherRuntimeInfo(inst config.Instance) map[string]any
}
```

职责划分如下：

1. `AllocateInstance`
负责创建实例时的端口分配和扩展字段初始化。

2. `PrepareWorkDir`
负责首次创建目录、写初始配置文件。

3. `ReconcileInstance`
负责启动前修复实例配置和 backend 文件。

4. `GatherRuntimeInfo`
负责启动成功后采集 token、派生地址等运行时信息。

### 7.3 backend 注册模型

每个 backend 在注册时同时提供：

1. `Backend`
2. `InstanceConfigurator`

可通过统一 registry 暴露，例如：

```go
type BackendSpec struct {
    Backend      Backend
    Configurator InstanceConfigurator
}
```

这样 manager 层拿到的是一份完整能力描述，而不是散落的对象和类型断言。

---

## 8. manager 层重构方案

### 8.1 manager 全面改为依赖 `config.Instance` 接口

以下文件都需要从字段直取改为 getter/helper：

1. `internal/manager/cmd_create.go`
2. `internal/manager/cmd_start.go`
3. `internal/manager/cmd_restart.go`
4. `internal/manager/cmd_info.go`
5. `internal/manager/cmd_list.go`
6. `internal/manager/cmd_status.go`
7. `internal/manager/process.go`

### 8.2 统一启动编排入口

建议将 `GatewayRunner.Start()` 升级为统一编排入口：

```go
func (r *GatewayRunner) Start(ctx context.Context) error {
    inst, changed, err := r.Configurator.ReconcileInstance(ctx, r.Config, r.Instance)
    if err != nil {
        return err
    }
    if changed {
        if err := saveInstance(inst); err != nil {
            return err
        }
    }

    if err := r.Backend.Start(inst, r.BinaryPath); err != nil {
        return err
    }

    runtimeInfo := r.Configurator.GatherRuntimeInfo(inst)
    if len(runtimeInfo) > 0 {
        if err := mergeRuntimeInfo(inst, runtimeInfo); err != nil {
            return err
        }
    }
    return nil
}
```

这样：

1. `start` 和 `restart` 共享同一套逻辑。
2. runtime info 回写统一走 merge。
3. backend-specific 行为不需要在命令层做类型断言。

### 8.3 `info/list/status` 的展示策略

manager 展示层需要支持：

1. 通用字段总是展示。
2. 若 backend 提供额外可读信息，则显示在扩展区块。

例如 `picoclaw`：

1. launcher port
2. gateway port
3. dashboard token
4. token saved time

不要求所有 backend 都有扩展区块，但输出结构需要预留。

---

## 9. picoclaw 专项设计

### 9.1 实例字段规则

对于 `picoclaw`：

1. `Instance.Port` 表示 launcher port。
2. `info.ports.launcher` 与 `Instance.Port` 保持一致，作为冗余可读字段。
3. `info.ports.gateway` 表示内部 gateway port。
4. `info.runtime.dashboard_token` 表示本次成功启动后采集到的 dashboard token。

### 9.2 端口分配规则

创建 `picoclaw` 实例时：

1. 如果用户指定 `--port`，它只作用于 launcher port。
2. launcher port 需要检查：
   - 当前系统监听冲突。
   - 配置文件中其它实例保留冲突。
3. gateway port 永远独立分配。
4. gateway port 同样需要检查：
   - 当前系统监听冲突。
   - 配置文件中其它实例保留冲突。

### 9.3 `PrepareWorkDir`

`PrepareWorkDir` 必须保证：

1. 创建标准目录结构。
2. 创建或更新 `config.json`。
3. `config.json` 中的 gateway 配置与 `info.ports.gateway` 一致。

注意：

1. 不能只在文件不存在时创建。
2. 需要支持幂等更新。

### 9.4 `ReconcileInstance`

在每次 `start/restart` 前：

1. 如果实例缺少 `info.ports.gateway`，则为其补齐。
2. 如果 `config.json` 缺少 gateway 配置，则补写。
3. 如果 `config.json` 中的 gateway 配置与实例记录不一致，则以实例记录为准修复。
4. 如果实例来自旧版本且主端口处于历史错误值，应根据迁移规则重新修正。

### 9.5 `GatherRuntimeInfo`

`GatherRuntimeInfo` 负责从 backend 日志或标准输出中提取：

1. `runtime.dashboard_token`
2. `runtime.dashboard_token_saved_at`

要求：

1. 不覆盖 `info.ports.*`
2. 不清空旧 `runtime.*`
3. token 为空或 `(empty)` 时不回写

---

## 10. 其他 backend 的适配策略

### 10.1 zeroclaw

`zero` 当前仍是单主端口模型，可作为新架构中的简单 backend：

1. `AllocateInstance` 只分配主端口。
2. `PrepareWorkDir` 只创建目录结构。
3. `ReconcileInstance` 默认仅补 `meta.schema_version`。
4. `Info` 可为空或仅包含 `meta.*`。

### 10.2 hermes

`hermes` 目前主端口固定，适合先按单端口 backend 接入新框架：

1. `Port` 仍表示主 gateway 端口。
2. 若未来出现更多 runtime 数据，可通过 `runtime.*` 扩展。

### 10.3 openclaw

`openclaw` 目前尚未完整实现，但仍建议先接入统一实例模型：

1. 保持单端口占位模型。
2. 保持 `PrepareWorkDir` 和 `ReconcileInstance` 的默认实现。
3. 后续实现时不再需要再次改造底层实例框架。

---

## 11. 迁移策略

### 11.1 迁移原则

迁移必须满足：

1. 读命令尽量无副作用。
2. 写路径和启动路径允许渐进修复。
3. 每个实例都能被识别其 schema 版本。

### 11.2 schema version

建议在 `info.meta.schema_version` 中记录实例配置 schema 版本。

建议版本：

1. `1`：旧扁平实例模型。
2. `2`：引入 `name` 落盘、`Info` 命名空间、backend configurator 的新模型。

### 11.3 旧实例加载规则

加载旧实例时：

1. 若 `name` 缺失，则由 config map key 回填。
2. 若 `info` 缺失，则初始化为空 map。
3. 若 `meta.schema_version` 缺失，则视为旧版本实例。
4. 仅在内存对象中标记待迁移，不在纯读命令中自动保存。

### 11.4 `picoclaw` 旧实例迁移

对于旧 `picoclaw` 实例：

1. 若 `info.ports.gateway` 缺失，不在 `info/list/status` 时补写。
2. 在 `start/restart` 前的 reconcile 阶段补齐并落盘。
3. 若 `config.json` 已存在但没有 gateway 配置，则在 reconcile 阶段修复。
4. 若旧实例的 `Port` 使用了历史错误默认值，需按以下逻辑处理：
   - 该端口如果仍被用作 launcher 且未冲突，可保留为 launcher port。
   - gateway port 永远重新独立分配或从新规则中恢复。
   - 最终以 `info.ports.gateway` 作为唯一事实来源。

### 11.5 `UpdateInstanceInfo` 的替换

旧的整块覆盖式 `UpdateInstanceInfo(name, info)` 需要替换为更安全的 API：

1. `UpdateInstance(name, mutator func(Instance) error) error`
2. `MergeInstanceInfo(name, patch map[string]any) error`

要求：

1. merge 而不是覆盖。
2. 对不存在的命名空间自动初始化。
3. 写入原子化。

---

## 12. 关键模块改造清单

### 12.1 `internal/config/config.go`

需要完成：

1. 引入 `InstanceRecord`
2. 引入 `Instance` 接口
3. 引入 record 与 runtime instance 的 encode/decode
4. 提供 `GetInfo` 和 info helper
5. 提供带 mutator 的原子更新入口

### 12.2 `internal/backend/backend.go`

需要完成：

1. 保留现有 `Backend`
2. 新增 `InstanceConfigurator`
3. 提供 `BackendSpec`
4. 调整 registry 返回结构

### 12.3 `internal/backend/picoclaw.go`

需要完成：

1. 实现 `AllocateInstance`
2. 实现 launcher/gateway 双端口分配
3. 实现 `PrepareWorkDir`
4. 实现 `ReconcileInstance`
5. 实现 `GatherRuntimeInfo`

### 12.4 `internal/backend/zeroclaw.go`

需要完成：

1. 实现单端口 `AllocateInstance`
2. 实现默认 `PrepareWorkDir`
3. 实现轻量 `ReconcileInstance`

### 12.5 `internal/backend/hermes.go`

需要完成：

1. 接入统一实例分配流程
2. 保持现有启动逻辑
3. 为 future runtime info 扩展预留结构

### 12.6 `internal/backend/openclaw.go`

需要完成：

1. 先接入统一模型
2. 保持 not implemented 行为不变

### 12.7 `internal/manager/process.go`

需要完成：

1. 引入 `BackendSpec` 和 configurator
2. 把 start/restart 公共流程收口到 runner
3. 把 reconcile 和 runtime info merge 放入统一流程

### 12.8 manager 命令层

以下命令需统一改为依赖接口和 helper：

1. `cmd_create.go`
2. `cmd_start.go`
3. `cmd_restart.go`
4. `cmd_info.go`
5. `cmd_list.go`
6. `cmd_status.go`
7. `cmd_reset.go`
8. `cmd_stop.go`
9. `cmd_delete.go`

---

## 13. 分阶段实施计划

### Phase 1：实例模型重构

目标：

1. 引入 `InstanceRecord + Instance interface`
2. 将 `Load/Save` 切到新模型
3. manager 全部改成 getter/helper 访问

要求：

1. 行为保持不变
2. 不在本阶段引入 pico 双端口逻辑

产出：

1. 编译通过
2. 基础命令行为不回归
3. 为后续 configurator 下沉打好底座

### Phase 2：backend 配置职责下沉

目标：

1. 引入 `InstanceConfigurator`
2. `create` 改为通过 backend 分配实例
3. `defaultPorts` 退役

要求：

1. `zero/hermes/openclaw/picoclaw` 都走统一分配流程
2. 仍保持单端口兼容逻辑

产出：

1. 创建流程结构稳定
2. backend 对自己的端口策略负责

### Phase 3：picoclaw 双端口与 reconcile

目标：

1. `picoclaw` 创建时分配 launcher/gateway 双端口
2. `PrepareWorkDir` 写入正确的 gateway 配置
3. `start/restart` 前自动 reconcile 旧实例

产出：

1. 新实例不再出现端口冲突
2. 旧实例可在启动时自动修复

### Phase 4：运行时信息回写与展示

目标：

1. token 回写统一走 runtime info merge
2. `info/status/list` 展示 backend 扩展信息
3. `start/restart` 共享同一套 runtime 更新路径

产出：

1. 运行时可观测性完成闭环
2. 扩展字段不再被整块覆盖

### Phase 5：迁移验证与测试补齐

目标：

1. 补 migration 相关单元测试
2. 补多实例端口分配测试
3. 补 pico reconcile 测试
4. 补 start/restart runtime info merge 测试

产出：

1. 方案具备长期维护基础
2. 旧实例升级路径清晰且可验证

---

## 14. 各阶段验收标准

### Phase 1 验收

1. `go test ./...` 通过
2. `create/list/info/status/start/stop/restart` 编译通过
3. manager 层不再直接依赖底层实例字段

### Phase 2 验收

1. 所有 backend 都能通过统一工厂创建实例
2. `defaultPorts` 不再作为创建来源
3. 新增 backend 不需要改 `config.NewInstance` 分支

### Phase 3 验收

1. 连续创建多个 `picoclaw` 实例时 launcher/gateway 端口均不冲突
2. 已存在的旧 pico 实例首次 `start` 能自动修复
3. `config.json` 与实例记录保持一致

### Phase 4 验收

1. `clawctl info <pico>` 能显示 launcher/gateway/token
2. `restart` 后 runtime 信息能正确刷新
3. runtime info 不会覆盖 ports/meta 命名空间

### Phase 5 验收

1. migration 相关测试通过
2. 多实例并发创建与启动测试通过
3. 关键路径均有回归保障

---

## 15. 风险与注意事项

### 15.1 接口切换的改动面较大

将 manager 层从字段直取切换为接口访问，会影响多个命令和流程文件。必须在 Phase 1 里一次性清理干净，避免一半字段一半 getter 的中间状态长期存在。

### 15.2 `Info` 虽灵活，但必须加 helper

如果没有统一 helper，即使做了命名空间约定，代码里仍会充满 `map[string]any` 断言。必须同步引入读取、写入、merge helper。

### 15.3 迁移不要让读命令产生写副作用

`info/list/status` 应尽量只读取和展示。真正的修复写入应集中在：

1. `create`
2. `start`
3. `restart`
4. 显式 update/migrate 命令

### 15.4 `picoclaw` 的外部源码改动需要晚于 Phase 3 设计确认

如果 `picoclaw` 侧需要通过 env 或配置文件支持 gateway port override，必须在 `clawctl` 侧的实例模型和 reconcile 规则稳定后再落地，否则很容易来回修改契约。

---

## 16. 推荐实施顺序

推荐严格按以下顺序推进：

1. 先完成 Phase 1，把实例模型和 manager 访问方式抽象稳定。
2. 再完成 Phase 2，把实例创建和配置职责下沉到 backend。
3. 然后集中处理 `picoclaw` 的双端口、配置修复和 runtime 信息。
4. 最后补展示层和测试。

原因：

1. 先换底座，再修业务问题，返工最少。
2. 如果先局部修 `picoclaw`，后面切实例模型时还会再改一轮。

---

## 17. 当前建议的下一步

建议立即从 Phase 1 开始，目标是：

1. 在不改变现有行为的前提下，引入新的实例抽象层。
2. 清理 manager 层对实例字段的直接依赖。
3. 为后续 `picoclaw` 双端口改造提供稳定底座。

完成 Phase 1 后，再进入 Phase 2 和 Phase 3，集中解决端口冲突和迁移问题。
