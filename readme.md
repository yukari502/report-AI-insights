# ECO 银行周报自动化系统技术文档

本文档详细描述 **ECO 银行周报** 的完整架构、工作流、核心组件及维护指南。

---

## 📋 目录

1. [项目概述](#1-项目概述)
2. [技术栈](#2-技术栈)
3. [目录结构](#3-目录结构)
4. [工作流程](#4-工作流程)
5. [核心组件详解](#5-核心组件详解)
6. [前端架构](#6-前端架构)
7. [自动化部署](#7-自动化部署)
8. [维护指南](#8-维护指南)

---

## 1. 项目概述

这是一个基于 **Go + LLM + GitHub Actions** 的自动化银行研报生成与发布系统。
系统自动抓取指定金融机构的研报，利用 LLM 进行总结分析，生成周报和月度汇总，并发布为静态网站。

### 核心特性

- ✅ **自动抓取**：自动访问目标金融网页提取正文。
- ✅ **智能总结**：调用 LLM (OpenAI 兼容接口) 生成客观的中文摘要。
- ✅ **周期报告**：支持周报（单篇总结）和月报（月度聚合分析）。
- ✅ **静态生成**：内置 Go 静态站点生成器 (SSG)，生成 SEO 友好的 HTML。
- ✅ **自动化**：通过 GitHub Actions 定时运行。

---

## 2. 技术栈

### 后端 (Go)
- **Go 1.21+**
- `go-readability`：网页正文提取
- `net/http`：API 调用与网络请求
- `html/template`：HTML 生成

### AI / LLM
- **Prompts**：精心设计的中文 Prompt，强调客观性和数据分析。
- **Interface**：兼容 OpenAI Chat Completion API。

### 前端
- **HTML5 / CSS3 (Dark Mode)**
- **Marked.js**：Markdown 前端渲染（同时也支持预渲染）。
- **Highlight.js**：代码高亮。

---

## 3. 目录结构

```plaintext
.
├── .github/workflows/         # [CI/CD] GitHub Actions 配置
│   ├── weekly.yml             # 每周运行配置
│   └── monthly.yml            # 每月运行配置
│
├── cmd/app/
│   └── main.go                # [入口] 程序主入口
│
├── internal/
│   ├── config/                # [配置] 环境变量加载
│   ├── crawler/               # [爬虫] 网页内容抓取
│   ├── llm/                   # [AI] LLM 客户端与 Prompt
│   ├── report/                # [业务] 报告生成流程控制
│   └── site/                  # [SSG] 静态网站生成器
│
├── data/
│   └── posts/                 # [数据] 生成的 Markdown 报告文件 (含 Frontmatter)
│
└── web/                       # [前端] 网站资源与输出目录
    ├── index.html             # 主页
    ├── article-template.html  # 文章页模板 (构建时使用)
    ├── style.css              # 样式表
    ├── script.js              # 前端逻辑
    ├── posts/                 # [生成物] 静态 HTML 文章页
    └── Index/                 # [生成物] JSON 索引数据
```

---

## 4. 工作流程

### 4.1 周报生成流程 (`--mode weekly`)

1. **加载配置**：读取 `TARGET_URLS` 和 LLM API Key。
2. **并发爬取**：对每个 URL 进行抓取 (`crawler`)。
3. **智能总结**：将正文发送给 LLM，生成中文 Markdown 摘要 (`llm`)。
4. **保存数据**：在 `data/posts/` 生成带有 Frontmatter 的 `.md` 文件。
   - 命名格式：`YYYY-MM-DD-Title.md`
5. **构建网站**：运行静态生成器 (`site`)，更新 HTML 和 JSON 索引。

### 4.2 月报生成流程 (`--mode monthly`)

1. **聚合数据**：扫描 `data/posts/` 下当月的所有周报。
2. **综合分析**：将当月所有摘要发送给 LLM，生成趋势分析报告。
3. **保存报告**：生成 `YYYY-MM-Monthly_Analysis.md`。
4. **构建网站**：重新构建整个静态站点。

---

## 5. 核心组件详解

### 5.1 爬虫模块 (`internal/crawler`)

负责从杂乱的网页中提取纯净的正文。
- **`FetchContent(url string) (*Article, error)`**
  - 使用 `readability` 算法去除广告和导航。
  - 设置 5秒 超时，防止长时间阻塞。

### 5.2 LLM 客户端 (`internal/llm`)

负责与 AI 模型交互。
- **`Client.Summarize(content string)`**
  - 使用 `WeeklyPromptTemplate`。
  - 强制要求：无个人观点、中文输出、Markdown 格式。
- **`Client.AnalyzeMonthly(summaries string)`**
  - 使用 `MonthlyPromptTemplate`。
  - 聚焦：不同机构观点的对比、未来趋势预测。

### 5.3 报告生成器 (`internal/report`)

协调爬虫和 LLM 的业务逻辑层。
- **`Generator.GenerateWeekly()`**
  - 并发执行（Goroutines）提高效率。
  - 错误隔离：单个 URL 失败不影响整体运行。
- **`Generator.GenerateMonthly()`**
  - 文件过滤器：只处理当前月份 (`YYYY-MM`) 的文件。

### 5.4 静态站点生成器 (`internal/site`)

仿照 Python 脚本逻辑的 Go 实现，负责将 Markdown 转换为 Web 页面。

**关键函数：**

1. **`GenerateSite(postsDir, webDir string)`**
   - 入口函数，串联扫描、索引、HTML 生成、Sitemap 生成。

2. **`scanArticles(sourceDir, webDir)`**
   - 扫描 `data/posts`，解析 Frontmatter。
   - 识别文章类型（Weekly/Monthly）。
   - 按日期降序排序。

3. **`generateIndices(articles, webDir)`**
   - 生成 `articles.json` (全量索引)。
   - 生成 `Index/index_YYYY.json` (按年份分片，提高加载性能)。
   - 生成 `Index/index.json` (主索引)。

4. **`generateHTMLs(articles, webDir)`**
   - 读取 `web/article-template.html`。
   - 预渲染 Markdown (通过 Frontmatter 分离 body)。
   - 处理特殊字符转义 (防止 XSS 和 JS 冲突)。
   - 输出到 `web/posts/{Category}/{Slug}.html`。

5. **`generateSitemap(articles, webDir)`**
   - 生成标准 `sitemap.xml`，利于 SEO。

---

## 6. 前端架构

网站采用 **混合渲染 (Hybrid Rendering)** 模式：

1. **列表页 (SPA 模式)**
   - `index.html` 加载时请求 `Index/index.json`。
   - 动态渲染文章列表，支持按年份加载。
   
2. **文章页 (Static 模式)**
   - 通过 Go 生成的纯 HTML 文件。
   - 包含预填充的 SEO Meta 标签。
   - 内容区域预置了 Markdown 源码（隐藏），由客户端 `marked.js` 激活高亮（如果需要）或直接展示预渲染内容（当前实现为客户端解析嵌入的 Source）。

---

## 7. 自动化部署

### GitHub Secrets 配置

在仓库 Settings -> Secrets and variables -> Actions 中配置：

| 变量名 | 必填 | 描述 |
|--------|-----|------|
| `LLM_API_KEY` | ✅ | LLM 提供商的 API Key |
| `EMAIL_USER` | ❌ | (已移除) |
| `EMAIL_PASS` | ❌ | (已移除) |

### GitHub Variables 配置

| 变量名 | 描述 |
|--------|------|
| `LLM_API_URL` | LLM API 地址 (如 `https://api.openai.com/v1`) |
| `LLM_MODEL` | 模型名称 (如 `gpt-4o`) |
| `TARGET_URLS` | 需要抓取的 URL 列表 (逗号分隔) |

### 定时任务

- **Weekly**: 每周六 00:00 UTC 运行。
- **Monthly**: 每月 1日 00:00 UTC 运行。

---

## 8. 维护指南

### 本地运行

```bash
# 安装依赖
go mod tidy

# 运行周报生成 (会生成数据但因无 Key 可能失败，建议配置 .env)
go run cmd/app/main.go --mode weekly

# 仅重新生成网站 (需先有 data/posts 数据)
# 暂时未开放单独指令，可修改 main.go 或直接运行 weekly 模式(会跳过已存在的抓取吗? 否，会覆盖)
```

### 添加新文章 (手动)

只需在 `data/posts/` 下创建一个 Markdown 文件，包含以下 Frontmatter 即可：

```yaml
---
title: "手动添加的报告"
date: 2025-12-07
source: "人工录入"
url: ""
---

# 文章标题

这里是正文内容...
```

下次运行构建时，该文章会被自动收录到网站中。
