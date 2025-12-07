# Research & Reports - AI Insights

本文档详细描述 **Research & Reports - AI Insights** 的完整架构、工作流、核心组件及维护指南。

---

## 📋 目录

1. [项目概述](#1-项目概述)
2. [技术栈](#2-技术栈)
3. [系统架构与目录](#3-系统架构与目录)
4. [核心流程详解](#4-核心流程详解)
5. [配置指南](#5-配置指南)
6. [前端架构 (UI)](#6-前端架构-ui)
7. [维护与排错](#7-维护与排错)

---

## 1. 项目概述

**Research & Reports - AI Insights** (原 ECO) 是一个全自动化的金融研报聚合与分析系统。它旨在解决人工阅读海量银行研报效率低下的问题，通过自动化爬虫和 LLM 技术，实时捕捉、总结并发布各大金融机构（如 Citi, UBS）的市场观点。

### 核心能力

- 🤖 **AI 智能捕获**：使用 `chromedp` 无头浏览器配合 LLM 智能识别网页中的文章链接，不再依赖死板的 CSS 选择器。
- 🧠 **双层 LLM 架构**：
  - **Crawler Model** (`gemini-2.0-flash`)：快速、低成本地进行内容发现与筛选。
  - **Analyzer Model** (`gemini-1.5-pro`)：高质量地进行金融内容总结与翻译。
- 🛡️ **智能去重**：基于 `data/history.json` 的持久化历史记录，防止重复抓取和处理。
- 📊 **多维报告**：支持“单篇即时研报”与“月度聚合分析”两种模式。
- 🎨 **金融终端 UI**：打造类似 Bloomberg/Eikon 的专业金融终端风格（Deep Navy & Gold），提供极致阅读体验。

---

## 2. 技术栈

| 领域 | 技术/工具 | 说明 |
|------|-----------|------|
| **后端 Core** | **Go 1.23+** | 高并发爬虫、逻辑控制、静态站点生成 |
| **浏览器自动化** | **Chromedp** | 控制 Headless Chrome 进行动态网页渲染 |
| **内容提取** | **go-readability** | 智能提取网页正文，去除广告杂质 |
| **AI 模型** | **Gemini 2.0 Flash / Pro** | Google 最新多模态模型，支持超长上下文 |
| **前端** | **HTML5 / CSS3 / ES6** | 原生开发，无繁重框架，追求极致加载速度 |
| **渲染引擎** | **Marked.js + Highlight.js** | 客户端 Markdown 渲染与代码高亮 |
| **自动化** | **GitHub Actions** | 定时任务 (Cron) 触发周报/月报构建 |

---

## 3. 系统架构与目录

系统采用模块化设计，各组件职责单一，易于维护。

```plaintext
.
├── .github/workflows/         # [CI/CD] 自动化流水线配置
│   ├── weekly.yml             # 周报任务 (Crawl -> Summarize -> Site)
│   └── monthly.yml            # 月报任务 (Analyze -> Site)
│
├── internal/
│   ├── config/                # [配置] 环境变量与 banks.json 加载
│   ├── crawler/               # [核心] AI 智能爬虫实现
│   ├── history/               # [数据] 历史记录与去重管理 (History Manager)
│   ├── llm/                   # [AI] LLM 客户端与 Prompt 模板
│   ├── report/                # [业务] 报告生成器 (调度器)
│   └── site/                  # [Web] 静态站点生成器 (SSG)
│
├── data/
│   ├── cache/                 # [缓存] 原始网页抓取内容 (JSON)
│   ├── posts/                 # [数据] 处理后的 Markdown 研报 (含 Metadata)
│   ├── history.json           # [持久化] 已处理 URL 的去重记录
│   └── banks.json             # [映射] 域名 -> 银行名称配置
│
├── Index/                     # [产物] 生成的静态网站索引与文章页
├── web/                       # [资源] 前端静态资源 (可移除，现已整合至根目录)
├── main.go                    # [入口] 程序主入口 CLI
├── index.html                 # [前端] Dashboard 首页
├── article-template.html      # [前端] 文章详情页模板
├── style.css                  # [前端] 金融主题样式表
└── script.js                  # [前端] 交互逻辑 (搜索、渲染)
```

---

## 4. 核心流程详解

系统支持三种主要命令模式，对应不同的业务场景。

### 4.1 周报模式 (`weekly`)

这是最高频运行的模式，通常由 GitHub Actions 每周触发（或手动触发）。推荐分步执行以提高稳健性。

1.  **Step 1: 抓取 (Crawl)**
    *   命令：`go run main.go --mode weekly --step crawl`
    *   逻辑：
        *   加载 `data/history.json` 排除已处理 URL。
        *   启动 Headless Chrome 渲染 `TARGET_URLS` 索引页。
        *   将页面文本投喂给 **Crawler LLM**，提取符合条件的最新文章链接。
        *   下载文章正文，保存至 `data/cache/`。
        *   更新历史记录，标记为 `Crawled`。

2.  **Step 2: 总结 (Summarize)**
    *   命令：`go run main.go --mode weekly --step summarize`
    *   逻辑：
        *   扫描 `data/cache/` 中的未处理文件。
        *   检查 `data/history.json` 确保未被总结过。
        *   调用 **Analyzer LLM** 生成中文 Markdown 摘要（严格遵循客观事实）。
        *   自动分类（根据 `banks.json` 识别 Source）。
        *   生成 Markdown 文件至 `data/posts/{Bank}/`。
        *   调用 `GenerateSite` 立即刷新网站。

3.  **Step 3: 建站 (Site)**
    *   命令：`go run main.go --mode weekly --step site`
    *   逻辑：
        *   扫描 `data/posts` 所有 Markdown。
        *   生成 `Index/index.json` (前端搜索索引)。
        *   使用 `article-template.html` 渲染静态 HTML 页面。

### 4.2 月报模式 (`monthly`)

*   命令：`go run main.go --mode monthly`
*   逻辑：聚合当月所有周报，通过 LLM 分析宏观趋势，生成由于“拼接”而成的深度月度报告，并发布。

---

## 5. 配置指南

系统高度可配置，主要通过环境变量 (`.env` 或 GitHub Secrets) 控制。

| 变量名 | 必填 | 说明 | 示例值 |
|--------|-----|------|--------|
| `LLM_API_KEY` | ✅ | Gemini API Key | `AIzaSy...` |
| `LLM_CRAWLER_API_URL` | ✅ | 爬虫模型 API 地址 | `https://generativelanguage.googleapis.com/v1beta/models` |
| `LLM_CRAWLER_MODEL` | ✅ | 爬虫模型名称 | `gemini-2.0-flash` (速度快) |
| `LLM_ANALYZER_API_URL` | ✅ | 分析模型 API 地址 | `https://generativelanguage.googleapis.com/v1beta/models` |
| `LLM_ANALYZER_MODEL` | ✅ | 分析模型名称 | `gemini-1.5-pro` (更聪明) |
| `TARGET_URLS` | ✅ | 监控的网页入口，逗号分隔 | `https://site1.com,https://site2.com` |
| `OUTPUT_LANGUAGE` | ❌ | 输出语言 | `Chinese` (默认) |

### 银行匹配配置 (`data/banks.json`)

系统根据 URL 自动归类文章来源。如需添加新银行，请更新此文件：

```json
{
  "ubs.com": "UBS",
  "citigroup.com": "Citi",
  "goldmansachs.com": "Goldman Sachs"
}
```

---

## 6. 前端架构 (UI)

本次升级引入了全新的 **"Financial Terminal"** 设计语言。

- **配色方案**：
  - **背景**：Deep Navy (`#0f172a`) —— 象征稳健与专业。
  - **强调色**：Bloomberg Gold (`#fbbf24`) —— 象征价值与洞察。
- **布局架构**：
  - **Sidebar**：侧边栏导航，自动根据数据源 (Source) 分组。
  - **Grid**：卡片式网格布局，展示最新研报。
- **技术实现**：
  - **混合渲染**：为了 SEO 与性能，文章页为**纯静态 HTML**；为了交互体验，列表页为**客户端渲染 (CSR)**。
  - **客户端 Markdown 解析**：文章页 HTML 仅包含 Shell 和隐藏的 Markdown 源码。页面加载时，`script.js` 调用 `marked.js` 将源码实时渲染为富文本。这保证了样式的高度统一和灵活性。

---

## 7. 维护与排错

### 常见问题

1.  **文章未抓取？**
    *   检查 `data/history.json`，该 URL 是否已被标记为 `crawled`。
    *   如果是误判，可手动从 JSON 中删除该 URL，然后重新运行 `crawl` 步骤。

2.  **生成了重复报告？**
    *   系统通过 URL 进行去重。如果网站 URL 发生微小变化（如参数不同），可能会被视为新文章。
    *   检查 `data/history.json` 确认去重逻辑。

3.  **样式显示异常？**
    *   尝试强制刷新浏览器 (Cmd+Shift+R)，因为 CSS 可能会被缓存。
    *   确保 `style.css` 和 `script.js` 位于根目录。

4.  **如何手动添加文章？**
    *   在 `data/posts/Other/` 下新建 `.md` 文件，头部包含 Frontmatter：
    ```yaml
    ---
    title: "我的手动分析"
    date: 2025-12-08
    category: "Manual"
    url: "https://example.com"
    ---
    ```
    *   运行 `go run main.go --mode weekly --step site` 重建网站。

---

**© 2025 Research & Reports Team**
