# skill-manager — Schema v2

> **Think: node has npm, skill has skill-manager.**
>
> `skills-manage.json` = `package.json`，`skill-manager install` = `npm install`，`~/.skills-manage/` = `node_modules/.cache/`，`skills-manage.lock.json` = `package-lock.json`。

skill-manager v2 把每个 skill 拉到一个中央 git 缓存，然后通过 symlink 装到目标 agent 目录。**`category` 字段**让你把 skill 落到任意子目录，**`skill-manager verify` 子命令**通过内容 hash 校验完整性。

---

## 文件

| 文件 | 角色 | 入仓？ |
|---|---|---|
| `skills-manage.json` | 声明式 manifest（source-of-truth） | ✅ 入仓 |
| `skills-manage.lock.json` | 已装的 SHA-256（生成产物） | ❌ gitignore |
| `~/.skills-manage/<host>/<owner>/<repo>/` | 中央 git 缓存（git pull 此处） | ❌ 全局共享 |

---

## `skills-manage.json` manifest（v2）

```jsonc
{
  "$schema": "https://github.com/yxc023/skill-manager/blob/main/SCHEMA.md",

  "version": 2,

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

> **Note**: Go 代码中 `version` 是 `int`（`2`），不是字符串 `"2"`。Python `sm` 时代是字符串，迁移时请改成整数。

---

## `skills-manage.lock.json`（v2）

自动生成，**不要手改**：

```json
{
  "version": 2,
  "skills": {
    "skill-creator": {
      "source": {                                   // 完整 Source 对象（嵌套），不是短形式字符串
        "type": "github",
        "repo": "anthropics/skills",
        "subpath": "skills/skill-creator",
        "ref": "main"
      },
      "category": "tools/meta",
      "skillFolderHash": "ad4f350be137206c..."      // SHA-256 hex（64 字符）
    },
    "my-local-skill": {
      "source": {
        "type": "local",
        "path": "/abs/path/to/skill"
      },
      "category": "personal",
      "skillFolderHash": "..."
    }
  }
}
```

> **Breaking vs Python `sm` v0.2.2**：Go 版锁文件精简为 3 个字段（`source`、`category`、`skillFolderHash`）。Python 版还有 `sourceType`、`sourceUrl`、`ref`、`skillPath`、`installedAt`、`updatedAt`、`localPath`，**这些字段在 Go 版不再写入**。读取旧锁文件兼容（多余字段忽略），但新生成的锁文件更精简。

---

## 字段约定（manifest）

### `source` 对象

| 字段 | 必填 | 类型 | 适用 type | 说明 |
|---|---|---|---|---|
| `type` | ✅ | string | - | `github` \| `gitlab` \| `git` \| `local` |
| `repo` | 与 `url`/`path` 二选一 | string | github, gitlab | 形如 `"owner/repo"`，自动拼成 `https://{github.com,gitlab.com}/<repo>.git` |
| `url` | 与 `repo` 二选一 | string | git | 完整 git URL（任意 host：GitLab / Bitbucket / 自托管 / 内网） |
| `path` | 与 `repo`/`url` 二选一 | string | local | 文件系统路径；支持 `~` |
| `subpath` | ❌ | string | github, gitlab, git | repo 内 SKILL.md 所在子路径（如 `"skills/pdf"`）。省略时 skill-manager 走标准发现路径 |
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
| `self` | **不创建任何东西**；验证 `<expanded-path>/<name>/SKILL.md` 已存在 | **Local source 即 target**，比如 skill 直接住在 agent 读的位置 |

### 路径模板展开规则

> **重要**：所有 mode（包括 `self`）都会**自动追加 `/<skill-name>`**。`mode=self` 不跳过这一步，只跳过 link/copy 那一步。

对任意 mode：

1. 把 `{category}` 替换为 skill 的 `category` 值（缺省则整段消失）
2. 末尾自动追加 `/<skill-name>`，得到最终 `dest`
3. 再根据 `mode` 决定 `dest` 上做什么（symlink / copy / 仅校验）

例子：

| target.path | category | name | mode | dest | 操作 |
|---|---|---|---|---|---|
| `.opencode/skills/{category}` | `tools/document` | `pdf` | symlink | `.opencode/skills/tools/document/pdf` | symlink skill_dir → dest |
| `.opencode/skills/{category}` | `""`（缺省） | `pdf` | symlink | `.opencode/skills/pdf` | symlink skill_dir → dest |
| `~/.config/opencode/skills/{category}` | `personal` | `weekly` | copy | `~/.config/opencode/skills/personal/weekly` | cp -r skill_dir → dest |
| `/abs/path/to/skill-parent` | - | `self-skill` | self | `/abs/path/to/skill-parent/self-skill` | 校验 SKILL.md 存在，不动文件系统 |

> **`self` 的典型用例**：当你已有目录 `/my/project/skills/foo/`（里面有 `SKILL.md`），你想 track 它存在但不复制——用 `mode: "self"`，把 `path` 写成**父目录** `/my/project/skills/{category}`，代码会展开为 `/my/project/skills/foo/`。

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
→ cache: `~/.skills-manage/github.com/anthropics/skills/`

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
→ cache: `~/.skills-manage/gitlab.com/group/sub/skills/`

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

→ cache: `~/.skills-manage/gitlab.inner.com/team/main-skills/`

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
→ install：默认 symlink 到 `<target>` 下；路径若与 target 重叠则自动跳过

---

## `skills-manage.lock.json` 字段

### `skillFolderHash`

- 算法：`SHA-256` over（按相对路径排序的）所有文件 `relative_path + \0 + mode_string + \0 + content`
- 用处：`skill-manager verify` 校验本地 cache 没被篡改、上游真的变了能被检出
- 敏感性：**对内容、文件名、文件 mode 都敏感**（`chmod` 之后 hash 会变）；**对目录 mtime、遍历顺序不敏感**（路径先排序再 hash）
- 不依赖网络（与官方 npx skills 的 GitHub Tree SHA 路线不同——我们用本地 hash，更快、可离线）

### `source`

存的是**完整的 Source 对象**（嵌套），不是字符串。`source.type` 决定如何解释：

- `type: "github"` — 主要看 `source.repo` + 可选 `source.host`
- `type: "local"` — 主要看 `source.path`
- `type: "git"` — 看 `source.url`

### `category`

安装时的 category，**用于将来路径回填**（当前 `verify`/`outdated` 不依赖它，但保留以便扩展）。

---

## 完整示例

```json
{
  "$schema": "https://github.com/yxc023/skill-manager/blob/main/SCHEMA.md",
  "version": 2,
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
          "path": "./docs/skills",
          "mode": "self"
        }
      ],
      "description": "skill 已在目标位置存在，不需复制"
    }
  }
}
```

> `self-hosted-doc` 这个 skill：source 在 `./docs/skills/doc-generator/`，用 `mode: "self"` 加 `path: "./docs/skills"`（父目录），最终 `dest = ./docs/skills/doc-generator`，系统只校验 `dest/SKILL.md` 存在。

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

`skill-manager install` 在检测到 v1 manifest（没有 `source` 嵌套对象）时直接报错，不会自动 migrate（避免静默改写你的文件）。

---

## 与 npx skills v3 对齐

skill-manager v2 lockfile 与 npx skills v3 字段对照：

| npx skills v3 字段 | skill-manager v2 字段 | 差异 |
|---|---|---|
| `source` | `source`（嵌套对象） | 形态不同（skill-manager 是对象，npx skills v3 有时是字符串） |
| `sourceType` | `source.type` | skill-manager 把 type 内嵌在 source 里 |
| `sourceUrl` | `source.url`（git 类型时） | 同上 |
| `ref` | `source.ref` | 同上 |
| `skillPath` | `source.subpath` | 同上 |
| `skillFolderHash` (GitHub Tree SHA) | `skillFolderHash` (本地 SHA-256) | **算法不同**（网络 vs 本地） |
| `installedAt` | - | skill-manager 不写入此字段 |
| `updatedAt` | - | skill-manager 不写入此字段 |
| `pluginName` | - | skill-manager 不分组 plugins（YAGNI） |
| `localPath` | `source.path`（local 类型时） | skill-manager 把 path 内嵌在 source 里 |
| `category` (skill-manager 特有) | `category` | skill-manager 独有，定位子目录 |

---

## 与其他工具对照

| 工具 | manifest 字段 | source 支持 | 内容 hash pin |
|---|---|---|---|
| **skill-manager v2** | `source{}`, `category`, `targets[].mode` | github / gitlab / git / local | ✓（本地 SHA-256，含 mode bits） |
| npx skills | `source{}` + `skillPath` + `ref` | github / mintlify / huggingface / local / well-known | ✓（GitHub Tree SHA） |
| skillfile | `source`, `repo`/`url`, `subpath`, `ref`, `install_to` | github / git | ❌ |
| vcspull | path-as-key + `repo`, `remotes`, `branch`, `subdir`, `depth` | 任意 git | ❌（仅 depth 浅克隆） |
| dfetch | `url`, `revision`, `dest` | 任意 git | ❌ |

skill-manager 是「npx skills 风格的字段 + 分层安装能力 + 跨 host 支持 + 离线 hash」的中间态。