# Dashboard Context

Dashboard 管理端前端，采用 Vue 3 生态实现。所有 dashboard 侧的技术约束以本文档为准。

## 技术栈

- Vue 3 + Vite
- Vue Router
- Pinia

**Bun**:
Dashboard 的包管理器统一使用 `bun`；执行一次性 CLI 用 `bunx`。
_Avoid_: npm, npx, yarn, pnpm

## UI 与样式

**Nuxt UI**:
Dashboard 的组件库统一使用 Nuxt UI。
_Avoid_: Element Plus, Ant Design Vue, Naive UI（除非明确发起架构决策）

**Tailwind CSS v4**:
Dashboard 的样式系统统一使用 Tailwind CSS v4。遇到语法或最佳实践不确定时，优先参考官方文档与示例。
_Avoid_: SCSS 变量体系、独立 CSS-in-JS 方案（除非有明确例外决策）

## 动效与数值展示

**Number Animation**:
对于适合用动效提升可读性与观感的数字，统一使用 `@number-flow/vue`。
_Avoid_: 手写数字补间动画、混用多个数字动效库
