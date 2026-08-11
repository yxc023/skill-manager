# WORK_LOG — sm (skill manager)

## 2026-07-29 — v0.2.2（CLI 直接调用）

### Why

用户：「如果想把 python 脚本开发成 cli 命令直接用，需要后续如何处理？」

不想每次都 `python3 work/20260724-skill-manager-sm/sm.py ...`。

### How

走「`pyproject.toml` + `uv tool install`」路径。理由：

- PEP 621 是 Python 官方打包标准（不是 setup.py 旧方案）
- `uv tool install` 是隔离环境，全局暴露 `sm` 命令；PEP 668 友好（不污染 system Python）
- 编辑 sm.py 后 `uv tool install --force --reinstall .` 立即生效

### 实现

1. `sm.py` 顶部加 `__version__ = "0.2.1"`
2. 加 `--version` flag 给 argparse
3. 新建 `pyproject.toml`：
   - `[project.scripts]` 声明 `sm = "sm:main"`
   - `[tool.setuptools] py-modules = ["sm"]` 让 sm.py 被当作单文件 module
4. 安装：`cd work/20260724-skill-manager-sm && uv tool install .`
5. 之后：`sm install` 直接用

### 验证

```
✓ which sm → /Users/michael/.local/bin/sm
✓ sm --version → sm 0.2.1
✓ sm -C <any-path> install/list/verify 全可用
✓ 旧式 `python3 .../sm.py --version` 仍可用（向后兼容）
```

### Next tier（如果以后想给团队用）

- 加 `[project.optional-dependencies]` dev = ["pytest", "ruff"]
- 加 `tests/` 目录
- 改 setuptools 为 hatchling 或 pdm（更现代）
- 发到 PyPI（`uv build && uv publish`）

---

## 2026-07-29 — v0.2.1（文件命名）

- `skills.json` → `skills-manage.json`
- `skills.lock.json` → `skills-manage.lock.json`（保持命名配对）
- `.gitignore` 同步更新
- 端到端测试：`validate` / `list` / `verify` 全绿
- `init` 生成新文件名正确
- 锁文件名同时改是设计选择（package.json ↔ package-lock.json 的同款配对），如想恢复 `skills.lock.json` 仅改一行即可

---

## 2026-07-28 — v0.2（lockfile 完整性 + 真 source 覆盖）

### v2 升级动机

拿到官方 vercel-labs/skills `skill-lock.ts` 源码后，发现 v1 lockfile 字段少了 4 个关键：

- `sourceType` — 显式 source 类型（github/git/gitlab/local）；v1 是字段隐式
- `sourceUrl` — 完整 URL（v1 用 owner/repo 缩写，遇到 gitlab/SSH 就完了）
- `skillFolderHash` — 内容 SHA-256；v1 只有 `installed_sha`（git commit SHA），**无法检出缓存被篡改、上游内容真变**
- `updatedAt` — 与 installedAt 独立；v1 只记一个 `installed_at`

→ v2 对齐 npx skills v3 字段。同时把 v1 flat 写法统一成 `source{}` 对象。

### 决策

- ✅ **全面升级 v2**（全量修改，而不是只补关键字段）
- ✅ **`sm verify`** 实现（把 skillFolderHash 真正用起来）
- ✅ **只支持 source object 写法**（v1 flat 不再支持，`install` 检测到直接报错）
- ❌ **不做 `pinHash`**（用 `ref: "<sha>"` 替代）
- ✅ **local source 的「self ref」用智能检测 + `mode: "self"` 显式 opt-out**

### v2 数据模型

```python
@dataclass
class Target:
    agent: str
    path: str
    mode: str = "symlink"   # "symlink" | "copy" | "self"

@dataclass
class SkillSource:
    type: str                  # "github" | "gitlab" | "git" | "local"
    repo: Optional[str]        # github/gitlab: "owner/repo"
    url:  Optional[str]        # git: 完整 URL
    path: Optional[Path]       # local: 文件系统路径
    subpath: Optional[str]     # repo 内 SKILL.md 子路径
    ref:    Optional[str]      # branch | tag | sha（默认 main）

@dataclass
class SkillDef:
    name: str
    source: SkillSource
    category: str = ""
    description: str = ""
    targets: list[Target] = []
    enabled: bool = True
```

### `mode: "self"` 怎么用

```json
{
  "source": { "type": "local", "path": "./docs/skills/doc-generator" },
  "targets": [
    { "agent": "opencode", "path": "./docs/skills/doc-generator", "mode": "self" }
  ]
}
```

- `mode="self"` → 不创建 symlink、不 copy；只验证 `target.path` 已存在 + 含 SKILL.md + 写 lock
- `mode="symlink"/"copy"` (默认) → `expand_target` 在路径末尾自动追加 `/<skill-name>`；`self` 跳过这步

自动检测作为兜底：local source 与 target path 解析到同一 Path 时（即使用户没写 `mode=self`），也不再 symlink，避免循环。

### v2 锁文件结构

```json
{
  "version": "2",
  "skills": {
    "skill-creator": {
      "source": "anthropics/skills",
      "sourceType": "github",
      "sourceUrl": "https://github.com/anthropics/skills.git",
      "ref": "main",
      "skillPath": "skills/skill-creator",
      "skillFolderHash": "ad4f350be137206c...",
      "installedAt": "2026-07-28T06:48:43+00:00",
      "updatedAt": "2026-07-28T06:48:43+00:00",
      "category": "tools/meta"
    }
  }
}
```

### 测试

| 测试 | 结果 |
|---|---|
| `validate` v2 manifest | ✅ 通过（v1 flat 报错并提示） |
| `install` 远程 4 个 skill | ✅ 自动 fetch、解析、symlink |
| `install` 同名 config 幂等 | ✅ "already linked" |
| **`sm verify`** 全绿 | ✅ 显示每个的 hash 前 12 字符 |
| **TAMPER 检测** — 改 cache 文件后 verify | ✅ 显示 locked vs actual hash 差异 |
| `sm install --only pdf`（hash 不匹配后） | ✅ 自动从远端重置 + 锁回到正确 hash |
| `mode=self` | ✅ 不创建 symlink，验证 SKILL.md 存在 |
| `mode=self` 路径不存在 | ✅ error |
| `mode=self` 缺 SKILL.md | ✅ error |
| parse_git_url (5 种 URL 形式) | ✅ SSH/HTTPS/SSH-protocol 全部解析 |
| parse_git_url 修复: HTTPS 被 SCP regex 误捕 | ✅ 修：先匹配 scheme form，再匹配 SCP form（且排除 `:` 后跟 `//`） |

### 实现细节 & 坑

1. **`parse_git_url` 必须先匹配 scheme**: `^(?:https?|ssh|git)://` 否则 SCP regex 会把 `https://github.com` 截成 `('https', 'github.com', 'anthropics/skills')` —— host 错了
2. **`expand_target` 加 `mode` 参数**: 只有 `mode != "self"` 才追加 `/<name>`
3. **`compute_folder_hash`**: 用 `relative_to(folder).as_posix() + \0 + content` 做 SHA-256；sorted paths 保证 deterministic
4. **`skillFolderHash` 不用 GitHub Tree SHA**: 改用本地 SHA-256。理由：npx skills 走 Tree API 要带 GitHub token + 受 rate limit 限制；本地 hash 离线可用、足够检出篡改
5. **`installedAt` vs `updatedAt`**: `install` 时只刷新 `updatedAt`，保留首次 `installedAt`

### 状态（v0.2）

- `sm.py` ~580 行
- `skills-manage.json`（v2 source object）：4 个 anthropic skill
- `skills-manage.lock.json`（v2）：含 hash + 时间戳
- `.opencode/skills/{tools/{meta,document},discovery/anthropic}/`（9 个 symlinks 都在）

### Next

- [ ] **`sm sync`** — `clean + install` 合一键
- [ ] **`sm install --refresh`** 强制重算 hash（不依赖 git fetch）
- [ ] **`sm diff`** — 对比 manifest vs lock
- [ ] **`sm publish`**（如未来要把自研 skill 发到 skills.sh / GitHub）
- [ ] **真正 version 0.3**：考虑支持 `pinHash`（hash 强锁，加回来）
- [ ] **README 加 GIF**（考虑）
- [ ] **要不要给 sm 也写一个 skill**（meta-skill，让 agent 引导用户用 sm 管理 skill 集）

---

## 2026-07-24 — v0.1（首次可用）

### Why

用户问：「如何让一个自研 skill 在多个 agent/多个人的环境里保持更新？核心是 manage skills as node module —— node 有 npm, skill 有 sm。」

调研见 `discovery-and-think/20260724-skill-dependency-manager-research/README.md`：
- 生态已经 80% 成熟（SKILL.md 是事实标准，npx skills 是 de-facto CLI）
- OpenCode 自己没有任何 install/marketplace/lockfile 机制
- 真缺口是「per-project 声明式 manifest」

### How

走**路径 A**：Python 脚本包装 npx skills 的 source 格式（`owner/repo[/<skill>]`），但**物理 fetch 用 git 直接做**（不是 `npx skills add`）。

最终设计：
- `skills-manage.json`（manifest）→ `~/.sm/cache/<repo>/` (git clone) → `<target>/<category>/<skill>` (symlink)
- 每 repo clone **一次**，所有 consumer 通过 symlink 共享
- category 控制 subdir（支持多级）

### Schema v1

```json
{
  "version": "1",
  "targets": [...],
  "skills": {
    "<name>": {
      "repo": "owner/repo",            // 或 "local": "/path"
      "skill": "<subdir-in-repo>",     // 可选
      "ref": "<branch|tag|sha>",       // 可选
      "category": "..."
    }
  }
}
```

### 测试（v0.1 已通过，v0.2 已废弃）

- ✅ install / list / lock / clean / outdated / validate / 幂等
- v0.1 lockfile 字段：`installed_sha` + `installed_at` + `category`，**没有 hash、没有 sourceType、没有 sourceUrl**

### v0.1 → v0.2 迁移

不是自动 migrate，因为 v0.1 flat 写法直接改写为 source object 后 `repo`/`local` 两个字段都消失，用户能立即察觉。

