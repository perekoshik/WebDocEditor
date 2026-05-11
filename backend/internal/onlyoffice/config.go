package onlyoffice

import (
	"fmt"
	"strings"
)

type ConfigBuilder struct {
	InternalAPIURL string
	PublicAPIURL   string
}

type UserInfo struct {
	ID   string
	Name string
}

func (b *ConfigBuilder) Build(docID string, version int, filename, title string, user UserInfo) map[string]any {
	ext := strings.TrimPrefix(strings.ToLower(extFrom(filename)), ".")
	return map[string]any{
		"document": map[string]any{
			"fileType": ext,
			"key":      fmt.Sprintf("%s_%d", docID, version),
			"title":    filename,
			"url":      fmt.Sprintf("%s/api/documents/%s/file", b.InternalAPIURL, docID),
			"permissions": map[string]any{
				"edit":     true,
				"comment":  true,
				"review":   true,
				"download": true,
				"print":    true,
				"copy":     true,
				"protect":  false,
				"chat":     false,
			},
		},
		"documentType": documentType(ext),
		"editorConfig": map[string]any{
			"callbackUrl": fmt.Sprintf("%s/api/documents/%s/callback", b.InternalAPIURL, docID),
			"lang":        "ru",
			"user": map[string]any{
				"id":   user.ID,
				"name": user.Name,
			},
			"customization": map[string]any{
				"goback": map[string]any{
					"url":  b.PublicAPIURL + "/",
					"text": "К списку документов",
				},
				"customer": map[string]any{
					"name": "LegalEdit",
				},
				"feedback":            false,
				"about":               false,
				"help":                false,
				"chat":                false,
				"plugins":             false,
				"macros":              false,
				"macrosMode":          "disable",
				"compactToolbar":      true,
				"hideRulers":          true,
				"toolbarHideFileName": true,
				"uiTheme":             "theme-light",
			},
		},
		"width":  "100%",
		"height": "100%",
		"type":   "desktop",
	}
}

func documentType(ext string) string {
	switch ext {
	case "docx", "doc", "odt", "rtf", "txt":
		return "word"
	case "xlsx", "xls", "ods", "csv":
		return "cell"
	case "pptx", "ppt", "odp":
		return "slide"
	default:
		return "word"
	}
}

func extFrom(filename string) string {
	i := strings.LastIndex(filename, ".")
	if i < 0 {
		return ""
	}
	return filename[i:]
}
