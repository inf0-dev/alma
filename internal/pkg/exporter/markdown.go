package exporter

import (
	"bytes"
	"embed"
	"fmt"
	"strings"
	"text/template"

	v1 "github.com/inf0-dev/alma/api/v1"
)

//go:embed templates/record.md.tmpl
var templateFS embed.FS

var funcMap = template.FuncMap{
	"joinStrings":     strings.Join,
	"yesNo":           yesNo,
	"requirementDesc": requirementDesc,
	"itemDesc":        itemDesc,
	"answerDesc":      answerDesc,
	"optionTitle":     optionTitle,
	"optionDesc":      optionDesc,
	"optionPros":      optionPros,
	"optionCons":      optionCons,
	"joinFailedReqs":  joinFailedReqs,
	"itemValue":       itemValue,
	"anchor":          anchor,
	"statusBadge":     statusBadge,
	"deref":           func(s *string) string { return *s },
	"string":          func(k v1.Kind) string { return string(k) },
	"isVisible":       isVisible,
}

var mdTemplate = template.Must(
	template.New("record.md.tmpl").Funcs(funcMap).ParseFS(templateFS, "templates/record.md.tmpl"),
)

// Markdown renders a Record as a markdown string.
func Markdown(r *v1.Record) (string, error) {
	var buf bytes.Buffer
	if err := mdTemplate.Execute(&buf, r); err != nil {
		return "", fmt.Errorf("failed to render markdown: %w", err)
	}
	return buf.String(), nil
}

func requirementDesc(doc v1.Document, id string) string {
	for _, req := range doc.Schema.Requirements {
		if req.ID == id {
			return req.Description
		}
	}
	return id
}

func itemDesc(doc v1.Document, id string) string {
	for _, item := range doc.Schema.Items {
		if item.ID == id {
			return item.Description
		}
	}
	return id
}

func answerDesc(doc v1.Document, itemID, answerID string) string {
	for _, item := range doc.Schema.Items {
		if item.ID == itemID {
			for _, co := range item.ChoiceOptions {
				if co.ID == answerID {
					return co.Description
				}
			}
		}
	}
	return answerID
}

func optionTitle(doc v1.Document, id string) string {
	for _, opt := range doc.Schema.DesignOptions {
		if opt.ID == id {
			return opt.Title
		}
	}
	return id
}

func itemValue(doc v1.Document, item v1.RecordItem) string {
	switch item.Kind {
	case v1.KindChoice:
		if item.Answer == nil {
			return "_(unanswered)_"
		}
		return answerDesc(doc, item.ID, *item.Answer)
	case v1.KindNumber:
		if item.Value == nil {
			return "_(empty)_"
		}
		if item.Unit != "" {
			return fmt.Sprintf("%v %s", item.Value, item.Unit)
		}
		return fmt.Sprintf("%v", item.Value)
	default:
		if item.Value == nil {
			return "_(empty)_"
		}
		return fmt.Sprintf("%v", item.Value)
	}
}

func anchor(title string) string {
	s := strings.ToLower(title)
	s = strings.ReplaceAll(s, " ", "-")
	var b strings.Builder
	for _, r := range s {
		if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') || r == '-' {
			b.WriteRune(r)
		}
	}
	return b.String()
}

func optionDesc(doc v1.Document, id string) string {
	for _, opt := range doc.Schema.DesignOptions {
		if opt.ID == id {
			return opt.Description
		}
	}
	return ""
}

func optionPros(doc v1.Document, id string) []string {
	for _, opt := range doc.Schema.DesignOptions {
		if opt.ID == id {
			return opt.Pros
		}
	}
	return nil
}

func optionCons(doc v1.Document, id string) []string {
	for _, opt := range doc.Schema.DesignOptions {
		if opt.ID == id {
			return opt.Cons
		}
	}
	return nil
}

func joinFailedReqs(doc v1.Document, ids []string) string {
	descs := make([]string, len(ids))
	for i, id := range ids {
		descs[i] = requirementDesc(doc, id)
	}
	return strings.Join(descs, ", ")
}

func yesNo(v bool) string {
	if v {
		return "Yes"
	}
	return "No"
}

func isVisible(item v1.RecordItem) bool {
	return item.Visible == nil || *item.Visible
}

func statusBadge(s v1.OptionStatus) string {
	switch s {
	case v1.StatusPicked:
		return "✅ **Picked**"
	case v1.StatusPossible:
		return "🟢 Possible"
	case v1.StatusEliminated:
		return "❌ Eliminated"
	case v1.StatusBlocked:
		return "⚠️ Blocked"
	default:
		return string(s)
	}
}
