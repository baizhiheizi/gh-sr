# gh-sr 项目长期笔记

## 本机跑 Go 测试的临时目录约定

本机 `/tmp` 是 10M tmpfs，Go linker 写大二进制会报 `no space left on device`；而把 `TMPDIR` 直接设成长路径（如 `/home/an-lee/.cache/gh-sr-tmp`）会让 `internal/host` 的 `t.TempDir()` Unix socket 路径超过 AF_UNIX 的 108 字节上限（`bind: invalid argument`），设到 `/home/an-lee/t` 之类沙箱外目录则会 `operation not permitted`。

可行做法：用**短符号链接指向大分区**，让路径短、存储足、且在允许写入的位置：

```bash
mkdir -p /home/an-lee/.cache/gh-sr-tmp
ln -sfn /home/an-lee/.cache/gh-sr-tmp /tmp/t
TMPDIR=/tmp/t GOTOOLCHAIN=auto go test ./... -race -count=1
```

`GOTMPDIR` 与 `TMPDIR` 同时设置时，Go 会把 `GOTMPDIR` 传给测试进程，仍会触发 socket 路径过长，不要这样组合。

根因（2026-11-12 诊断）：`/proc/self/mountinfo` 里 `/tmp` 有**两层叠加挂载**——底层 `size=31682936k`（≈30.2 GiB，systemd 默认的 50% RAM，完全正常），上层 `size=10240k,uid=1000,gid=1000`（AI 会话沙箱注入的私有 tmpfs，把底层盖住）。`/etc/fstab` 里没有 /tmp 条目，`/dev/shm` 也被同样覆盖成 10M，`systemd-detect-virt=container-other`。所以不是磁盘故障、也不是配置写错，无需修系统；只是当前会话的 tmpfs 配额。CapEff 含 `cap_sys_admin`，理论上能 remount 调大，但只对当前私有 mount namespace 生效、新会话会恢复，绕开更稳妥。

## 仓库惯例

- 批处理远端探针（如 `dirSizesBatchPOSIX`、PR #633 的 `status_batch.go`）用远端循环 + heredoc 传递实例列表，脚本长度只随实例名增长，不要逐实例展开状态机。
- 重复的 shell 片段要单一真相源 + 测试 pin（参考 `internal/autostart/launchd_test.go` 的 `TestLaunchdDomainList_IsSingleSourceOfTruth`；PR #633 新增 `containerStatusProbeBody()`）。
- 提交/合并风格：main 用 squash merge，commit message 形如 `perf(runner): ... (#633)`。
- `bench-compare` CI 会对 `internal/runner` 的 B/op、allocs/op、round-trips/op 做 gate（ns/op 仅 warn）。B/op 或 allocs 回归会让该 job 非零退出，但不影响 `test` / `build` 结果。
