// Copyright 2026 Crow CI Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0

// Package theme loads admin-provided UI themes from files or URLs.
package theme

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path"
	"regexp"
	"strings"
	"time"

	"github.com/rs/zerolog/log"
)

// Theme is a UI theme provided by the instance admin.
// Light and Dark map shadcn/ui CSS variable names (without the leading `--`) to
// CSS color values; variables that are not set are taken from the Base theme.
type Theme struct {
	ID          string            `json:"id"`
	Name        string            `json:"name,omitempty"`
	Description string            `json:"description,omitempty"`
	Base        string            `json:"base,omitempty"`
	Light       map[string]string `json:"light,omitempty"`
	Dark        map[string]string `json:"dark,omitempty"`
}

// jsonTheme is the JSON format of a theme. Besides `light` and `dark` it
// accepts the `cssVars` object of a shadcn/tweakcn registry item, whose `name`
// is the id and `title` the display name, so generator output can be used as is.
type jsonTheme struct {
	Theme
	Title   string `json:"title"`
	CSSVars struct {
		Light map[string]string `json:"light"`
		Dark  map[string]string `json:"dark"`
	} `json:"cssVars"`
}

const (
	// Values end up in style.setProperty, so anything that could close the
	// declaration or the surrounding block is rejected outright.
	invalidValueChars = ";{}<>\\"

	maxSourceSize = 1 << 20
	fetchTimeout  = 10 * time.Second
)

var (
	idPattern       = regexp.MustCompile(`^[a-z0-9][a-z0-9_-]*$`)
	idInvalidChars  = regexp.MustCompile(`[^a-z0-9_-]+`)
	cssCommentRE    = regexp.MustCompile(`(?s)/\*.*?\*/`)
	cssRuleRE       = regexp.MustCompile(`([^{}]*)\{([^{}]*)\}`)
	cssImportantRE  = regexp.MustCompile(`(?i)\s*!important\s*$`)
	cssSelectorTrim = strings.NewReplacer(" ", "", "\t", "", "\n", "", "\r", "", `"`, "", "'", "")

	lightSelectors = map[string]bool{
		":root": true, "html": true, ".light": true, ":root.light": true, "html.light": true,
		"[data-theme=light]": true, ":root[data-theme=light]": true,
	}
	darkSelectors = map[string]bool{
		".dark": true, ":root.dark": true, "html.dark": true,
		"[data-theme=dark]": true, ":root[data-theme=dark]": true,
	}

	// The variables a theme can set; must match CSS_VAR_MAP in web/src/lib/themes/css-vars.ts.
	themeVars = map[string]bool{
		"background": true, "foreground": true,
		"card": true, "card-foreground": true,
		"popover": true, "popover-foreground": true,
		"primary": true, "primary-foreground": true,
		"secondary": true, "secondary-foreground": true,
		"muted": true, "muted-foreground": true,
		"accent": true, "accent-foreground": true,
		"destructive": true, "destructive-foreground": true,
		"border": true, "input": true, "ring": true,
		"chart-1": true, "chart-2": true, "chart-3": true, "chart-4": true, "chart-5": true,
		"sidebar": true, "sidebar-foreground": true,
		"sidebar-primary": true, "sidebar-primary-foreground": true,
		"sidebar-accent": true, "sidebar-accent-foreground": true,
		"sidebar-border": true, "sidebar-ring": true,
	}
)

// errFetch marks network failures, which Load skips instead of failing on.
var errFetch = errors.New("fetch custom themes")

// Load reads themes from each source, a file path or an http(s) URL.
// Sources that cannot be fetched over the network are logged and skipped, so
// an unreachable host does not keep the server from starting; unreadable
// files and invalid content are errors.
func Load(ctx context.Context, sources []string) ([]Theme, error) {
	var themes []Theme
	seen := make(map[string]string)

	for _, source := range sources {
		source = strings.TrimSpace(source)
		if source == "" {
			continue
		}

		data, name, err := read(ctx, source)
		if errors.Is(err, errFetch) {
			log.Error().Err(err).Str("source", source).Msg("skipping custom themes that could not be fetched")
			continue
		}
		if err != nil {
			return nil, err
		}

		parsed, err := Parse(data, name)
		if err != nil {
			return nil, fmt.Errorf("parse custom themes from %s: %w", source, err)
		}
		for _, t := range parsed {
			if other, ok := seen[t.ID]; ok {
				return nil, fmt.Errorf("theme %q from %s: id already defined by %s", t.ID, source, other)
			}
			seen[t.ID] = source
		}
		themes = append(themes, parsed...)
	}
	return themes, nil
}

// read returns the content of source and its file name.
func read(ctx context.Context, source string) ([]byte, string, error) {
	u, err := url.Parse(source)
	if err != nil || (u.Scheme != "http" && u.Scheme != "https") {
		data, err := os.ReadFile(source)
		if err != nil {
			return nil, "", fmt.Errorf("read custom themes file: %w", err)
		}
		if len(data) > maxSourceSize {
			return nil, "", fmt.Errorf("custom themes file %s is larger than %d bytes", source, maxSourceSize)
		}
		return data, path.Base(strings.ReplaceAll(source, "\\", "/")), nil
	}

	ctx, cancel := context.WithTimeout(ctx, fetchTimeout)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, source, nil)
	if err != nil {
		return nil, "", fmt.Errorf("%w: %w", errFetch, err)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, "", fmt.Errorf("%w: %w", errFetch, err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, "", fmt.Errorf("%w: unexpected status %s", errFetch, resp.Status)
	}

	data, err := io.ReadAll(io.LimitReader(resp.Body, maxSourceSize+1))
	if err != nil {
		return nil, "", fmt.Errorf("%w: %w", errFetch, err)
	}
	if len(data) > maxSourceSize {
		return nil, "", fmt.Errorf("custom themes at %s are larger than %d bytes", source, maxSourceSize)
	}
	return data, path.Base(u.Path), nil
}

// Parse decodes themes from data, detecting the format from its content:
//   - a JSON array of themes,
//   - a single JSON theme or shadcn/tweakcn registry item,
//   - CSS with a `:root` block for light and a `.dark` block for dark mode.
//
// name is the source's file name, used as id and display name for themes that
// do not define one (CSS never does).
func Parse(data []byte, name string) ([]Theme, error) {
	trimmed := bytes.TrimSpace(data)
	var entries []jsonTheme

	switch {
	case bytes.HasPrefix(trimmed, []byte("[")):
		if err := json.Unmarshal(trimmed, &entries); err != nil {
			return nil, fmt.Errorf("invalid JSON array of themes: %w", err)
		}
	case bytes.HasPrefix(trimmed, []byte("{")) && json.Valid(trimmed):
		var entry jsonTheme
		if err := json.Unmarshal(trimmed, &entry); err != nil {
			return nil, fmt.Errorf("invalid JSON theme: %w", err)
		}
		entries = []jsonTheme{entry}
	default:
		entry, err := parseCSS(string(trimmed))
		if err != nil {
			return nil, err
		}
		entries = []jsonTheme{entry}
	}

	single := len(entries) == 1
	themes := make([]Theme, 0, len(entries))
	seen := make(map[string]bool, len(entries))
	for i, entry := range entries {
		t, err := entry.normalize(single, name)
		if err != nil {
			return nil, fmt.Errorf("theme %d: %w", i, err)
		}
		if seen[t.ID] {
			return nil, fmt.Errorf("theme %q: duplicate id", t.ID)
		}
		seen[t.ID] = true
		themes = append(themes, t)
	}
	return themes, nil
}

func (entry jsonTheme) normalize(single bool, fileName string) (Theme, error) {
	t := entry.Theme
	t.ID = strings.TrimSpace(t.ID)
	t.Name = strings.TrimSpace(t.Name)

	// Registry items name the theme with `name` (a slug used as id) and an
	// optional `title` (display name).
	if t.ID == "" && idPattern.MatchString(t.Name) {
		t.ID, t.Name = t.Name, strings.TrimSpace(entry.Title)
		if t.Name == "" {
			t.Name = nameFromID(t.ID)
		}
	}
	// A source holding a single theme may leave naming to its file name.
	if t.ID == "" && single {
		t.ID = idFromFileName(fileName)
		if t.Name == "" {
			t.Name = nameFromID(t.ID)
		}
	}
	if !idPattern.MatchString(t.ID) {
		return Theme{}, fmt.Errorf("id %q must match %s", t.ID, idPattern)
	}

	if t.Light == nil {
		t.Light = entry.CSSVars.Light
	}
	if t.Dark == nil {
		t.Dark = entry.CSSVars.Dark
	}

	var err error
	if t.Light, err = normalizeVars(t.Light); err != nil {
		return Theme{}, fmt.Errorf("theme %q: light: %w", t.ID, err)
	}
	if t.Dark, err = normalizeVars(t.Dark); err != nil {
		return Theme{}, fmt.Errorf("theme %q: dark: %w", t.ID, err)
	}
	if len(t.Light) == 0 && len(t.Dark) == 0 {
		return Theme{}, fmt.Errorf("theme %q: defines no theme colors", t.ID)
	}
	return t, nil
}

// normalizeVars strips the leading `--` and drops variables that are not
// theme colors, such as the fonts, radius and shadows generators emit.
func normalizeVars(vars map[string]string) (map[string]string, error) {
	out := make(map[string]string, len(vars))
	for name, value := range vars {
		name = strings.TrimPrefix(strings.TrimSpace(name), "--")
		if !themeVars[name] {
			continue
		}
		value = strings.TrimSpace(value)
		if value == "" {
			return nil, fmt.Errorf("variable %q: empty value", name)
		}
		if strings.ContainsAny(value, invalidValueChars) {
			return nil, fmt.Errorf("variable %q: value %q contains one of %q", name, value, invalidValueChars)
		}
		out[name] = value
	}
	if len(out) == 0 {
		return nil, nil
	}
	return out, nil
}

// parseCSS reads custom properties from light (`:root`) and dark (`.dark`)
// blocks; other blocks, such as tweakcn's `@theme inline`, are ignored.
func parseCSS(css string) (jsonTheme, error) {
	var entry jsonTheme
	entry.Light = map[string]string{}
	entry.Dark = map[string]string{}

	css = cssCommentRE.ReplaceAllString(css, "")
	rules := cssRuleRE.FindAllStringSubmatch(css, -1)
	if len(rules) == 0 {
		return entry, errors.New("neither JSON nor CSS with :root or .dark blocks")
	}

	for _, rule := range rules {
		var target map[string]string
		switch selector := cssSelectorTrim.Replace(rule[1]); {
		case allIn(selector, darkSelectors):
			target = entry.Dark
		case allIn(selector, lightSelectors):
			target = entry.Light
		default:
			continue
		}

		for _, decl := range strings.Split(rule[2], ";") {
			name, value, ok := strings.Cut(decl, ":")
			name = strings.TrimSpace(name)
			if !ok || !strings.HasPrefix(name, "--") {
				continue
			}
			target[name] = cssImportantRE.ReplaceAllString(strings.TrimSpace(value), "")
		}
	}
	return entry, nil
}

// allIn reports whether every selector of a selector list is in set.
func allIn(selectorList string, set map[string]bool) bool {
	for _, selector := range strings.Split(selectorList, ",") {
		if !set[selector] {
			return false
		}
	}
	return true
}

func idFromFileName(name string) string {
	name = strings.ToLower(strings.TrimSuffix(name, path.Ext(name)))
	name = strings.TrimPrefix(name, "theme-")
	return strings.Trim(idInvalidChars.ReplaceAllString(name, "-"), "-_")
}

func nameFromID(id string) string {
	words := strings.FieldsFunc(id, func(r rune) bool { return r == '-' || r == '_' })
	for i, word := range words {
		words[i] = strings.ToUpper(word[:1]) + word[1:]
	}
	return strings.Join(words, " ")
}
