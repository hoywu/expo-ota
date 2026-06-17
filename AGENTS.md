# AGENTS.md

---

## 语言规范 (Language)

- 语言偏好：优先使用简体中文与用户进行交流。
- 中英混排：在遇到专业术语、专有名词，或使用英文描述更显简洁、清晰、高效的场景时，直接使用英文，采用最高效的中英文混合方式输出。
- 表达风格：保持语言专业、逻辑清晰且易于理解，避免无谓的比喻与类比。保持客观中立，切勿过度迎合，主动指出用户的错误与非最佳实践。

---

## 行为准则 (Behavioral Guidelines)

对于任何编码 (Coding) 任务，参考 /karpathy-guidelines 来规范你的行为。除非用户明确表示这是一个简单的一次性任务。

---

## 测试规范 (Testing)

- 实现 server 的 API logic（`server/api/*/internal/logic/`）时，必须使用 `mockgen`（go.uber.org/mock）mock 依赖的 model 接口，编写必要的单元测试。
- 修改已有 logic 时，同步更新对应的 `*_test.go`。
- mock 生成在 `server/db/models/` 包内（接口含未导出方法，需 `-source` + `-self_package`），参考已有的 `usersmodel_mock.go` 头部命令。
- 编写 dashboard 组件时，必须使用 `vitest` 编写必要的单元测试。
- Dashboard 相关脚本（dev、build、test、lint 等）定义在 `dashboard/package.json`，不混入根目录 `Makefile`。
- Dashboard 包管理使用 `bun` / `bunx`，禁止使用 `npm` / `npx`。
- 运行测试：server 用 `make test`；dashboard 在 `dashboard/` 下执行 `bun run test:unit`（或对应 package script）。

---

## Agent skills

### Issue tracker

GitHub Issues, accessed via the `gh` CLI. See `docs/agents/issue-tracker.md`.

### Triage labels

Default vocabulary — the five role names are the literal label strings. See `docs/agents/triage-labels.md`.

### Domain docs

Multi-context layout. A `CONTEXT-MAP.md` at the root indexes per-context `CONTEXT.md` files (e.g. `server/CONTEXT.md`, `dashboard/CONTEXT.md`). System-wide ADRs live in `docs/adr/`. See `docs/agents/domain.md`.
