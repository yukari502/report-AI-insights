document.addEventListener('DOMContentLoaded', () => {
    const markdownSource = document.getElementById('markdown-source');

    if (markdownSource) {
        initStaticArticle();
    } else {
        initSPA();
    }
});

function initStaticArticle() {
    const contentDiv = document.getElementById('content');
    const markdownSource = document.getElementById('markdown-source');

    if (typeof marked !== 'undefined') {
        const htmlContent = marked.parse(markdownSource.textContent.trim());
        contentDiv.innerHTML = htmlContent;

        // Highlight code
        if (typeof hljs !== 'undefined') {
            document.querySelectorAll('pre code').forEach((block) => {
                hljs.highlightElement(block);
            });
        }
    }
}

function initSPA() {
    const reportList = document.getElementById('report-list');
    if (!reportList) return;

    fetch('Index/index.json')
        .then(res => res.json())
        .then(async idx => {
            let allArticles = [];
            for (const year of idx.years) {
                try {
                    const yRes = await fetch(`Index/index_${year}.json`);
                    const yData = await yRes.json();
                    allArticles.push(...yData.articles);
                } catch (e) {
                    console.error("Failed to load year index", year, e);
                }
            }

            // Sort keys are already sorted in generation, but verify
            renderArticles(allArticles);
        });
}

function renderArticles(articles) {
    const list = document.getElementById('report-list');
    list.innerHTML = articles.map(art => `
        <li class="report-item">
            <a href="${art.url}">
                <h2>${art.title}</h2>
                <div class="article-meta">
                    <span>${art.date}</span>
                    <span class="category">${art.category}</span>
                </div>
                <p>${art.description || ''}</p>
            </a>
        </li>
    `).join('');
}
