// Package shortcut generates a ready-to-import Apple Shortcut for Snagarr, so
// an operator can send a household member a link instead of dictating the steps
// to build one by hand.
//
// A .shortcut file is a property list. Apple writes binary plists, but
// WorkflowKit decodes with NSPropertyListSerialization, which sniffs the format,
// so the XML plist written here imports the same way and needs no dependency.
package shortcut

import (
	"bytes"
	"encoding/xml"
	"errors"
	"fmt"
	"net/url"
	"strings"
)

// Options describes the Shortcut to generate.
type Options struct {
	BaseURL string // e.g. https://snagarr.example.com — no trailing slash
	Token   string // the bearer token baked into the Shortcut
	Name    string // Shortcut name, default "Snag"
}

// DefaultName is used when Options.Name is empty.
const DefaultName = "Snag"

const (
	// Shortcuts joins one action's output to a later action by UUID rather than
	// by position, so the two actions that are referenced carry fixed ones.
	captureUUID = "3F1C6A2E-9D74-4B08-A5E1-7C0B92D4F6A3"
	titleUUID   = "8B54D0C1-2E6F-4A93-B7D5-1F80A3E5C29B"

	// Shortcuts stores a variable reference out of line: the text holds
	// U+FFFC OBJECT REPLACEMENT CHARACTER as a placeholder, and
	// attachmentsByRange maps that character's offset to the variable.
	objectReplacement = "￼"

	// Kept ASCII on purpose: attachment offsets are UTF-16 code units, which
	// only match len() while the prefix stays single-byte.
	notificationPrefix = "Snagged "
)

// Build returns an XML plist ready to serve as a .shortcut file.
func Build(opts Options) ([]byte, error) {
	base := strings.TrimRight(opts.BaseURL, "/")
	u, err := url.Parse(base)
	if err != nil {
		return nil, fmt.Errorf("parse base url %q: %w", opts.BaseURL, err)
	}
	if (u.Scheme != "http" && u.Scheme != "https") || u.Host == "" {
		return nil, fmt.Errorf("base url %q must be an absolute http or https url", opts.BaseURL)
	}
	if opts.Token == "" {
		return nil, errors.New("shortcut: token is required")
	}
	name := opts.Name
	if name == "" {
		name = DefaultName
	}

	capture := dict{
		{"WFWorkflowActionIdentifier", "is.workflow.actions.downloadurl"},
		{"WFWorkflowActionParameters", dict{
			{"UUID", captureUUID},
			{"Advanced", true},
			{"WFURL", base + "/api/v1/capture"},
			{"WFHTTPMethod", "POST"},
			{"WFHTTPBodyType", "JSON"},
			{"WFHTTPHeaders", fieldValue(
				field("Authorization", text("Bearer "+opts.Token)),
				field("Content-Type", text("application/json")),
			)},
			// Always "query", never "url": telling a URL from plain text needs a
			// conditional whose comparison operands are brittle to hand-write,
			// and the server resolves links passed as query anyway.
			{"WFJSONValues", fieldValue(
				field("query", attachment("", dict{{"Type", "ExtensionInput"}})),
				field("source", text("shortcut")),
			)},
		}},
	}

	title := dict{
		{"WFWorkflowActionIdentifier", "is.workflow.actions.getvalueforkey"},
		{"WFWorkflowActionParameters", dict{
			{"UUID", titleUUID},
			{"WFGetDictionaryValueType", "Value"},
			{"WFDictionaryKey", "title"},
		}},
	}

	notify := dict{
		{"WFWorkflowActionIdentifier", "is.workflow.actions.notification"},
		{"WFWorkflowActionParameters", dict{
			{"WFNotificationActionTitle", name},
			{"WFNotificationActionSound", false},
			{"WFNotificationActionBody", attachment(notificationPrefix, dict{
				{"Type", "ActionOutput"},
				{"OutputName", "Dictionary Value"},
				{"OutputUUID", titleUUID},
			})},
		}},
	}

	root := dict{
		{"WFWorkflowClientVersion", "1146.11"},
		// Deliberately below any shipping client: a value above the importing
		// app's version makes Shortcuts refuse the file as "format too new".
		{"WFWorkflowMinimumClientVersion", 900},
		{"WFWorkflowName", name},
		{"WFWorkflowIcon", dict{
			{"WFWorkflowIconStartColor", 463140863}, // blue
			{"WFWorkflowIconGlyphNumber", 59753},
		}},
		{"WFWorkflowImportQuestions", []any{}},
		{"WFWorkflowTypes", []any{"ActionExtension"}}, // "Show in Share Sheet"
		{"WFWorkflowInputContentItemClasses", []any{
			"WFArticleContentItem",
			"WFRichTextContentItem",
			"WFSafariWebPageContentItem",
			"WFStringContentItem",
			"WFURLContentItem",
		}},
		{"WFWorkflowHasShortcutInputVariables", true},
		// Covers the run-directly case without a branch: with no share-sheet
		// input, Shortcuts itself prompts for text and feeds it to the input.
		{"WFWorkflowNoInputBehavior", dict{
			{"Name", "WFWorkflowNoInputBehaviorAskForInput"},
			{"Parameters", dict{{"ItemClass", "WFStringContentItem"}}},
		}},
		{"WFWorkflowActions", []any{capture, title, notify}},
	}

	var b bytes.Buffer
	b.WriteString(xml.Header)
	b.WriteString("<!DOCTYPE plist PUBLIC \"-//Apple//DTD PLIST 1.0//EN\" \"http://www.apple.com/DTDs/PropertyList-1.0.dtd\">\n")
	b.WriteString("<plist version=\"1.0\">\n")
	write(&b, root)
	b.WriteString("\n</plist>\n")
	return b.Bytes(), nil
}

// entry is one key/value pair of a plist dictionary. A plist dictionary is
// written as a flat run of <key> and value elements, so the generator keeps an
// ordered slice rather than a Go map, which would also reorder between builds.
type entry struct {
	key string
	val any
}

type dict []entry

// text is a Shortcuts string with no variable spliced into it.
func text(s string) dict {
	return dict{
		{"Value", dict{{"attachmentsByRange", dict{}}, {"string", s}}},
		{"WFSerializationType", "WFTextTokenString"},
	}
}

// attachment is a Shortcuts string ending in a variable reference described by
// att, such as the shortcut's own input or an earlier action's output.
func attachment(prefix string, att dict) dict {
	return dict{
		{"Value", dict{
			{"attachmentsByRange", dict{{fmt.Sprintf("{%d, 1}", len(prefix)), att}}},
			{"string", prefix + objectReplacement},
		}},
		{"WFSerializationType", "WFTextTokenString"},
	}
}

// field is one row of a Shortcuts dictionary parameter. WFItemType 0 marks the
// value as text.
func field(key string, value dict) dict {
	return dict{{"WFItemType", 0}, {"WFKey", text(key)}, {"WFValue", value}}
}

// fieldValue wraps rows as the dictionary parameter that WFHTTPHeaders and
// WFJSONValues expect.
func fieldValue(fields ...dict) dict {
	items := make([]any, len(fields))
	for i, f := range fields {
		items[i] = f
	}
	return dict{
		{"Value", dict{{"WFDictionaryFieldValueItems", items}}},
		{"WFSerializationType", "WFDictionaryFieldValue"},
	}
}

func write(b *bytes.Buffer, v any) {
	switch v := v.(type) {
	case string:
		b.WriteString("<string>")
		_ = xml.EscapeText(b, []byte(v))
		b.WriteString("</string>")
	case int:
		fmt.Fprintf(b, "<integer>%d</integer>", v)
	case bool:
		if v {
			b.WriteString("<true/>")
		} else {
			b.WriteString("<false/>")
		}
	case dict:
		b.WriteString("<dict>")
		for _, e := range v {
			b.WriteString("<key>")
			_ = xml.EscapeText(b, []byte(e.key))
			b.WriteString("</key>")
			write(b, e.val)
		}
		b.WriteString("</dict>")
	case []any:
		b.WriteString("<array>")
		for _, item := range v {
			write(b, item)
		}
		b.WriteString("</array>")
	default:
		panic(fmt.Sprintf("shortcut: cannot write %T into a plist", v))
	}
}
