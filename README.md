# ClawCtl

Claw 实例管理工具，用于管理 picoclaw 和 zeroclaw 集群实例。

## 安装

```bash
go build -o $HOME/go/bin/clawctl ./cmd/clawctl/
```

## 快速开始

```bash
# 创建实例
clawctl create my-pico --type picoclaw

# 启动实例
clawctl start my-pico

# 查看状态
clawctl list

# 停止实例
clawctl stop my-pico
```

## 实例管理

| 命令 | 说明 |
|------|------|
| `clawctl create <name> --type <type>` | 创建新实例 |
| `clawctl list` | 列出所有实例 |
| `clawctl info [name]` | 显示实例详情 |
| `clawctl status <name>` | 显示运行状态 |
| `clawctl start <name>` | 启动实例 |
| `clawctl stop <name>` | 停止实例 |
| `clawctl restart <name>` | 重启实例 |
| `clawctl delete <name>` | 删除实例（移至回收站） |
| `clawctl reset <name>` | 重置工作区模板（仅 picoclaw） |
| `clawctl use <name>` | 设置默认实例 |

## 版本管理

| 命令 | 说明 |
|------|------|
| `clawctl versions` | 查看可用版本 |
| `clawctl install <type> <version>` | 安装版本 |
| `clawctl uninstall <type> <version>` | 卸载版本 |

## 回收站

| 命令 | 说明 |
|------|------|
| `clawctl trash list` | 列出回收站 |
| `clawctl trash restore <id>` | 恢复实例 |
| `clawctl trash clean <id>` | 永久删除 |
| `clawctl trash purge` | 清空回收站 |

## Claw 类型

| 类型 | 说明 | 默认端口 |
|------|------|----------|
| `picoclaw` | Picoclaw 实现 | 18790 |
| `zero` | Zeroclaw 实现 | 18792 |

## 配置文件

- 配置：`~/.clawctl/config.json`
- 实例：`~/.clawctl/instances/<name>/`
- 回收站：`~/.clawctl/trash/`
- 下载版本：`~/.clawctl/claw_release/<type>/`

详细文档：[docs/CLAWCTL_USAGE.md](docs/CLAWCTL_USAGE.md)
