document.addEventListener('DOMContentLoaded', () => {
    // DOM Elements
    const articlesGrid = document.getElementById('articles-grid');
    const searchInput = document.getElementById('search-input');
    const themeToggle = document.getElementById('theme-toggle');
    const categoriesList = document.getElementById('categories-list');
    const categoriesToggle = document.getElementById('categories-toggle');
    const viewTitle = document.getElementById('view-title');
    const sidebarToggle = document.getElementById('sidebar-toggle');
    const sidebar = document.querySelector('.sidebar');

    let allArticles = [];

    // --- Theme Handling ---
    function initTheme() {
        const savedTheme = localStorage.getItem('theme') || 'light';
        document.documentElement.setAttribute('data-theme', savedTheme);

        themeToggle.addEventListener('click', () => {
            const current = document.documentElement.getAttribute('data-theme');
            const next = current === 'dark' ? 'light' : 'dark';
            document.documentElement.setAttribute('data-theme', next);
            localStorage.setItem('theme', next);
        });
    }

    // --- Data Fetching ---
    async function fetchArticles() {
        try {
            // New Hierarchical Index
            const response = await fetch('Index/index.json');
            if (response.ok) {
                const mainIndex = await response.json();
                const articles = [];

                // Fetch each year's index
                for (const year of mainIndex.years) {
                    try {
                        const yearRes = await fetch(`Index/index_${year}.json`);
                        if (yearRes.ok) {
                            const yearData = await yearRes.json();
                            articles.push(...yearData.articles);
                        }
                    } catch (e) {
                        console.warn(`Failed to load year ${year}`, e);
                    }
                }
                return articles.sort((a, b) => new Date(b.date) - new Date(a.date));
            }
        } catch (e) {
            console.error('Failed to load indices', e);
        }
        return [];
    }

    // --- Rendering ---
    function renderArticles(articles) {
        articlesGrid.innerHTML = '';

        if (articles.length === 0) {
            articlesGrid.innerHTML = `
                <div style="grid-column: 1/-1; text-align: center; padding: 4rem; color: var(--text-secondary);">
                    <p style="font-size: 1.2rem;">No reports found matching your criteria.</p>
                </div>`;
            return;
        }

        articles.forEach(article => {
            const card = document.createElement('div');
            card.className = 'card';

            // Map category to icon (simple logic)
            const icon = '📄';

            card.innerHTML = `
                <div class="card-meta">
                    <span>${icon}</span>
                    <span>${article.category}</span>
                    <span style="margin-left: auto; font-size: 0.8rem; opacity: 0.8;">${article.date}</span>
                </div>
                <h3>${article.title}</h3>
                <p class="card-desc">${article.description || 'Click to read full analysis...'}</p>
                <div class="card-footer">Read Analysis</div>
            `;

            card.addEventListener('click', () => {
                window.location.href = article.url;
            });

            articlesGrid.appendChild(card);
        });
    }

    function renderCategories(articles) {
        // Group by Category (Bank)
        const categories = {};
        articles.forEach(art => {
            const cat = art.category || 'Other';
            if (!categories[cat]) categories[cat] = [];
            categories[cat].push(art);
        });

        const sortedCats = Object.keys(categories).sort();

        categoriesList.innerHTML = '';
        sortedCats.forEach(cat => {
            const group = document.createElement('div');
            group.className = 'category-group';

            const header = document.createElement('div');
            header.className = 'category-header';
            header.innerHTML = `
                <svg xmlns="http://www.w3.org/2000/svg" width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round" class="chevron" style="transform: rotate(-90deg); transition: transform 0.2s;"><polyline points="6 9 12 15 18 9"></polyline></svg>
                <span>${cat}</span>
                <span style="margin-left: auto; font-size: 0.7rem; opacity: 0.6; background: var(--bg-tertiary); padding: 2px 6px; border-radius: 10px;">${categories[cat].length}</span>
            `;

            const sublist = document.createElement('div');
            sublist.className = 'category-sublist collapsed';

            // Toggle logic
            header.addEventListener('click', () => {
                sublist.classList.toggle('collapsed');
                const chevron = header.querySelector('.chevron');
                chevron.style.transform = sublist.classList.contains('collapsed') ? 'rotate(-90deg)' : 'rotate(0deg)';

                // Also filter main view to this category
                renderArticles(categories[cat]);
                viewTitle.textContent = `${cat} Reports`;
                window.scrollTo(0, 0);
            });

            // Add recent items to sublist (limit 5)
            categories[cat].slice(0, 5).forEach(art => {
                const link = document.createElement('a');
                link.className = 'article-link';
                link.textContent = art.title;
                link.href = art.url;
                sublist.appendChild(link);
            });

            group.appendChild(header);
            group.appendChild(sublist);
            categoriesList.appendChild(group);
        });
    }

    // --- Search ---
    function initSearch() {
        searchInput.addEventListener('input', (e) => {
            const query = e.target.value.toLowerCase();
            const filtered = allArticles.filter(art =>
                art.title.toLowerCase().includes(query) ||
                (art.description && art.description.toLowerCase().includes(query))
            );

            renderArticles(filtered);
            viewTitle.textContent = query ? `Search: "${query}"` : 'Latest Reports';
        });
    }

    // --- Sidebar Toggle ---
    function initSidebar() {
        if (categoriesToggle) {
            categoriesToggle.addEventListener('click', () => {
                categoriesToggle.classList.toggle('collapsed');
                categoriesList.classList.toggle('collapsed');
            });
        }

        if (sidebarToggle) {
            sidebarToggle.addEventListener('click', () => {
                sidebar.classList.toggle('active');
            });
        }
    }

    // --- Article Rendering (Static) ---
    function initArticlePage() {
        const source = document.getElementById('markdown-source');
        const target = document.getElementById('content');

        if (!source || !target) return;

        if (typeof marked === 'undefined') {
            target.innerHTML = '<p style="color:red">Error: Markdown parser not loaded.</p>';
            return;
        }

        // Configure marked
        marked.setOptions({
            highlight: function (code, lang) {
                if (lang && hljs.getLanguage(lang)) {
                    return hljs.highlight(code, { language: lang }).value;
                }
                return hljs.highlightAuto(code).value;
            },
            breaks: true,
            gfm: true
        });

        const renderer = new marked.Renderer();

        // Custom link renderer to open in new tab
        renderer.link = function (href, title, text) {
            // Handle object token (Marked v5+)
            if (typeof href === 'object' && href !== null) {
                const token = href;
                href = token.href;
                title = token.title;
                text = token.text;
            }
            return `<a href="${href}" target="_blank" rel="noopener noreferrer" title="${title || ''}">${text}</a>`;
        };

        try {
            const html = marked.parse(source.textContent, { renderer: renderer });
            target.innerHTML = html;

            // Re-run highlighting for safety
            document.querySelectorAll('pre code').forEach((el) => {
                hljs.highlightElement(el);
            });

        } catch (e) {
            console.error('Markdown rendering failed:', e);
            target.innerHTML = '<p>Error rendering content.</p>';
        }
    }

    // --- Main Init ---
    async function init() {
        initTheme();
        initSidebar();

        // Check if we are on an article page
        if (document.getElementById('markdown-source')) {
            initArticlePage();
        } else {
            // We are on the dashboard
            allArticles = await fetchArticles();
            renderArticles(allArticles);
            renderCategories(allArticles);
            initSearch();

            // Reset "All Articles" link
            const resetLink = document.querySelector('.nav-link[data-filter="all"]');
            if (resetLink) {
                resetLink.addEventListener('click', (e) => {
                    e.preventDefault();
                    renderArticles(allArticles);
                    viewTitle.textContent = 'Latest Reports';
                    window.scrollTo(0, 0);
                });
            }
        }
    }

    init();
});
