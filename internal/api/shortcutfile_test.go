package api

import (
	"encoding/xml"
	"strings"
	"testing"
)

// node mirrors any plist element, so a built Shortcut can be walked back
// without a plist decoder.
type node struct {
	XMLName  xml.Name
	Chardata string `xml:",chardata"`
	Nodes    []node `xml:",any"`
}

// parse asserts the output is well-formed XML and returns the root dictionary.
func parse(t *testing.T, opts shortcutOptions) node {
	t.Helper()
	out, err := buildShortcutFile(opts)
	if err != nil {
		t.Fatalf("buildShortcutFile: %v", err)
	}
	var plist node
	if err := xml.Unmarshal(out, &plist); err != nil {
		t.Fatalf("buildShortcutFile output is not well-formed XML: %v", err)
	}
	if plist.XMLName.Local != "plist" {
		t.Fatalf("root element = %q, want plist", plist.XMLName.Local)
	}
	if len(plist.Nodes) != 1 || plist.Nodes[0].XMLName.Local != "dict" {
		t.Fatalf("plist must hold exactly one dict, got %d children", len(plist.Nodes))
	}
	return plist.Nodes[0]
}

// at walks a plist tree. A string step looks up a dictionary key; an int step
// indexes an array.
func at(t *testing.T, n node, steps ...any) node {
	t.Helper()
	for _, step := range steps {
		switch step := step.(type) {
		case string:
			if n.XMLName.Local != "dict" {
				t.Fatalf("cannot look up %q in <%s>", step, n.XMLName.Local)
			}
			// A dict is a flat run of <key> followed by its value element.
			found := false
			for i := 0; i+1 < len(n.Nodes); i += 2 {
				if n.Nodes[i].XMLName.Local == "key" && n.Nodes[i].Chardata == step {
					n, found = n.Nodes[i+1], true
					break
				}
			}
			if !found {
				t.Fatalf("key %q not found", step)
			}
		case int:
			if n.XMLName.Local != "array" {
				t.Fatalf("cannot index <%s>", n.XMLName.Local)
			}
			if step >= len(n.Nodes) {
				t.Fatalf("index %d out of range, array has %d elements", step, len(n.Nodes))
			}
			n = n.Nodes[step]
		}
	}
	return n
}

// headerValue reads the value of the nth row of a Shortcuts dictionary
// parameter such as WFHTTPHeaders or WFJSONValues.
func headerValue(t *testing.T, params node, param string, row int) string {
	t.Helper()
	n := at(t, params, param, "Value", "WFDictionaryFieldValueItems", row, "WFValue", "Value", "string")
	return n.Chardata
}

func TestBuildIsAWellFormedPlistWithTheExpectedActions(t *testing.T) {
	root := parse(t, shortcutOptions{BaseURL: "https://snagarr.example.com", Token: "abc123"})

	for _, key := range []string{
		"WFWorkflowClientVersion", "WFWorkflowMinimumClientVersion", "WFWorkflowName",
		"WFWorkflowIcon", "WFWorkflowTypes", "WFWorkflowInputContentItemClasses",
		"WFWorkflowNoInputBehavior", "WFWorkflowActions",
	} {
		at(t, root, key) // fails the test if absent
	}

	if got := at(t, root, "WFWorkflowTypes", 0).Chardata; got != "ActionExtension" {
		t.Errorf("WFWorkflowTypes[0] = %q, want ActionExtension (Show in Share Sheet)", got)
	}

	actions := at(t, root, "WFWorkflowActions")
	want := []string{
		"is.workflow.actions.downloadurl",
		"is.workflow.actions.getvalueforkey",
		"is.workflow.actions.notification",
	}
	if len(actions.Nodes) != len(want) {
		t.Fatalf("got %d actions, want %d", len(actions.Nodes), len(want))
	}
	for i, id := range want {
		if got := at(t, actions, i, "WFWorkflowActionIdentifier").Chardata; got != id {
			t.Errorf("action %d identifier = %q, want %q", i, got, id)
		}
	}
}

func TestBuildPutsBaseURLAndTokenInTheRequest(t *testing.T) {
	root := parse(t, shortcutOptions{BaseURL: "https://snagarr.example.com", Token: "abc123"})
	params := at(t, root, "WFWorkflowActions", 0, "WFWorkflowActionParameters")

	if got := at(t, params, "WFURL").Chardata; got != "https://snagarr.example.com/api/v1/capture" {
		t.Errorf("WFURL = %q", got)
	}
	if got := at(t, params, "WFHTTPMethod").Chardata; got != "POST" {
		t.Errorf("WFHTTPMethod = %q, want POST", got)
	}
	if got := at(t, params, "WFHTTPBodyType").Chardata; got != "JSON" {
		t.Errorf("WFHTTPBodyType = %q, want JSON", got)
	}
	if got := headerValue(t, params, "WFHTTPHeaders", 0); got != "Bearer abc123" {
		t.Errorf("Authorization header = %q, want Bearer abc123", got)
	}
	if got := headerValue(t, params, "WFHTTPHeaders", 1); got != "application/json" {
		t.Errorf("Content-Type header = %q", got)
	}
	if got := headerValue(t, params, "WFJSONValues", 1); got != "shortcut" {
		t.Errorf("source field = %q, want shortcut", got)
	}

	// The query field carries the shortcut's own input, held out of line as an
	// attachment against the placeholder character.
	query := at(t, params, "WFJSONValues", "Value", "WFDictionaryFieldValueItems", 0)
	if got := at(t, query, "WFKey", "Value", "string").Chardata; got != "query" {
		t.Errorf("first JSON field = %q, want query", got)
	}
	if got := at(t, query, "WFValue", "Value", "string").Chardata; got != objectReplacement {
		t.Errorf("query value = %q, want the placeholder character", got)
	}
	ranges := at(t, query, "WFValue", "Value", "attachmentsByRange")
	if got := at(t, ranges, "{0, 1}", "Type").Chardata; got != "ExtensionInput" {
		t.Errorf("query attachment Type = %q, want ExtensionInput", got)
	}
}

func TestBuildEscapesMarkupInTheToken(t *testing.T) {
	const nasty = `a&b<c>d"e'f`
	out, err := buildShortcutFile(shortcutOptions{BaseURL: "https://snagarr.example.com/?a=1&b=2", Token: nasty})
	if err != nil {
		t.Fatalf("buildShortcutFile: %v", err)
	}
	// The raw ampersand and angle brackets must not reach the document.
	if strings.Contains(string(out), nasty) {
		t.Error("token was written unescaped")
	}

	root := parse(t, shortcutOptions{BaseURL: "https://snagarr.example.com/?a=1&b=2", Token: nasty})
	params := at(t, root, "WFWorkflowActions", 0, "WFWorkflowActionParameters")
	if got := headerValue(t, params, "WFHTTPHeaders", 0); got != "Bearer "+nasty {
		t.Errorf("token did not survive escaping: got %q, want %q", got, "Bearer "+nasty)
	}
	if got := at(t, params, "WFURL").Chardata; got != "https://snagarr.example.com/?a=1&b=2/api/v1/capture" {
		t.Errorf("url did not survive escaping: got %q", got)
	}
}

func TestBuildDefaultsNameAndTrimsTrailingSlash(t *testing.T) {
	root := parse(t, shortcutOptions{BaseURL: "https://snagarr.example.com///", Token: "abc123"})

	if got := at(t, root, "WFWorkflowName").Chardata; got != defaultShortcutName {
		t.Errorf("WFWorkflowName = %q, want %q", got, defaultShortcutName)
	}
	if got := at(t, root, "WFWorkflowActions", 0, "WFWorkflowActionParameters", "WFURL").Chardata; got != "https://snagarr.example.com/api/v1/capture" {
		t.Errorf("WFURL = %q, want the trailing slashes trimmed", got)
	}

	named := parse(t, shortcutOptions{BaseURL: "https://snagarr.example.com", Token: "abc123", Name: "Snag it"})
	if got := at(t, named, "WFWorkflowName").Chardata; got != "Snag it" {
		t.Errorf("WFWorkflowName = %q, want %q", got, "Snag it")
	}
}

func TestBuildRejectsUnusableOptions(t *testing.T) {
	for name, opts := range map[string]shortcutOptions{
		"no base url":  {Token: "abc123"},
		"relative url": {BaseURL: "/api", Token: "abc123"},
		"wrong scheme": {BaseURL: "ftp://snagarr.example.com", Token: "abc123"},
		"no token":     {BaseURL: "https://snagarr.example.com"},
	} {
		t.Run(name, func(t *testing.T) {
			if _, err := buildShortcutFile(opts); err == nil {
				t.Error("buildShortcutFile succeeded, want an error")
			}
		})
	}
}
