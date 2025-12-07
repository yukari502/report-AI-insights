package site

import (
	"encoding/json"
	"fmt"
	"html"
	"log"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

// Article structure for JSON index
type ArticleIndex struct {
	Title       string `json:"title"`
	Description string `json:"description"`
	Path        string `json:"path"` // Relative path to source md (but we might just use posts/filename.md)
	Date        string `json:"date"`
	Category    string `json:"category"`
	Slug        string `json:"slug"`
	URL         string `json:"url"` // Relative URL to generated html
}

type YearIndex struct {
	Year     string         `json:"year"`
	Articles []ArticleIndex `json:"articles"`
}

type MainIndex struct {
	Updated string   `json:"updated"`
	Years   []string `json:"years"`
}

// GenerateSite performs the full site generation process
// 1. Scan data/posts/*.md
// 2. Generate articles.json and Index/
// 3. Generate HTML pages in web/posts/
// 4. Generate sitemap.xml
func GenerateSite(postsDir, webDir string) error {
	log.Println("Starting Site Generation...")

	articles, err := scanArticles(postsDir, webDir)
	if err != nil {
		return fmt.Errorf("scan error: %w", err)
	}

	if err := generateIndices(articles, webDir); err != nil {
		return fmt.Errorf("index error: %w", err)
	}

	if err := generateHTMLs(articles, webDir); err != nil {
		return fmt.Errorf("html error: %w", err)
	}

	if err := generateSitemap(articles, webDir); err != nil {
		return fmt.Errorf("sitemap error: %w", err)
	}

	log.Println("Site Generation Complete.")
	return nil
}

func scanArticles(sourceDir, webDir string) ([]ArticleIndex, error) {
	entries, err := os.ReadDir(sourceDir)
	if err != nil {
		return nil, err
	}

	var articles []ArticleIndex

	for _, entry := range entries {
		if !strings.HasSuffix(entry.Name(), ".md") {
			continue
		}

		info, err := entry.Info()
		if err != nil {
			continue
		}

		content, err := os.ReadFile(filepath.Join(sourceDir, entry.Name()))
		if err != nil {
			log.Printf("Failed to read %s: %v", entry.Name(), err)
			continue
		}

		fm := parseFrontMatter(string(content))
		title := fm["title"]
		if title == "" {
			title = strings.TrimSuffix(entry.Name(), ".md")
		}
		date := fm["date"]
		if date == "" {
			date = info.ModTime().Format("2006-01-02")
		}

		slug := strings.TrimSuffix(entry.Name(), ".md")

		// Determine category/type
		category := "Uncategorized"
		if strings.Contains(slug, "Monthly_Analysis") {
			category = "Monthly"
		} else {
			category = "Weekly"
		}

		articles = append(articles, ArticleIndex{
			Title:       title,
			Description: title,                                        // Use title as desc for now
			Path:        filepath.Join("data", "posts", entry.Name()), // Keep ref to source
			Date:        date,
			Category:    category,
			Slug:        slug,
			URL:         fmt.Sprintf("posts/%s/%s.html", category, slug),
		})
	}

	// Sort by date desc
	sort.Slice(articles, func(i, j int) bool {
		return articles[i].Date > articles[j].Date
	})

	return articles, nil
}

func generateIndices(articles []ArticleIndex, webDir string) error {
	// articles.json
	data, _ := json.MarshalIndent(articles, "", "  ")
	if err := os.WriteFile(filepath.Join(webDir, "articles.json"), data, 0644); err != nil {
		return err
	}

	// Index/index.json and Index/index_YYYY.json
	yearsMap := make(map[string][]ArticleIndex)
	for _, a := range articles {
		y := a.Date[:4]
		yearsMap[y] = append(yearsMap[y], a)
	}

	var years []string
	for y, arts := range yearsMap {
		years = append(years, y)
		idx := YearIndex{
			Year:     y,
			Articles: arts,
		}
		yData, _ := json.MarshalIndent(idx, "", "  ")
		if err := os.WriteFile(filepath.Join(webDir, "Index", fmt.Sprintf("index_%s.json", y)), yData, 0644); err != nil {
			return err
		}
	}
	sort.Sort(sort.Reverse(sort.StringSlice(years)))

	mainIdx := MainIndex{
		Updated: time.Now().Format("2006-01-02 15:04:05"),
		Years:   years,
	}
	mData, _ := json.MarshalIndent(mainIdx, "", "  ")
	return os.WriteFile(filepath.Join(webDir, "Index", "index.json"), mData, 0644)
}

func generateHTMLs(articles []ArticleIndex, webDir string) error {
	tmplBytes, err := os.ReadFile(filepath.Join(webDir, "article-template.html"))
	if err != nil {
		return fmt.Errorf("missing template: %w", err)
	}
	tmplStr := string(tmplBytes)

	for _, a := range articles {
		// Read source markdown
		srcPath := a.Path // This is relative to root, e.g. data/posts/foo.md
		mdBytes, err := os.ReadFile(srcPath)
		if err != nil {
			log.Printf("Skipping %s, cannot read source: %v", a.Slug, err)
			continue
		}

		// Parse just body (remove frontmatter)
		_, body := splitFrontMatter(string(mdBytes))

		// Escape content for JS embedding
		escapedBody := strings.ReplaceAll(body, "`", "\\`")
		escapedBody = strings.ReplaceAll(escapedBody, "</script>", "<\\/script>")

		// Replacements
		out := tmplStr
		out = strings.ReplaceAll(out, "{TITLE}", html.EscapeString(a.Title))
		out = strings.ReplaceAll(out, "{DESCRIPTION}", html.EscapeString(a.Description))
		out = strings.ReplaceAll(out, "{DATE}", a.Date)
		out = strings.ReplaceAll(out, "{CATEGORY}", a.Category)
		out = strings.ReplaceAll(out, "{SLUG}", a.Slug)
		out = strings.ReplaceAll(out, "{CONTENT}", escapedBody)

		// Calculate Root Path
		// URL is posts/{category}/{slug}.html -> depth 2 -> ../../
		out = strings.ReplaceAll(out, "{ROOT_PATH}", "../../")

		destDir := filepath.Join(webDir, "posts", a.Category)
		if err := os.MkdirAll(destDir, 0755); err != nil {
			return err
		}

		destFile := filepath.Join(destDir, a.Slug+".html")
		if err := os.WriteFile(destFile, []byte(out), 0644); err != nil {
			log.Printf("Failed to write html %s: %v", destFile, err)
		}
	}
	return nil
}

func generateSitemap(articles []ArticleIndex, webDir string) error {
	baseUrl := "https://kaze.github.io/eco_report" // TODO: Make configurable

	var sb strings.Builder
	sb.WriteString(`<?xml version="1.0" encoding="UTF-8"?>
<urlset xmlns="http://www.sitemaps.org/schemas/sitemap/0.9">
`)

	for _, a := range articles {
		sb.WriteString(fmt.Sprintf(`  <url>
    <loc>%s/%s</loc>
    <lastmod>%s</lastmod>
  </url>
`, baseUrl, a.URL, a.Date))
	}
	sb.WriteString("</urlset>")

	return os.WriteFile(filepath.Join(webDir, "sitemap.xml"), []byte(sb.String()), 0644)
}

func parseFrontMatter(content string) map[string]string {
	fm := make(map[string]string)
	lines := strings.Split(content, "\n")
	if len(lines) == 0 || strings.TrimSpace(lines[0]) != "---" {
		return fm
	}

	for i := 1; i < len(lines); i++ {
		line := strings.TrimSpace(lines[i])
		if line == "---" {
			break
		}
		parts := strings.SplitN(line, ":", 2)
		if len(parts) == 2 {
			key := strings.TrimSpace(parts[0])
			val := strings.TrimSpace(parts[1])
			val = strings.Trim(val, `"'`)
			fm[key] = val
		}
	}
	return fm
}

func splitFrontMatter(content string) (string, string) {
	parts := strings.SplitN(content, "---", 3)
	if len(parts) >= 3 {
		return parts[1], parts[2]
	}
	return "", content
}
