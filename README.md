# agentcall

[English README](README.en.md)

`agentcall` 用真实 PTY 启动终端型 agent，例如交互式 `claude`、交互式 `codex`，以及其他基于终端的 CLI agent，并通过本机 `localhost` HTTP callback 收集结构化停止态结果。

runner 会先启动目标 TUI，再通过 PTY 注入一段包装后的任务 prompt，并额外发送一次 Enter，尽量模拟人类在终端里的操作。

目标 agent 需要能在交互界面中接收这段注入的 prompt，并遵守 callback contract：向其中给出的 `localhost` URL 发送 JSON 回调，字段至少包括 `token`、`status`、`content_type`、`content`。

## 能力

- 保留真实终端交互行为，而不是走 `-p` 这类纯打印模式
- 通过 PTY 注入 prompt，而不是依赖命令行 positional prompt
- 输出结构化 JSON 结果，适合被上层 invoker 或 agent 编排
- 支持保存 `status.json`、原始 `transcript.log`，以及去 ANSI 的纯文本 `transcript.txt`
- 支持 `--auto-trust`，自动确认已识别的 workspace trust 对话框一次

## 安装

发布包里同时包含可执行文件和 skill 文件 `skills/agentcall/SKILL.md`。

### 方式一：下载预编译发布包

1. 从 GitHub Releases 下载与你的平台匹配的压缩包：
   - Linux: `linux_amd64` 或 `linux_arm64`
   - macOS: `darwin_amd64` 或 `darwin_arm64`
   - Windows: `windows_amd64` 或 `windows_arm64`
2. 解压后，把 `agentcall`（Windows 下为 `agentcall.exe`）放到 `PATH` 里的目录。
3. 安装 skill 文件到 Codex 和 Claude：

```bash
mkdir -p ~/.agents/skills/agentcall ~/.claude/skills/agentcall
cp skills/agentcall/SKILL.md ~/.agents/skills/agentcall/SKILL.md
cp skills/agentcall/SKILL.md ~/.claude/skills/agentcall/SKILL.md
```

如果你在 Windows 上没有 bash，就手动复制到 `%USERPROFILE%\.agents\skills\agentcall\SKILL.md`（Codex）和 `%USERPROFILE%\.claude\skills\agentcall\SKILL.md`（Claude）。

### 方式二：从源码构建

```bash
make build
install -m 755 bin/agentcall ~/.local/bin/agentcall
mkdir -p ~/.agents/skills/agentcall ~/.claude/skills/agentcall
cp skills/agentcall/SKILL.md ~/.agents/skills/agentcall/SKILL.md
cp skills/agentcall/SKILL.md ~/.claude/skills/agentcall/SKILL.md
```

## 基本用法

```bash
agentcall run \
  --timeout 600s \
  --prompt "review this diff" \
  -- claude --dangerously-skip-permissions
```

## 常用参数

- `--prompt`：要通过 PTY 注入给目标 agent 的任务文本
- `--timeout`：单次运行超时，默认 `600s`；大任务通常需要更长时间，应由 caller 按复杂度显式调大
- `--heartbeat-period`：活跃运行保持足够久时，heartbeat JSON 向 `stderr` 输出的周期；默认 `1s`
- `--verbose`：heartbeat 输出级别；`0` 完全关闭 heartbeat，`1` 在 heartbeat 实际触发时输出基础字段，`2` 在 heartbeat 实际触发时额外输出诊断字段
- `--artifacts-dir`：结果和 transcript 的输出目录；不传时会自动创建临时目录，但路径不可预测
- `--status-file`：显式指定状态 JSON 路径；不传时默认写到 `artifacts-dir/status.json`
- `--auto-trust`：自动确认一次已识别的 trust prompt

对 `claude` / `codex` 的非交互调用，默认推荐直接使用 yolo 参数：

- `claude --dangerously-skip-permissions`
- `codex --dangerously-bypass-approvals-and-sandbox`

对应的关键 flag 分别是 `--dangerously-skip-permissions` 和 `--dangerously-bypass-approvals-and-sandbox`。

否则它们可能在命令审批或权限确认上阻塞。现在 `agentcall` 检测到这类确认提示时会直接报错退出，把线索写进 `error` 和 `status.json`，而不是静默卡到超时。

对于 Codex 启动阶段出现的更新提示，如果界面提供 `Skip` 选项，`agentcall` 会自动切到 `Skip` 并继续运行；如果自动选择后更新对话框仍然停留在 screen snapshot 里，runner 会返回 `startup_blocked`。

## Claude 示例

```bash
agentcall run \
  --auto-trust \
  --timeout 180s \
  --prompt "Review the current diff and send the final result through the callback." \
  -- claude --dangerously-skip-permissions
```

## Codex 示例

```bash
agentcall run \
  --timeout 180s \
  --prompt "Review the current diff and send the final result through the callback." \
  -- codex --dangerously-bypass-approvals-and-sandbox
```

## 输出

当 runner 成功启动目标 agent 后，如果活跃时间足够长，达到当前配置的 `--heartbeat-period`，默认会向 `stderr` 输出按行分隔的 heartbeat JSON；如果传 `--verbose=0`，则会完全抑制这些 heartbeat。`stdout` 始终保留给最终唯一一条结果 JSON envelope，不论最终是收到 callback，还是走到 `timed_out` / `callback_missing` 这类 runner 终态。
如果是参数错误、启动失败或 JSON 编码失败，CLI 会改为输出纯文本错误到 `stderr`，并返回 exit code `1`。

heartbeat 行示例：

```json
{"type":"heartbeat","run_id":"latest","seq":7,"timestamp":"2026-05-26T12:34:56Z","state":"active"}
```

runner 自身会把最终结果输出为一条 JSON，例如：

```json
{
  "run_id": "latest",
  "state": "callback_received",
  "status": "ok",
  "exit_code": 0,
  "content_type": "text/plain",
  "content": "done",
  "error": ""
}
```

callback 可接受的 `status`：

- `ok`
- `needs_input`
- `error`
- `refused`

runner 还可能生成这些非 callback 终态：

- `startup_blocked`
- `approval_required`
- `restart_required`
- `timed_out`
- `callback_missing`

## Exit Code

- `0`：`ok`
- `2`：`needs_input` 或 `refused`
- `3`：`timed_out`
- `4`：`callback_missing`
- `1`：`error`，以及其他失败，例如参数错误、runner 启动失败、内部错误

## Artifact

当 runner 成功启动目标 agent，并且运行走到可收敛的终态时，会在 artifact 目录下写出：

- `status.json`
- `transcript.log`
- `transcript.txt`

如果目标进程在启动前就失败，例如命令不存在，那么目录可能已经创建，但这两个文件不会出现。

如果没有传 `--artifacts-dir`，runner 会创建一个临时目录，适合临时调试，但不适合让外部 invoker 依赖。
如果需要稳定的最终结果路径：

- 传 `--status-file` 获取可预测的最终状态 JSON 路径
- 传 `--artifacts-dir` 获取可预测的 transcript / artifact 路径

`status.json` 只会在运行结束时写出，不会在 `starting`、`running`、`active` 之类的中间状态持续刷新，所以它不是实时进度通道。

其中 `transcript.log` 会保留原始终端输出和 runner 注记，例如 `auto-trust confirmed`；`transcript.txt` 会去掉 ANSI 控制序列、spinner 和重复标题，便于上层 caller 直接读取关键明文。
