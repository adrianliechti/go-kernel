package text_test

import (
	"context"
	"errors"
	"testing"

	"github.com/adrianliechti/go-kernel/pkg/extract"
	"github.com/adrianliechti/go-kernel/pkg/text"
)

func TestExtractorPassesTextThroughUnchanged(t *testing.T) {
	tests := []struct {
		input     extract.Input
		format    extract.Format
		mediaType string
	}{
		{extract.Input{Name: "notes.txt", Data: []byte("one\r\ntwo\n")}, extract.FormatText, "text/plain; charset=utf-8"},
		{extract.Input{Name: "README.md", Data: []byte("# Heading\n\nBody\n")}, extract.FormatMarkdown, "text/markdown; charset=utf-8"},
		{extract.Input{Name: "README.md", MediaType: "text/plain", Data: []byte("Markdown by extension")}, extract.FormatMarkdown, "text/markdown; charset=utf-8"},
		{extract.Input{Name: "upload", MediaType: "text/x-markdown; charset=utf-8", Data: []byte("**bold**")}, extract.FormatMarkdown, "text/markdown; charset=utf-8"},
	}
	extractor := text.NewExtractor()
	for _, test := range tests {
		doc, err := extractor.Extract(context.Background(), test.input)
		if err != nil {
			t.Fatal(err)
		}
		if doc.Format != test.format || doc.MediaType != test.mediaType || doc.Markdown != string(test.input.Data) {
			t.Fatalf("Document = %#v", doc)
		}
	}
}

func TestExtractorDoesNotClaimBinaryOrUnhintedFiles(t *testing.T) {
	extractor := text.NewExtractor()
	for _, input := range []extract.Input{
		{Name: "data.bin", Data: []byte("looks textual")},
		{Name: "notes.txt", Data: []byte{'a', 0, 'b'}},
		{Name: "notes.txt", Data: []byte{0xff}},
	} {
		if extractor.Supports(input) {
			t.Fatalf("Supports(%q) = true", input.Name)
		}
		if _, err := extractor.Extract(context.Background(), input); !errors.Is(err, text.ErrNotText) {
			t.Fatalf("Extract(%q) error = %v, want ErrNotText", input.Name, err)
		}
	}
}
