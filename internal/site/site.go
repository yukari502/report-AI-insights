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
	URL         string `json:"url"`          // Relative URL to generated html
	OriginalURL string `json:"original_url"` // Source URL from frontmatter
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
	var articles []ArticleIndex

	err := filepath.WalkDir(sourceDir, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			return nil
		}
		if !strings.HasSuffix(d.Name(), ".md") {
			return nil
		}

		content, err := os.ReadFile(path)
		if err != nil {
			log.Printf("Failed to read %s: %v", path, err)
			return nil
		}

		info, _ := d.Info()
		fm := parseFrontMatter(string(content))
		title := fm["title"]
		if title == "" {
			title = strings.TrimSuffix(d.Name(), ".md")
		}
		date := fm["date"]
		if date == "" {
			date = info.ModTime().Format("2006-01-02")
		}

		category := fm["category"]
		if category == "" {
			// Fallback: use parent directory name if not "posts"
			parent := filepath.Base(filepath.Dir(path))
			if parent != "posts" && parent != "." {
				category = parent
			} else {
				category = "Uncategorized"
			}
		}

		// Special handling for Monthly Analysis
		if strings.Contains(title, "Monthly Analysis") {
			category = "Monthly"
		}

		originalURL := fm["url"]

		slug := strings.TrimSuffix(d.Name(), ".md")

		// Determine relative path for JSON (useful if we want to debug)
		// relPath, _ := filepath.Rel("data", path)
		// We need the full path for os.ReadFile later

		articles = append(articles, ArticleIndex{
			Title:       title,
			Description: title,
			Path:        path, // Store full path e.g. data/posts/Citi/foo.md
			Date:        date,
			Category:    category,
			Slug:        slug,
			URL:         fmt.Sprintf("posts/%s/%s.html", category, slug),
			OriginalURL: originalURL,
		})
		return nil
	})

	if err != nil {
		return nil, err
	}

	// Sort by date desc
	sort.Slice(articles, func(i, j int) bool {
		return articles[i].Date > articles[j].Date
	})

	return articles, nil
}

func generateIndices(articles []ArticleIndex, webDir string) error {
	// Ensure Index directory exists
	if err := os.MkdirAll(filepath.Join(webDir, "Index"), 0755); err != nil {
		return err
	}

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

		// If OriginalURL is empty (e.g. monthly analysis), we might fallback or just leave empty href (handled in template)
		out = strings.ReplaceAll(out, "{ORIGINAL_URL}", a.OriginalURL)

		// Calculate Root Path
		// URL is Index/posts/{category}/{slug}.html -> depth 3 -> ../../../
		out = strings.ReplaceAll(out, "{ROOT_PATH}", "../../../")

		destDir := filepath.Join(webDir, "Index", "posts", a.Category)
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
