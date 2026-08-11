# sm (skill manager) — README

> **Think: node has npm, skill has sm.**

`sm` 是 AI agent skill 的声明式依赖管理器。从 `skills-manage.json`（类似 `package.json`）读取声明，把每个 skill 源 git clone 到 `~/.sm/cache/`，然后 symlink 到目标 agent 目录，**通过 `category` 字段控制每个 skill 的子目录位置**。

**v2 核心能力**：
- source 用对象表达，支持 github / gitlab / git（任意 host）/ local 四种类型
- lockfile 含 SHA-256 校验，`sm verify` 能检出本地篡改 / 上游内容真变
- `mode: "self"` 让「skill 已经在目标位置」情况下无需 symlink/copy
- 与 npx skills v3 字段一一对应（`sourceType` / `sourceUrl` / `skillFolderHash` / `installedAt` / `updatedAt`）

---

## 5 分钟入门

```bash
# 1. 初始化（生成 starter）
sm init

# 2. 编辑 skills-manage.json 声明依赖

# 3. 安装 — git clone + symlink 到 agent 目录
sm install

# 4. 完整性验证 — hash / 链路 / 完整性检查
sm verify

# 5. 日常用法
sm list       # 看现状
sm outdated   # 看哪些落后
sm update     # git fetch + install
```

如果 `sm` 命令还没装（首次）：

```bash
# 装到 PATH：uv tool install （推荐，隔离环境）
cd work/20260724-skill-manager-sm
uv tool install .

# 或直接 symlink（30 秒方案）
ln -sf "$(pwd)/work/20260724-skill-manager-sm/sm.py" ~/.local/bin/sm

# 之后改 sm.py 想立即生效：
uv tool install --force --reinstall .
```

---

## 目录结构

```
~/michael-work/                                # 项目根
├── skills-manage.json                         # 入仓 ← source-of-truth
├── skills-manage.lock.json                    # 不入仓 ← 本机锁定状态
└── .opencode/skills/                          # 不入仓 ← 已安装 symlink
    ├── tools/
    │   ├── document/{pdf,docx}                # category = "tools/document"
    │   └── meta/skill-creator                 # category = "tools/meta"
    └── discovery/anthropic/frontend-design

~/.sm/cache/                                   # 不入仓，全局共享
└── anthropics/
    └── skills/                                # 一个 repo clone 一次
        └── skills/
            ├── pdf/        ← 含 SKILL.md
            ├── skill-creator/
            └── frontend-design/
```

---

## subcommand 速查

| 命令 | 作用 |
|---|---|
| `sm init [-f]` | 生成 starter `skills-manage.json` |
| `sm install [--only a,b]` | 按 manifest 安装 / 重装所有 skill |
| `sm update` | 同 install（硬重置为远端最新） |
| `sm list` | 列出 manifest 里的 skill + 锁定的 hash |
| `sm outdated` | 对比远端 SHA vs 已装 SHA，找落后项 |
| **`sm verify`** | **校验完整性**——hash 对照 + 链路检查 |
| `sm validate` | 校验 `skills-manage.json` 语法 |
| `sm clean` | 删掉所有 sm 创建的 symlink（缓存保留） |
| `sm lock` | 打印 `skills-manage.lock.json` |

### 常用 flags

- `-C <dir>` — 用指定目录的 `skills-manage.json`，缺省 `.`
- `--cache-dir <dir>` — 覆盖 `~/.sm/cache/`

---

## 多 agent / 多机器场景

`skills-manage.json` 是 portable 的。放在任何地方都能跑 `sm -C <that-dir> install`。

### 同一台机器，多个项目

```bash
# 把同一份 skills-manage.json symlink 到多个项目
ln -s ~/michael-skills/skills-manage.json ~/project-a/skills-manage.json
ln -s ~/michael-skills/skills-manage.json ~/project-b/skills-manage.json

# 然后在每个项目分别跑
cd project-a && sm install
cd project-b && sm install

# 共享 ~/.sm/cache，git clone 只发生一次
```

### 同一项目，多 agent

在 `targets` 里加多行：

```json
"targets": [
  { "agent": "opencode",    "path": ".opencode/skills/{category}" },
  { "agent": "claude-code", "path": ".claude/skills/{category}" },
  { "agent": "cursor",      "path": ".cursor/skills/{category}" }
]
```

### 全局 vs 项目级

把 `path` 改成以 `~/` 开头：

```json
{ "agent": "opencode", "path": "~/.config/opencode/skills/{category}" }
```

或者同时装项目级 + 全局级：

```json
"targets": [
  { "agent": "opencode-project", "path": ".opencode/skills/{category}" },
  { "agent": "opencode-global",  "path": "~/.config/opencode/skills/{category}" }
]
```

---

## 物理流程（`sm install` 做了什么）

1. 读 `skills-manage.json`
2. 对每个 skill 的 `source`：
   - **github/gitlab/git**: `git clone --depth 1 -b <ref> <resolved-url> ~/.sm/cache/<...>/`
     - 已 clone 则 `git fetch --depth 1 --force` + `git reset --hard origin/<ref>`
   - **local**: 直接用 `source.path`（无缓存）
3. 在 repo / local 目录里用 `subpath` 或自动发现定位 SKILL.md → `skill_dir`
4. 计算 `skill_dir` 内容的 SHA-256 → `skillFolderHash`
5. 对 `targets` 里每个 target，按 `mode` 处理：
   - **`symlink`** (默认): 在 `<expanded-path>/<name>` 创建指向 `skill_dir` 的 symlink（如已存在且指向同一 dir，跳过）
   - **`copy`**: `shutil.copytree(skill_dir, target)`
   - **`self`**: 验证 `target.path` 已存在且含 SKILL.md，**不创建任何东西**
6. 智能检测：**local source + target path resolve 相同时** → 跳过 symlink（即「source IS target」）
7. 把所有元信息写入 `skills-manage.lock.json`（仅 `installedAt` 保留，`updatedAt` + `skillFolderHash` 每次更新）

---

## `sm verify` 是干嘛的

```bash
sm verify
# ✓ skill-creator  ad4f350be137
# ✓ frontend-design  827565e52bfd
# ✗ pdf
#     HASH MISMATCH
#         locked:  89b66cee7e45...
#         actual:  5a9fc0aee37b...
#         location: /Users/michael/.sm/cache/anthropics/skills/skills/pdf
```

校验三件事：

1. **每个 skill 的 `skill_dir` 存在且含 SKILL.md**
2. **`skill_dir` 内容的 SHA-256 == `lockfile.skillFolderHash`**
   - 不匹配 ⇒ 内容被改过（篡改或上游真的变了需要 `sm update`）
3. **每个 `target` 安装位置存在**

适合：

- CI 跑一遍确认部署一致
- 排查「这 skill 怎么行为变了」
- 多人/多机器协作时确认大家 lock 在一致版本

---

## 三种 target mode

| mode | 何时用 | 行为 |
|---|---|---|
| `symlink`（默认） | 日常开发 | target 是个 symlink → cache 里的 `skill_dir`，中央源改全员更新 |
| `copy` | 生产 / 容器 / CI | target 是真实文件夹（独立的快照） |
| `self` | **本地 skill 已在目标位置** | target 就是 `skill_dir` 本身，不创建任何东西 |

**示例（mode=self）**：你的文档 skill 已经住在 `./docs/skills/doc-generator/`，而你希望 OpenCode 直接读它：

```json
{
  "source": { "type": "local", "path": "./docs/skills/doc-generator" },
  "targets": [
    { "agent": "opencode", "path": "./docs/skills/doc-generator", "mode": "self" }
  ]
}
```

`sm install` 会验证 `SKILL.md` 存在、记录 lock，不创建任何额外东西。

---

## 四种 source type

| type | 字段 | 用途 |
|---|---|---|
| `github` | `repo: "owner/name"` | `https://github.com/owner/name.git` |
| `gitlab` | `repo: "owner/name"` | `https://gitlab.com/owner/name.git`（暂只支持官方） |
| `git` | `url: "git@host:o/r.git"` 或 `https://...` | 任意 GitLab/Bitbucket/自托管/内网 |
| `local` | `path: "/abs/path"` 或 `~/...` | 本地文件系统路径（无 git、无缓存） |

---

## 与 `npx skills` 关系

**npx skills**（vercel-labs/skills）走纯 CLI、无 manifest。

**sm** 是它的 manifest-aware + category-aware 替代：

- 同样的 source 格式（`owner/repo` / git URL / local）
- 同样的 SKILL.md 发现路径（Agent Skills spec）
- sm 多一层 `category` 控制子目录
- sm 多一份 `skills-manage.lock.json` 锁版本 + hash
- sm 多一条 `sm verify` 校验命令

两者**可共存**：

- `sm install` 处理你固定的依赖（commit 到 git 的项目级 manifest）
- `npx skills add <random-repo>@<random-skill>` 处理临时尝试

> sm 当前不调 `npx skills`——直接用 git fetch。理由是 npx skills 的安装拓扑（标准 agent 路径，无 category 层）跟我们想要的 `<expanded>/<name>` 拓扑不直接 compose。

---

## 已知行为 / 待办

- ❌ **`sm sync`** — `clean + install` 合一键。待办。
- ❌ **`sm outdated`** 对 lock hash 对 git HEAD SHA —— 当前用 git short SHA 拍脑袋比，未来用 `skillFolderHash` 对 `<ref>` 真实 commit 的 tree SHA。
- ❌ **per-skill `targets` 的 `agent` 字段**目前能覆盖但不能指定 mode 之外的 path ——已经支持（per-skill targets list 直接替换 default list）。
- ✅ **v2 完整字段**与 npx skills v3 一一对应；不远的将来写 `npx skills sync && sm install` 互转也有可能。
- 旧的 v1 manifest 不会被自动 migrate——`sm install` 检测到 v1 直接报错并提示改写（避免静默改用户文件）。

---

## 文件

| 文件 | 角色 |
|---|---|
| `sm.py` | 主脚本（纯 stdlib） |
| `SCHEMA.md` | `skills-manage.json` + `skills-manage.lock.json` 详细 schema |
| `pyproject.toml` | PEP 621 metadata，让 `sm` 可以 `uv tool install .` |
| `WORK_LOG.md` | 工作日志 |
| `../../skills-manage.json` | 仓库根下的实际 manifest |
| `../../skills-manage.lock.json` | 本机锁文件（gitignored） |

---

## License

MIT
