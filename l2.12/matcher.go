package main

import (
	"regexp"
	"strings"
)

// buildMatcher подготавливает функцию проверки строки на соответствие шаблону
// с учётом флагов -i (игнор регистра) и -F (фиксированная подстрока).
// Для -F используется strings.Contains, иначе — регулярное выражение.
func buildMatcher(cfg *Config) (func(string) bool, error) {
	pat := cfg.pattern

	if cfg.fixed {
		if cfg.ignoreCase {
			pat = strings.ToLower(pat)
			return func(s string) bool {
				return strings.Contains(strings.ToLower(s), pat)
			}, nil
		}
		return func(s string) bool {
			return strings.Contains(s, pat)
		}, nil
	}

	if cfg.ignoreCase {
		// (?i) — inline-флаг игнорирования регистра для regexp.
		pat = "(?i)" + pat
	}

	re, err := regexp.Compile(pat)
	if err != nil {
		return nil, err
	}
	return re.MatchString, nil
}
