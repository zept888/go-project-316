package crawler

import (
	"bytes"
	"html"
	"strings"

	ghtml "golang.org/x/net/html"
)

type SEOReport struct {
	HasTitle       bool   `json:"has_title"`
	Title          string `json:"title"`
	HasDescription bool   `json:"has_description"`
	Description    string `json:"description"`
	HasH1          bool   `json:"has_h1"`
}

func extractSEO(body []byte) SEOReport {
	doc, err := ghtml.Parse(bytes.NewReader(body))
	if err != nil {
		return SEOReport{}
	}

	seo := SEOReport{}

	var visit func(*ghtml.Node)
	visit = func(n *ghtml.Node) {
		if n.Type == ghtml.ElementNode {
			switch n.Data {
			case "title":
				if !seo.HasTitle {
					seo.HasTitle = true
					seo.Title = cleanText(nodeText(n))
				}
			case "meta":
				if !seo.HasDescription && isDescriptionMeta(n) {
					seo.HasDescription = true
					seo.Description = cleanText(metaContent(n))
				}
			case "h1":
				if !seo.HasH1 {
					seo.HasH1 = true
				}
			}
		}
		for child := n.FirstChild; child != nil; child = child.NextSibling {
			visit(child)
		}
	}
	visit(doc)

	return seo
}

func isDescriptionMeta(n *ghtml.Node) bool {
	for _, attr := range n.Attr {
		if strings.EqualFold(attr.Key, "name") && strings.EqualFold(attr.Val, "description") {
			return true
		}
	}
	return false
}

func metaContent(n *ghtml.Node) string {
	for _, attr := range n.Attr {
		if strings.EqualFold(attr.Key, "content") {
			return attr.Val
		}
	}
	return ""
}

func nodeText(n *ghtml.Node) string {
	var buf strings.Builder
	var collect func(*ghtml.Node)
	collect = func(n *ghtml.Node) {
		if n.Type == ghtml.TextNode {
			buf.WriteString(n.Data)
		}
		for child := n.FirstChild; child != nil; child = child.NextSibling {
			collect(child)
		}
	}
	collect(n)
	return buf.String()
}

func cleanText(s string) string {
	s = html.UnescapeString(s)
	s = strings.Join(strings.Fields(s), " ")
	return strings.TrimSpace(s)
}
