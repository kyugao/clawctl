# clawctl Backend 版本管理与目录结构说明

## 1. 整体目录结构

```
~/.clawctl/
├── config.json          # clawctl 配置 (实例列表)
├── instances/           # 实例工作目录
│   ├── <instance-name>/
│   │   ├── agent_type      # 标识agent类型
│   │   ├── config.json      # picoclaw配置
│   │   ├── workspace/       # 工作区
│   │   ├── skills/          # 技能目录
│   │   ├── .gateway.log     # 日志文件
│   │   └── .*.pid          # PID文件 (picoclaw/hermes)
│   └── ...
├── trash/              # 回收站 (软删除)
└── claw_release/       # 版本安装目录
    ├── picoclaw/
    │   └── <version>/
    │       ├── picoclaw
    │       ├── picoclaw-launcher
    │       └── picoclaw-launcher-tui
    ├── zero/
    │   └── <version>/
    │       └── zeroclaw
    └── hermes/
        └── <version>/
            ├── hermes        # wrapper脚本
            ├── venv/         # Python虚拟环境
            └── src/          # hermes-agent源码
```

## 2. Backend 接口定义

```go
type Backend interface {
    Repo() string           // GitHub仓库 (如 "sipeed/picoclaw")
    BinaryNames() []string // 发布的二进制文件列表
    GatewayBinary() string  // 启动gateway的二进制名
    IsRunning(workDir string) (int, bool, error)  // 检测运行状态
    StatusDetail(workDir string) (*StatusDetail, error)  // 详细信息
    Start(inst InstanceInfo, binaryPath string) error
    Stop(inst InstanceInfo) error
    InitWorkDir(inst InstanceInfo) error
    ResetWorkspace(inst InstanceInfo) error
}
```

## 3. 各 Backend 详细对比

### 3.1 picoclaw

| 属性 | 值 |
|------|-----|
| GitHub Repo | `sipeed/picoclaw` |
| 二进制文件 | `picoclaw`, `picoclaw-launcher`, `picoclaw-launcher-tui` |
| Gateway 二进制 | `picoclaw-launcher` |
| 启动参数 | `-console -no-browser -port <N> <config.json>` |
| 进程检测方式 | **ps 匹配**: `picoclaw-launcher` + `workDir` |
| PID 文件 | `.picoclaw.pid` (用于检测，但实际用ps匹配) |
| 默认端口 | 18790 |
| Reset 支持 | ✅ 模板复制 |

**安装逻辑**: GitHub Release → 下载压缩包 → 解压 → 移动到 `~/.clawctl/claw_release/picoclaw/<version>/`

**目录结构**:
```
picoclaw/<version>/
├── picoclaw
├── picoclaw-launcher
└── picoclaw-launcher-tui
```

---

### 3.2 zeroclaw (zero)

| 属性 | 值 |
|------|-----|
| GitHub Repo | `zeroclaw-labs/zeroclaw` |
| 二进制文件 | `zeroclaw` |
| Gateway 二进制 | `zeroclaw` |
| 启动参数 | `--config-dir <workDir> daemon -p <port>` |
| 进程检测方式 | **ps 匹配**: `zeroclaw` + `--config-dir` + `workDir` |
| PID 文件 | ❌ 无 |
| 默认端口 | 18792 |
| Reset 支持 | ❌ |

**特殊行为**: zeroclaw 启动后会 fork，父进程退出。启动时等待父进程退出，然后通过 ps 匹配验证 daemon 进程是否运行。

**目录结构**:
```
zero/<version>/
└── zeroclaw
```

---

### 3.3 hermes

| 属性 | 值 |
|------|-----|
| GitHub Repo | `NousResearch/hermes-agent` |
| 二进制文件 | N/A (Python包) |
| Gateway 二进制 | `hermes` (wrapper脚本) |
| 启动参数 | `gateway run` |
| 进程检测方式 | **PID 文件**: `.hermes.pid` |
| PID 文件位置 | `<workDir>/.hermes.pid` |
| 默认端口 | 8642 |
| Reset 支持 | ❌ |

**安装逻辑**:
1. `git clone --depth 1` 克隆源码到临时目录
2. `git fetch --tags && git checkout <tag>`
3. 复制源码到 `~/.clawctl/claw_release/hermes/<version>/src/`
4. `python3 -m venv` 创建虚拟环境
5. `uv pip install .` 安装 hermes-agent 包
6. 创建 wrapper 脚本设置 `HERMES_HOME` 和 `PYTHONPATH`

**目录结构**:
```
hermes/<version>/
├── hermes           # wrapper脚本
├── venv/            # Python虚拟环境
│   └── bin/
│       ├── hermes
│       ├── hermes-agent
│       └── ...
└── src/             # hermes-agent 源码副本
    ├── hermes_cli/
    ├── agent/
    └── ...
```

**wrapper 脚本内容**:
```bash
#!/bin/bash
export HERMES_HOME="${HERMES_HOME:-/path/to/version}"
export PYTHONPATH="/path/to/version/src:${PYTHONPATH:-}"
exec "/path/to/venv/bin/hermes" "$@"
```

---

### 3.4 openclaw

| 属性 | 值 |
|------|-----|
| GitHub Repo | `sipeed/openclaw` |
| 二进制文件 | `openclaw`, `openclaw-launcher` |
| Gateway 二进制 | `openclaw-launcher` |
| 支持状态 | ❌ **未实现** |

---

## 4. 版本管理流程

### 4.1 版本解析 (`ResolveVersion`)

```go
switch version {
case "latest":
    return FetchLatestTag(repo)  // GitHub API 获取最新 tag
case "nightly":
    return "nightly"             // 直接使用 "nightly"
default:
    return version               // 直接使用提供的版本
}
```

### 4.2 安装流程 (`InstallVersion`)

```
1. ResolveVersion (latest/nightly → 实际tag)
2. ListLocalVersions 检查是否已安装
3. Backend.Repo() 获取 GitHub 仓库
4. DownloadAndExtractRelease (GitHub API → 下载 → 解压)
5. moveAndFilterBinaries (按 BinaryNames 过滤)
6. 移动到 ~/.clawctl/claw_release/<type>/<tag>/
```

### 4.3 特殊处理

**Hermes 特殊处理**:
```go
if clawType == "hermes" {
    return InstallHermesVersion(tag)  // 专用安装函数
}
```

**Picoclaw/Zeroclaw**: 使用通用 GitHub Release 下载流程

---

## 5. 进程检测对比

| Backend | 检测方式 | 检测模式 |
|---------|---------|---------|
| picoclaw | ps 匹配 | `picoclaw-launcher` + `workDir` |
| zeroclaw | ps 匹配 | `zeroclaw` + `--config-dir` + `workDir` |
| hermes | PID 文件 | 启动时写入 `.hermes.pid`，检测时读取 |

**为什么 hermes 用 PID 文件?**

hermes 是 Python 程序，运行命令是:
```
python ... hermes gateway run
```
其中 `HERMES_HOME` 是环境变量，不会在 `ps` 输出中显示。wrapper 脚本将其设置为 `workDir`，但 ps 输出的命令参数中不包含 `HERMES_HOME` 的值，所以无法通过 ps 匹配可靠检测。

---

## 6. 启动命令对比

| Backend | 完整启动命令 |
|---------|------------|
| picoclaw | `<binary> -console -no-browser -port 18790 /path/to/config.json` |
| zeroclaw | `<binary> --config-dir /path/to/workdir daemon -p 18792` |
| hermes | `<wrapper> gateway run` (HERMES_HOME 由 wrapper 设置) |

---

## 7. 默认端口

```go
var defaultPorts = map[string]int{
    "picoclaw": 18790,
    "openclaw": 18791,
    "zero":     18792,
    "hermes":   8642,  // hermes 硬编码在 Backend 接口中
}
```
