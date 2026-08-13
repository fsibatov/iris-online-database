package main

import (
	"errors"
	"net"
	"net/url"
	"strings"
)

var allowedExternalHosts = map[string]struct{}{
	"aminoapps.com":      {},
	"coub.com":           {},
	"discord.com":        {},
	"docs.google.com":    {},
	"github.com":         {},
	"irisonline.ru":      {},
	"t.me":               {},
	"vk.com":             {},
	"vk.ru":              {},
	"wiki.irisonline.ru": {},
	"www.vk.com":         {},
	"www.vk.ru":          {},
}

func validateExternalURL(raw string) (string, error) {
	if raw != strings.TrimSpace(raw) || strings.ContainsAny(raw, "\r\n\t") {
		return "", errors.New("некорректная внешняя ссылка")
	}
	parsed, err := url.Parse(raw)
	if err != nil || !parsed.IsAbs() || parsed.Scheme != "https" || parsed.User != nil || parsed.Opaque != "" {
		return "", errors.New("разрешены только HTTPS-ссылки")
	}
	hostname := strings.ToLower(parsed.Hostname())
	if hostname == "" || net.ParseIP(hostname) != nil {
		return "", errors.New("некорректный адрес внешней ссылки")
	}
	if port := parsed.Port(); port != "" && port != "443" {
		return "", errors.New("нестандартный порт внешней ссылки запрещён")
	}
	if _, allowed := allowedExternalHosts[hostname]; !allowed {
		return "", errors.New("внешний адрес не входит в список разрешённых")
	}
	return parsed.String(), nil
}
