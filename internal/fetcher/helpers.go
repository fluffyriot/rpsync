package fetcher

import (
	"strings"

	"golang.org/x/net/html"
)

func stripHTMLToText(input string) string {
	doc, err := html.Parse(strings.NewReader(input))
	if err != nil {
		return ""
	}

	var b strings.Builder

	var walk func(*html.Node)
	walk = func(n *html.Node) {
		if n.Type == html.TextNode {
			text := strings.TrimSpace(n.Data)
			if text != "" {
				if b.Len() > 0 {
					b.WriteString(" ")
				}
				b.WriteString(text)
			}
		}

		for c := n.FirstChild; c != nil; c = c.NextSibling {
			walk(c)
		}
	}

	walk(doc)

	return strings.Join(strings.Fields(html.UnescapeString(b.String())), " ")
}
