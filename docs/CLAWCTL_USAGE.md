# ClawCtl 使用说明

ClawCtl 是 Claw 集群控制工具，用于管理 picoclaw 和 zeroclaw 实例。

## 快速开始

```bash
# 创建新实例
clawctl create my-pico --type picoclaw

# 启动实例
clawctl start my-pico

# 查看状态
clawctl list

# 停止实例
clawctl stop my-pico
```

## 实例管理命令

### clawctl create

创建新实例。

```bash
clawctl create <instance> --type <picoclaw|zero> [flags]
```

**参数：**
- `<instance>` 实例名称

**选项：**
- `--type <type>` 实例类型（必填，可选值：`picoclaw`、`zero`）
- `--port <port>` 网关端口（可选，默认根据类型不同）
- `--version <version>` 版本号（可选，默认 `latest`）
- `--dir <path>` 工作目录（可选）

**示例：**
```bash
clawctl create my-pico --type picoclaw
clawctl create my-zero --type zero --port 18792 --version v0.6.9
```

---

### clawctl list

列出所有实例。

```bash
clawctl list
```

显示实例名称、类型、端口、版本、状态和工作目录。带 `*` 标记的是默认实例。

**示例输出：**
```
NAME         TYPE      PORT   VERSION  STATUS              WORK_DIR
* my-pico    picoclaw  18790  latest   running (PID 1234)  ~/.clawctl/instances/my-pico
  my-zero    zero      18792  v0.6.9  stopped            ~/.clawctl/instances/my-zero
```

---

### clawctl info

显示实例详细信息。

```bash
clawctl info [instance]
```

如果不指定实例名，显示所有实例信息。

**示例：**
```bash
clawctl info my-pico
```

---

### clawctl status

显示实例运行状态。

```bash
clawctl status <instance>
```

**示例：**
```bash
clawctl status my-pico
```

**输出示例：**
```
Instance: my-pico
  Type:    picoclaw
  Version: latest
  Port:    18790
  WorkDir: ~/.clawctl/instances/my-pico
  Binary:  picoclaw
  Status:  running (PID 1234)
  Log:     ~/.clawctl/instances/my-pico/.gateway.log
```

---

### clawctl start

启动实例。

```bash
clawctl start <instance>
```

**示例：**
```bash
clawctl start my-pico
```

---

### clawctl stop

停止实例。

```bash
clawctl stop <instance>
```

**示例：**
```bash
clawctl stop my-pico
```

---

### clawctl restart

重启实例（先停止后启动）。

```bash
clawctl restart <instance>
```

**示例：**
```bash
clawctl restart my-pico
```

---

### clawctl delete

删除实例（移动到回收站）。

```bash
clawctl delete <instance> [flags]
```

**选项：**
- `--force` 跳过确认提示

**示例：**
```bash
clawctl delete my-pico
clawctl delete my-pico --force
```

---

### clawctl reset

重置实例工作区模板（仅 picoclaw 支持）。

```bash
clawctl reset <instance>
```

**注意：** zeroclaw 类型不支持此操作。

**示例：**
```bash
clawctl reset my-pico
```

---

### clawctl use

设置默认实例。

```bash
clawctl use <instance>
```

默认实例在 `clawctl list` 等命令中会被优先显示。

**示例：**
```bash
clawctl use my-pico
```

---

## 版本管理命令

### clawctl versions

查看已安装和可用的版本。

```bash
clawctl versions [flags]
```

**选项：**
- `--type <type>` 指定 Claw 类型（默认 `picoclaw`）

**示例：**
```bash
clawctl versions
clawctl versions --type zero
```

---

### clawctl install

安装指定版本。

```bash
clawctl install <claw_type> <version>
```

**示例：**
```bash
clawctl install picoclaw v0.2.6
clawctl install zero v0.6.9
clawctl install picoclaw latest
clawctl install picoclaw nightly
```

---

### clawctl uninstall

卸载指定版本。

```bash
clawctl uninstall <claw_type> <version>
```

**示例：**
```bash
clawctl uninstall picoclaw v0.2.5
```

---

## 回收站命令

### clawctl trash

管理回收站。包含以下子命令：

---

#### clawctl trash list

列出回收站中的实例。

```bash
clawctl trash list
```

---

#### clawctl trash restore

恢复回收站中的实例。

```bash
clawctl trash restore <trash-id>
```

---

#### clawctl trash clean

永久删除回收站中的单个实例。

```bash
clawctl trash clean <trash-id> [flags]
```

**选项：**
- `--force` 跳过确认提示

---

#### clawctl trash purge

清空回收站。

```bash
clawctl trash purge [flags]
```

**选项：**
- `--force` 跳过确认提示

---

## Claw 类型

| 类型 | 说明 | 默认端口 |
|------|------|----------|
| `picoclaw` | Picoclaw 实现 | 18790 |
| `zero` | Zeroclaw 实现 | 18792 |

---

## 配置文件

- 配置文件：`~/.clawctl/config.json`
- 实例工作目录：`~/.clawctl/instances/<instance>/`
- 回收站：`~/.clawctl/trash/`
- 下载版本：`~/.clawctl/claw_release/<type>/<version>/`
