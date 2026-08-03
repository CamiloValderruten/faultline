// HTML-to-markdown extraction. This file was extracted from executor.go
// to give the matching html_test.go a natural source counterpart and
// shrink the otherwise 4k-line executor.

package tools

import (
	"fmt"
	"net/url"
	"strings"

	"golang.org/x/net/html"
)

// htmlExtractor walks an HTML node tree and produces clean markdown text.
type htmlExtractor struct {
	sb        strings.Builder
	baseURL   *url.URL
	listStack []listContext
	inPre     bool
	cellIndex int
}

type listContext struct {
	ordered bool
	index   int
}

// extractTextFromHTML parses HTML and renders it as readable markdown text.
// baseURL is used to resolve relative links.
func extractTextFromHTML(rawHTML, baseURL string) string {
	doc, err := html.Parse(strings.NewReader(rawHTML))
	if err != nil {
		return basicStripTags(rawHTML)
	}

	base, _ := url.Parse(baseURL)

	root := findContentRoot(doc)
	ext := &htmlExtractor{baseURL: base}
	ext.walkChildren(root)

	return cleanupText(ext.sb.String())
}

// findContentRoot returns the best content node: <main>, then a sole
// <article>, then <body>, then the document itself.
func findContentRoot(doc *html.Node) *html.Node {
	if n := findElement(doc, "main"); n != nil {
		return n
	}
	articles := findElements(doc, "article")
	if len(articles) == 1 {
		return articles[0]
	}
	if n := findElement(doc, "body"); n != nil {
		return n
	}
	return doc
}

func findElement(n *html.Node, tag string) *html.Node {
	if n.Type == html.ElementNode && n.Data == tag {
		return n
	}
	for c := n.FirstChild; c != nil; c = c.NextSibling {
		if found := findElement(c, tag); found != nil {
			return found
		}
	}
	return nil
}

func findElements(n *html.Node, tag string) []*html.Node {
	var results []*html.Node
	if n.Type == html.ElementNode && n.Data == tag {
		results = append(results, n)
	}
	for c := n.FirstChild; c != nil; c = c.NextSibling {
		results = append(results, findElements(c, tag)...)
	}
	return results
}

func (e *htmlExtractor) walk(n *html.Node) {
	if n.Type == html.ElementNode {
		// Skip noise elements
		switch n.Data {
		case "script", "style", "noscript", "nav", "footer", "aside",
			"form", "svg", "iframe", "button", "input", "select", "textarea":
			return
		}
		for _, attr := range n.Attr {
			if attr.Key == "hidden" {
				return
			}
			if attr.Key == "aria-hidden" && attr.Val == "true" {
				return
			}
			if attr.Key == "style" && strings.Contains(attr.Val, "display:none") {
				return
			}
		}
	}

	// Text nodes
	if n.Type == html.TextNode {
		text := n.Data
		if e.inPre {
			e.sb.WriteString(text)
		} else {
			e.sb.WriteString(collapseWhitespace(text))
		}
		return
	}

	if n.Type != html.ElementNode {
		e.walkChildren(n)
		return
	}

	// Element handling
	switch n.Data {
	// --- Headings ---
	case "h1", "h2", "h3", "h4", "h5", "h6":
		level := int(n.Data[1] - '0')
		e.sb.WriteString("\n\n")
		e.sb.WriteString(strings.Repeat("#", level))
		e.sb.WriteString(" ")
		e.walkChildren(n)
		e.sb.WriteString("\n\n")

	// --- Block elements ---
	case "p":
		e.sb.WriteString("\n\n")
		e.walkChildren(n)
		e.sb.WriteString("\n\n")
	case "div", "article", "section", "main", "header", "figure":
		e.sb.WriteString("\n")
		e.walkChildren(n)
		e.sb.WriteString("\n")
	case "figcaption":
		e.sb.WriteString("\n*")
		e.walkChildren(n)
		e.sb.WriteString("*\n")

	// --- Blockquote ---
	case "blockquote":
		inner := &htmlExtractor{baseURL: e.baseURL}
		inner.walkChildren(n)
		text := strings.TrimSpace(inner.sb.String())
		e.sb.WriteString("\n\n")
		for _, line := range strings.Split(text, "\n") {
			e.sb.WriteString("> ")
			e.sb.WriteString(line)
			e.sb.WriteString("\n")
		}
		e.sb.WriteString("\n")

	// --- Preformatted / code ---
	case "pre":
		e.sb.WriteString("\n\n```\n")
		e.inPre = true
		e.walkChildren(n)
		e.inPre = false
		e.sb.WriteString("\n```\n\n")
	case "code":
		if !e.inPre {
			e.sb.WriteString("`")
			e.walkChildren(n)
			e.sb.WriteString("`")
		} else {
			e.walkChildren(n)
		}

	// --- Links ---
	case "a":
		href := getAttr(n, "href")
		text := nodeText(n)
		if strings.HasPrefix(href, "#") || strings.HasPrefix(href, "javascript:") {
			href = ""
		}
		if href != "" && text != "" {
			e.sb.WriteString("[")
			e.sb.WriteString(text)
			e.sb.WriteString("](")
			e.sb.WriteString(e.resolveHref(href))
			e.sb.WriteString(")")
		} else if text != "" {
			e.sb.WriteString(text)
		}

	// --- Inline formatting ---
	case "strong", "b":
		e.sb.WriteString("**")
		e.walkChildren(n)
		e.sb.WriteString("**")
	case "em", "i":
		e.sb.WriteString("*")
		e.walkChildren(n)
		e.sb.WriteString("*")
	case "del", "s":
		e.sb.WriteString("~~")
		e.walkChildren(n)
		e.sb.WriteString("~~")

	// --- Line break / horizontal rule ---
	case "br":
		e.sb.WriteString("\n")
	case "hr":
		e.sb.WriteString("\n\n---\n\n")

	// --- Images ---
	case "img":
		alt := getAttr(n, "alt")
		if alt != "" {
			e.sb.WriteString("[image: ")
			e.sb.WriteString(alt)
			e.sb.WriteString("]")
		}

	// --- Lists ---
	case "ul":
		e.sb.WriteString("\n")
		e.listStack = append(e.listStack, listContext{ordered: false})
		e.walkChildren(n)
		e.listStack = e.listStack[:len(e.listStack)-1]
		e.sb.WriteString("\n")
	case "ol":
		e.sb.WriteString("\n")
		e.listStack = append(e.listStack, listContext{ordered: true, index: 0})
		e.walkChildren(n)
		e.listStack = e.listStack[:len(e.listStack)-1]
		e.sb.WriteString("\n")
	case "li":
		depth := len(e.listStack)
		indent := ""
		if depth > 1 {
			indent = strings.Repeat("  ", depth-1)
		}
		if depth > 0 {
			ctx := &e.listStack[depth-1]
			if ctx.ordered {
				ctx.index++
				fmt.Fprintf(&e.sb, "\n%s%d. ", indent, ctx.index)
			} else {
				fmt.Fprintf(&e.sb, "\n%s- ", indent)
			}
		} else {
			e.sb.WriteString("\n- ")
		}
		e.walkChildren(n)

	// --- Definition lists ---
	case "dt":
		e.sb.WriteString("\n**")
		e.walkChildren(n)
		e.sb.WriteString("**")
	case "dd":
		e.sb.WriteString("\n: ")
		e.walkChildren(n)

	// --- Tables ---
	case "table":
		e.sb.WriteString("\n\n")
		e.walkChildren(n)
		e.sb.WriteString("\n")
	case "thead", "tbody", "tfoot":
		e.walkChildren(n)
	case "tr":
		e.cellIndex = 0
		e.sb.WriteString("| ")
		e.walkChildren(n)
		e.sb.WriteString("|\n")
	case "td", "th":
		if e.cellIndex > 0 {
			e.sb.WriteString(" | ")
		}
		e.cellIndex++
		e.walkChildren(n)
		e.sb.WriteString(" ")

	default:
		e.walkChildren(n)
	}
}

func (e *htmlExtractor) walkChildren(n *html.Node) {
	for c := n.FirstChild; c != nil; c = c.NextSibling {
		e.walk(c)
	}
}

func (e *htmlExtractor) resolveHref(href string) string {
	if e.baseURL == nil {
		return href
	}
	parsed, err := url.Parse(href)
	if err != nil {
		return href
	}
	return e.baseURL.ResolveReference(parsed).String()
}

// getAttr returns the value of an attribute on an HTML element node.
func getAttr(n *html.Node, key string) string {
	for _, attr := range n.Attr {
		if attr.Key == key {
			return attr.Val
		}
	}
	return ""
}

// nodeText extracts plain text from a node tree, collapsing whitespace.
func nodeText(n *html.Node) string {
	var sb strings.Builder
	nodeTextWalk(&sb, n)
	return strings.TrimSpace(collapseWhitespace(sb.String()))
}

func nodeTextWalk(sb *strings.Builder, n *html.Node) {
	if n.Type == html.TextNode {
		sb.WriteString(n.Data)
		return
	}
	for c := n.FirstChild; c != nil; c = c.NextSibling {
		nodeTextWalk(sb, c)
	}
}

// collapseWhitespace replaces runs of whitespace with a single space.
func collapseWhitespace(s string) string {
	var sb strings.Builder
	inSpace := false
	for _, r := range s {
		if r == ' ' || r == '\t' || r == '\n' || r == '\r' || r == '\f' {
			if !inSpace {
				sb.WriteRune(' ')
				inSpace = true
			}
		} else {
			sb.WriteRune(r)
			inSpace = false
		}
	}
	return sb.String()
}

// cleanupText normalizes whitespace in the final output: trims lines,
// collapses runs of blank lines to at most two, trims overall.
func cleanupText(text string) string {
	lines := strings.Split(text, "\n")
	var cleaned []string
	blankRun := 0
	for _, line := range lines {
		line = strings.TrimRight(line, " \t")
		if line == "" {
			blankRun++
			if blankRun <= 2 {
				cleaned = append(cleaned, "")
			}
		} else {
			blankRun = 0
			cleaned = append(cleaned, line)
		}
	}
	return strings.TrimSpace(strings.Join(cleaned, "\n"))
}

// basicStripTags is a fallback HTML tag remover used when parsing fails.
func basicStripTags(s string) string {
	var sb strings.Builder
	inTag := false
	for _, r := range s {
		if r == '<' {
			inTag = true
			continue
		}
		if r == '>' {
			inTag = false
			sb.WriteRune(' ')
			continue
		}
		if !inTag {
			sb.WriteRune(r)
		}
	}
	return sb.String()
}
