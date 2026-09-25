package web

import (
	"io/fs"
	"strings"
	"testing"
)

func TestEmbeddedAppLinksCanonicalSource(t *testing.T) {
	app, err := fs.ReadFile(static, "static/app.js")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(app), "https://github.com/Bahonio/awg-docui") {
		t.Fatal("embedded Web UI does not link to the canonical source repository")
	}
}

func TestEmbeddedAppOffersPersistentEndpointEditing(t *testing.T) {
	app, err := fs.ReadFile(static, "static/app.js")
	if err != nil {
		t.Fatal(err)
	}
	source := string(app)
	for _, want := range []string{`data-action="edit-endpoint"`, `data-form="endpoint"`, `/endpoint`, `Existing client installations are not updated automatically`} {
		if !strings.Contains(source, want) {
			t.Errorf("embedded Web UI is missing endpoint editor marker %q", want)
		}
	}
}
