package crawler

import (
	"bytes"
	"net/url"
	"strings"

	"golang.org/x/net/html"
)

func extractLinks(pageURL string, body []byte) ([]string, error) {
	doc, err := html.Parse(bytes.NewReader(body))
	if err != nil {
		return nil, err
	}

	seen := make(map[string]struct{})
	var links []string

	var visit func(*html.Node)
	visit = func(n *html.Node) {
		if n.Type == html.ElementNode && n.Data == "a" {
			for _, attr := range n.Attr {
				if attr.Key != "href" {
					continue
				}
				abs, ok := normalizeLink(pageURL, attr.Val)
				if !ok {
					continue
				}
				if _, exists := seen[abs]; exists {
					continue
				}
				seen[abs] = struct{}{}
				links = append(links, abs)
			}
		}
		for child := n.FirstChild; child != nil; child = child.NextSibling {
			visit(child)
		}
	}
	visit(doc)

	return links, nil
}

func normalizeLink(baseURL, href string) (string, bool) {
	href = strings.TrimSpace(href)
	if href == "" || strings.HasPrefix(href, "#") {
		return "", false
	}

	lower := strings.ToLower(href)
	for _, prefix := range []string{"mailto:", "javascript:", "tel:", "data:"} {
		if strings.HasPrefix(lower, prefix) {
			return "", false
		}
	}

	base, err := url.Parse(baseURL)
	if err != nil {
		return "", false
	}

	ref, err := url.Parse(href)
	if err != nil {
		return "", false
	}

	abs := base.ResolveReference(ref)
	if abs.Scheme != "http" && abs.Scheme != "https" {
		return "", false
	}

	abs.Fragment = ""
	return abs.String(), true
}
