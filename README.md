<div align="center">

# 🇨🇳 china-mirror-cli (`cmc`)

**给中国开发者的单二进制 CLI ——一行命令把 pip / npm / docker / apt / 等等切到国内镜像。**

让 AI 帮你装环境，或者你自己在终端里敲，都很顺手。

</div>

---

## 它解决什么问题

国内拉 pypi / npm / docker hub / golang.org 太慢甚至断网，每装一台新机器都要：
- Google "pip 国内镜像 清华"
- 复制粘贴 `pip config set global.index-url ...`
- 同样的事在 `npm`、`yarn`、`docker daemon.json`、`apt sources.list`、`go env` 重复 10 次
- 配错了又要回滚

`cmc` 把这些都收敛成一个命令行工具：

```bash
cmc python setup           # 一条命令搞定 pip + uv + poetry
cmc node setup             # 一条命令搞定 npm + yarn + pnpm
cmc doctor                 # 看看当前机器什么没配好
cmc health                 # 看看哪个镜像现在最快
```

镜像清单（清华 / USTC / 阿里 / npmmirror / 腾讯云 / ...）维护在 `data/mirrors.yml`，运行时读取，CI 每天自动检测健康度。

---

## 为什么是 CLI 而不是脚本或 Skill

设计灵感来自 [OpenCLI](https://github.com/jackwener/opencli) ——「让 AI agent 和人类用同一套接口」。

| 场景 | 用法 |
|---|---|
| **AI 自己装环境**（推荐） | Claude / Cursor / Codex 在你的 shell 里直接 `cmc python setup`，不用嵌一大段 bash |
| **人在终端** | `cmc <Tab><Tab>` 自动补全，`cmc list mirrors --category pip --format json \| jq` |
| **CI / 脚本** | `cmc health --format json --output report.json` 输出 JSON，退出码 sysexits 规范 |
| **插件化扩展** | `cmc plugin install github:user/cmc-plugin-foo` 添加新工具适配器（v2） |

一份能力，三种入口；调用方式都是一样的 `cmc <tool> <command> [--flags]`。

---

## 安装

> 计划三种途径，目前 v0.1 阶段建议用 `go install` 或源码编译。

```bash
# 1) 一键脚本（GitHub Release 二进制，正在准备）
curl -fsSL https://raw.githubusercontent.com/loredunk/china-mirror-cli/main/scripts/install.sh | bash

# 2) Homebrew（正在准备）
brew install loredunk/china-mirror/cmc

# 3) 从源码（现在就能用）
git clone https://github.com/loredunk/china-mirror-cli.git
cd china-mirror-cli
make install      # 安装到 $(go env GOPATH)/bin/cmc

# 4) 直接 go install
go install github.com/loredunk/china-mirror/cmd/cmc@latest
```

装完 `cmc version` 应输出类似 `cmc v0.1.0 (darwin/arm64)`。

---

## 快速上手

### 让 AI 帮你装

直接对 Claude / Cursor / Codex 说：

> 帮我用 china mirror cli 把这台机器的所有包管理器都切到国内镜像，然后体检一下。

AI 会跑：

```bash
cmc doctor                 # 体检：什么装了、什么没配
cmc python setup           # 切 pip/uv/poetry
cmc node setup             # 切 npm/yarn/pnpm
cmc docker setup           # （即将支持）
cmc doctor                 # 复检
```

每一步都是 idempotent 的，可以反复跑。

### 自己在终端用

```bash
# 看看支持哪些工具
cmc list

# 看看 pip 类目下都有哪些镜像源
cmc list mirrors --category pip
cmc list mirrors --category pip --format json | jq

# 用清华源配 pip + uv + poetry（默认会用优先级最高的活动镜像）
cmc python setup
cmc python setup --mirror pip-tuna       # 显式选清华
cmc python setup --mirror pip-aliyun     # 改阿里
cmc python setup --dry-run               # 干跑，只打印不写入
cmc python setup --tool pip              # 只配 pip，不动 uv/poetry

# Node 同理
cmc node setup
cmc node setup --mirror npm-tencent

# 想测一下哪个 pip 镜像现在最快？
cmc health --category pip

# 出了问题想回滚
cmc backup --all              # 提前备份所有工具的配置
cmc restore --list            # 列出所有备份
cmc restore pip --latest      # 把 pip 配置还原到最近一次备份
```

### 全局标志（所有子命令通用）

```
-m, --mirror <id>     选镜像 id（不传则用 mirrors.yml 中 priority=1 的 active 镜像）
-d, --dry-run         预览生成的配置内容，不写盘
-y, --yes             跳过确认（脚本/CI 用）
-f, --force           强制覆盖
    --format <fmt>    table|json|yaml|md|csv（list/health 等输出类命令）
-v, --verbose         详细日志
```

---

## 当前进度

- ✅ `cmc list / list mirrors`（5 种输出格式）
- ✅ `cmc doctor`（OS / proxy / 已装工具 / 各工具当前镜像配置 / 连通性 / 建议）
- ✅ `cmc health`（并发健康检查，JSON 输出与 `reports/report.json` schema 一致，可直接替换 CI 里的 Python 脚本）
- ✅ `cmc backup [tool|--all]` / `cmc restore [tool] --latest|--id|--list`（与旧 bash 脚本同格式，旧备份可互通）
- ✅ `cmc python setup` — pip / uv / poetry
- ✅ `cmc node setup` — npm / yarn / pnpm
- ⏳ `cmc docker / apt / homebrew / conda / rust / go / flutter / github`
- ⏳ `cmc plugin install github:user/cmc-plugin-xxx`（OpenCLI 风格清单插件）
- ⏳ 一键安装脚本 + Homebrew tap + goreleaser

---

## 架构（一段话）

```
data/mirrors.yml ── 唯一镜像数据源（id/url/category/priority/verify）
       │  go:embed
       ▼
internal/mirrors   运行时加载 + ~/.config/cmc/mirrors.yml 覆盖 + plugin 合并
       │
       ├──► internal/adapter/python ── pip / uv / poetry config writer
       ├──► internal/adapter/node   ── npm / yarn / pnpm config writer
       ├──► internal/doctor         ── 只读环境检测
       ├──► internal/health         ── 并发 HTTP 健康检查（移植自 check_mirrors.py）
       └──► internal/config         ── 跨工具 backup/restore（与旧 bash 同格式）
       ▼
cmd/cmc                cobra root，每个 adapter 自动注册为子命令
```

加一个新工具 ＝ 在 `internal/adapter/<name>/` 下写一个 `Adapter`，在 `init()` 里 `adapter.Register(&Adapter{})`，就出现在 `cmc list` 和 `cmc <name> setup` 里了。

---

## 贡献

镜像数据改 `data/mirrors.yml`（CI 会自动跑健康检查）。
代码改 `internal/`，跑 `make build && make test`。
PR 欢迎 ——尤其是新工具的 adapter。

---

## 与旧仓库的关系

本项目脱胎于 [`loredunk/china-mirror-skills`](https://github.com/loredunk/china-mirror-skills)（原来是一组 bash 脚本 + Claude Skill）。`mirrors.yml` 与原仓库保持兼容，旧的 `~/.china-mirror-backup/` 备份目录也能被 `cmc restore` 直接还原。

旧 Skill 不再单独维护——AI 直接调 `cmc` 就行。

---

## License

MIT
