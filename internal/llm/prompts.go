package llm

// WeeklyPromptTemplate is the prompt for weekly summaries.
// It expects the article content as an argument.
const WeeklyPromptTemplate = `
请阅读以下金融研究文章，并进行总结分析。
目标：作为对Research或Review文章的忠实总结概括，不允许有你自己的看法。
语言：中文 (Markdown格式)

要求：
1. 注重各大银行和金融机构对整个事件的看法。
2. 包括对未来一年或者半年走势的推测。
3. 涵盖行业形势研究、局势安全、进出口、PPI、CPI等各种看法。
4. 将其中的推理（如果有）体现出来，展示前因后果。

文章内容：
%s
`

// MonthlyPromptTemplate is the prompt for monthly analysis.
// It expects the aggregated weekly summaries.
const MonthlyPromptTemplate = `
请分析以下各大银行的Research文章总结（月度汇总）。

要求：
1. 分析各金融机构对相同事件的看法（相同或不同点）。
2. 如果需要，用表格分类列举不同机构的看法。
3. 聚焦对未来走势的预测和关键经济指标的判断。
4. 语言：中文 (Markdown格式)

汇总内容：
%s
`
