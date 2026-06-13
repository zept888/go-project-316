package crawler

import (
	"net/url"
	"strings"
)

func canonicalURL(raw string) (string, error) {
	u, err := url.Parse(raw)
	if err != nil {
		return "", err
	}

	u.Fragment = ""
	if u.Path == "/" {
		u.Path = ""
	} else if strings.HasSuffix(u.Path, "/") && len(u.Path) > 1 {
		u.Path = strings.TrimSuffix(u.Path, "/")
	}
	u.RawPath = ""

	return u.String(), nil
}

func sameDomain(baseURL, targetURL string) bool {
	base, err := url.Parse(baseURL)
	if err != nil {
		return false
	}
	target, err := url.Parse(targetURL)
	if err != nil {
		return false
	}
	return strings.EqualFold(base.Host, target.Host)
}
