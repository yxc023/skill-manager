# sm (skill manager) — Schema v2

> **Think: node has npm, skill has sm.**
>
> `skills-manage.json` = `package.json`，`sm install` = `npm install`，`~/.sm/cache/` = `node_modules/.cache/`，`skills-manage.lock.json` = `package-lock.json`。

sm v2 把每个 skill 拉到一个中央 git 缓存，然后通过 symlink 装到目标 agent 目录。**`category` 字段**让你把 skill 落到任意子目录，**`sm verify` 子命令**通过内容 hash 校验完整性。

---

## 文件

| 文件 | 角色 | 入仓？ |
|---|---|---|
| `skills-manage.json` | 声明式 manifest（source-of-truth） | ✅ 入仓 |
| `skills-manage.lock.json` | 已装的 SHA-256 + 时间戳（生成产物） | ❌ gitignore |
| `~/.sm/cache/<host>/<owner>/<repo>/` | 中央 git 缓存（git pull 此处） | ❌ 全局共享 |

---

## `skills-manage.json` manifest（v2）

```jsonc
{
  "$schema": "https://github.com/yxc023/skill-manager/blob/main/SCHEMA.md",

  "version": "2",

  // 默认 install 目标（per-skill `targets` 可覆盖）
  "targets": [
    {
      "agent": "opencode",                          // 任意命名，只是个 label
      "path": ".opencode/skills/{category}",        // {category} 占位；末尾自动追加 /<skill-name>，除非 mode=self
      "mode": "symlink"                              // "symlink" | "copy" | "self"
    }
  ],

  "skills": {
    "<skill-name>": {
      "source": {                                  // v2 唯一来源描述方式
        "type": "github",                            // github | gitlab | git | local
        "repo": "anthropics/skills",                 // github/gitlab: "owner/repo"
        "url":  "https://...",                       // git: 完整 URL（任意 host）
        "path": "/path/to/source",                   // local: 文件系统路径（支持 ~）
        "subpath": "skills/skill-creator",            // 可选：repo 内 SKILL.md 所在子路径
        "ref": "main"                                // 可选：branch | tag | sha，缺省 main
      },

      "category": "tools/document",                 // 控制 install 位置（核心差异化能力）
      "description": "...",
      "enabled": true                                // 可选：false 跳过 install/verify
    }
  }
}
```

---

## `skills-manage.lock.json`（v2）

自动生成，**不要手改**：

```json
{
  "version": "2",
  "skills": {
    "skill-creator": {
      "source": "anthropics/skills",                  // 短形式
      "sourceType": "github",                          // github | gitlab | git | local
      "sourceUrl": "https://github.com/anthropics/skills.git",   // 实际 fetch 用的 URL
      "ref": "main",
      "skillPath": "skills/skill-creator",             // repo 内子路径
      "skillFolderHash": "ad4f350be137206c...",        // SHA-256 of folder contents
      "installedAt": "2026-07-28T06:48:43+00:00",      // 首次安装时间
      "updatedAt":   "2026-07-28T06:48:43+00:00",      // 最近一次更新时间（独立于 installedAt）
      "category": "tools/meta"                        // sm 特有
    },
    "my-local-skill": {
      "source": "/abs/path/to/skill",
      "sourceType": "local",
      "sourceUrl": "/abs/path/to/skill",
      "ref": "main",
      "skillPath": "my-local-skill",                  // folder name
      "skillFolderHash": "...",
      "installedAt": "...",
      "updatedAt": "...",
      "category": "personal",
      "localPath": "/abs/path/to/skill"               // 本地路径冗余记录
    }
  }
}
```

---

## 字段约定（manifest）

### `source` 对象

| 字段 | 必填 | 类型 | 适用 type | 说明 |
|---|---|---|---|---|
| `type` | ✅ | string | - | `github` \| `gitlab` \| `git` \| `local` |
| `repo` | 与 `url`/`path` 二选一 | string | github, gitlab | 形如 `"owner/repo"`，自动拼成 `https://{github.com,gitlab.com}/<repo>.git` |
| `url` | 与 `repo` 二选一 | string | git | 完整 git URL（任意 host：GitLab / Bitbucket / 自托管 / 内网） |
| `path` | 与 `repo`/`url` 二选一 | string | local | 文件系统绝对路径；支持 `~` |
| `subpath` | ❌ | string | github, gitlab, git | repo 内 SKILL.md 所在子路径（如 `"skills/pdf"`）。省略时 sm 走标准发现路径 |
| `ref` | ❌ | string | github, gitlab, git | branch / tag / commit SHA。**缺省 `main`**。生产环境建议锁 commit |

**type=local 时无缓存概念**——source 就是 source。`subpath` 和 `ref` 不适用。

### `targets[]`

| 字段 | 必填 | 类型 | 说明 |
|---|---|---|---|
| `agent` | ✅ | string | 任意标识（如 `"opencode"`、`"claude-code"`），不是平台枚举 |
| `path` | ✅ | string | 模板，用 `{category}` 占位 |
| `mode` | ❌ | string | `symlink` (默认) \| `copy` \| `self` |

**`mode` 语义**：

| mode | 行为 | 何时用 |
|---|---|---|
| `symlink` | 在 `<expanded-path>/<name>` 创建 symlink → skill_dir | **默认**。中央源一改全更新 |
| `copy` | 把 skill_dir 内容复制到 `<expanded-path>/<name>` | 生产环境、容器、不能用 symlink 的 CI |
| `self` | **不创建任何东西**；把 `<path>` 当成最终安装位置（不追加 `/<name>`），仅校验 SKILL.md 存在 | **Local source 即 target**，比如 skill 直接住在 agent 读的位置 |

> **`self` 的典型用例**：当 SKILL.md 已经住在 `<some-agent-skills-dir>/<skill-name>/`，你不要 symlink 也不要 copy，只想 track 它存在 → 用 `mode: "self"`。

### 路径模板展开规则

对 `mode: "symlink"`（默认）或 `mode: "copy"`：

1. 把 `{category}` 替换为 skill 的 `category` 值（缺省则整段消失）
2. 末尾自动追加 `/<skill-name>`

对 `mode: "self"`：路径**原样使用**，不追加。

例子：

| target.path | category | name | 展开后 |
|---|---|---|---|
| `.opencode/skills/{category}` | `tools/document` | `pdf` | `.opencode/skills/tools/document/pdf` |
| `.opencode/skills/{category}` | `""`（缺省） | `pdf` | `.opencode/skills/pdf` |
| `~/.config/opencode/skills/{category}` | `personal` | `weekly` | `~/.config/opencode/skills/personal/weekly` |
| `/abs/path/to/skill` (mode=self) | - | `self-skill` | `/abs/path/to/skill` (无追加) |

---

## source 类型详解

### `github`（默认）

```json
"source": {
  "type": "github",
  "repo": "anthropics/skills",
  "subpath": "skills/skill-creator",
  "ref": "main"
}
```

→ fetch: `https://github.com/anthropics/skills.git`
→ cache: `~/.sm/cache/anthropics/skills/`

### `gitlab`

```json
"source": {
  "type": "gitlab",
  "repo": "group/sub/skills",
  "subpath": "skills/foo",
  "ref": "v1.3.0"
}
```

→ fetch: `https://gitlab.com/group/sub/skills.git`
→ cache: `~/.sm/cache/group/sub/skills/`

> ⚠️ 当前只支持 `gitlab.com`。自托管 GitLab 用 `type: "git"`。

### `git`（任意 host）

```json
"source": {
  "type": "git",
  "url": "git@gitlab.inner.com:team/main-skills.git",
  "subpath": "skills/file_parse",
  "ref": "v1.3.0"
}
```

→ cache: `~/.sm/cache/gitlab.inner.com/team/main-skills/`

支持 URL 形式：
- `git@host:owner/repo.git`
- `https://host/owner/repo.git`
- `ssh://git@host/owner/repo.git`

### `local`

```json
"source": {
  "type": "local",
  "path": "~/work/my-private-skill"
}
```

→ cache: 无（源即源）
→ install: 默认 symlink 到 `<target>` 下；路径若与 target 重叠则自动跳过

---

## `skills-manage.lock.json` 字段

### `skillFolderHash`

- 算法：`SHA-256` over（按文件名排序的）所有文件 `relative_path + \0 + content`
- 用处：`sm verify` 校验本地 cache 没被篡改、上游真的变了能被检出
- 不依赖网络（与官方 npx skills 的 GitHub Tree SHA 路线不同——我们用本地 hash，更快、可离线）

### `sourceUrl`

存的是**实际 fetch 时用的 URL**（`https://...`）。`git@host:owner/repo.git` 形式会被 `normalize_url` 转成 `ssh://host/owner/repo.git` 再存。

### `installedAt` vs `updatedAt`

| 字段 | 何时更新 |
|---|---|
| `installedAt` | **只在首次 install** |
| `updatedAt` | 每次 `sm install` / `sm update` |

→ 用 `updatedAt` 判断「最近一次 fetch 是何时」，用 `installedAt` 判断「这台机器上首次安装是何时」。

---

## 完整示例

```json
{
  "$schema": "https://github.com/yxc023/skill-manager/blob/main/SCHEMA.md",
  "version": "2",
  "targets": [
    { "agent": "opencode",    "path": ".opencode/skills/{category}" },
    { "agent": "claude-code", "path": ".claude/skills/{category}" }
  ],
  "skills": {
    "skill-creator": {
      "source": {
        "type": "github",
        "repo": "anthropics/skills",
        "subpath": "skills/skill-creator",
        "ref": "main"
      },
      "category": "tools/meta",
      "description": "Anthropic 元 skill - 创建/改进 skill"
    },
    "frontend-design": {
      "source": {
        "type": "github",
        "repo": "anthropics/skills",
        "subpath": "skills/frontend-design",
        "ref": "main"
      },
      "category": "discovery/anthropic"
    },
    "internal-foo": {
      "source": {
        "type": "git",
        "url": "git@gitlab.inner.com:team/skills.git",
        "subpath": "skills/foo",
        "ref": "v1.3.0"
      },
      "category": "internal/team"
    },
    "my-local-rag": {
      "source": {
        "type": "local",
        "path": "~/work/my-rag-skill"
      },
      "category": "personal/experimental"
    },
    "self-hosted-doc": {
      "source": {
        "type": "local",
        "path": "./docs/skills/doc-generator"
      },
      "targets": [
        {
          "agent": "opencode-docs",
          "path": "./docs/skills/doc-generator",
          "mode": "self"
        }
      ],
      "description": "skill 已在目标位置存在，不需复制"
    }
  }
}
```

---

## v1 → v2 迁移

v1 的扁平字段已不再支持。需要把：

```json
// v1
"my-skill": {
  "repo": "owner/repo",
  "skill": "sub",
  "ref": "main",
  "category": "..."
}
```

改成：

```json
// v2
"my-skill": {
  "source": {
    "type": "github",
    "repo": "owner/repo",
    "subpath": "sub",
    "ref": "main"
  },
  "category": "..."
}
```

`sm install` 在检测到 v1 manifest 时直接报错，不会自动 migrate（避免静默改写你的文件）。

---

## 与 npx skills v3 对齐

sm v2 lockfile 字段与 npx skills v3 几乎一一对应：

| npx skills v3 字段 | sm v2 字段 | 差异 |
|---|---|---|
| `source` | `source` | ✓ |
| `sourceType` | `sourceType` | ✓ |
| `sourceUrl` | `sourceUrl` | ✓ |
| `ref` | `ref` | ✓ |
| `skillPath` | `skillPath` | ✓ |
| `skillFolderHash` (GitHub Tree SHA) | `skillFolderHash` (本地 SHA-256) | **算法不同**（网络 vs 本地） |
| `installedAt` | `installedAt` | ✓ |
| `updatedAt` | `updatedAt` | ✓ |
| `pluginName` | - | sm 当前不分组 plugins（YAGNI） |
| `localPath` | `localPath` | ✓ |
| `category` (sm 特有) | `category` | sm 独有，定位子目录 |

---

## 与其他工具对照

| 工具 | manifest 字段 | source 支持 | 内容 hash pin |
|---|---|---|---|
| **sm v2** | `source{}`, `category`, `targets[].mode` | github / gitlab / git / local | ✓（本地 SHA-256） |
| npx skills | `source{}` + `skillPath` + `ref` | github / mintlify / huggingface / local / well-known | ✓（GitHub Tree SHA） |
| skillfile | `source`, `repo`/`url`, `subpath`, `ref`, `install_to` | github / git | ❌ |
| vcspull | path-as-key + `repo`, `remotes`, `branch`, `subdir`, `depth` | 任意 git | ❌（仅 depth 浅克隆） |
| dfetch | `url`, `revision`, `dest` | 任意 git | ❌ |

sm 是「npx skills 风格的字段 + 分层安装能力 + 跨 host 支持」的中间态。
