package ooxml_test

import (
	"context"
	"testing"

	"github.com/adrianliechti/go-kernel/pkg/extract"
	"github.com/adrianliechti/go-kernel/pkg/ooxml"
)

func TestExtractorReportsOfficeVariantMediaTypes(t *testing.T) {
	word := minimalOfficePackage(t, "word/document.xml")
	excel := minimalOfficePackage(t, "xl/workbook.xml")
	powerPoint := minimalOfficePackage(t, "ppt/presentation.xml")

	tests := []struct {
		name      string
		data      []byte
		format    extract.Format
		mediaType string
	}{
		{"file.docx", word, extract.FormatDOCX, "application/vnd.openxmlformats-officedocument.wordprocessingml.document"},
		{"file.docm", word, extract.FormatDOCX, "application/vnd.ms-word.document.macroEnabled.12"},
		{"file.dotx", word, extract.FormatDOCX, "application/vnd.openxmlformats-officedocument.wordprocessingml.template"},
		{"file.dotm", word, extract.FormatDOCX, "application/vnd.ms-word.template.macroEnabled.12"},
		{"file.xlsx", excel, extract.FormatXLSX, "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet"},
		{"file.xlsm", excel, extract.FormatXLSX, "application/vnd.ms-excel.sheet.macroEnabled.12"},
		{"file.xltx", excel, extract.FormatXLSX, "application/vnd.openxmlformats-officedocument.spreadsheetml.template"},
		{"file.xltm", excel, extract.FormatXLSX, "application/vnd.ms-excel.template.macroEnabled.12"},
		{"file.pptx", powerPoint, extract.FormatPPTX, "application/vnd.openxmlformats-officedocument.presentationml.presentation"},
		{"file.pptm", powerPoint, extract.FormatPPTX, "application/vnd.ms-powerpoint.presentation.macroEnabled.12"},
		{"file.ppsx", powerPoint, extract.FormatPPTX, "application/vnd.openxmlformats-officedocument.presentationml.slideshow"},
		{"file.ppsm", powerPoint, extract.FormatPPTX, "application/vnd.ms-powerpoint.slideshow.macroEnabled.12"},
		{"file.potx", powerPoint, extract.FormatPPTX, "application/vnd.openxmlformats-officedocument.presentationml.template"},
		{"file.potm", powerPoint, extract.FormatPPTX, "application/vnd.ms-powerpoint.template.macroEnabled.12"},
	}

	extractor := ooxml.NewExtractor(ooxml.Options{})
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			input := extract.Input{Name: test.name, Data: test.data}
			if !extractor.Supports(input) {
				t.Fatal("Supports rejected OOXML package")
			}
			doc, err := extractor.Extract(context.Background(), input)
			if err != nil {
				t.Fatal(err)
			}
			if doc.Format != test.format || doc.MediaType != test.mediaType {
				t.Fatalf("format/media type = %q, %q; want %q, %q", doc.Format, doc.MediaType, test.format, test.mediaType)
			}
		})
	}
}

func TestExtractorUsesMatchingMediaTypeHintWithoutExtension(t *testing.T) {
	data := minimalOfficePackage(t, "ppt/presentation.xml")
	input := extract.Input{
		Name:      "upload",
		MediaType: "application/vnd.openxmlformats-officedocument.presentationml.slideshow; version=1",
		Data:      data,
	}

	doc, err := ooxml.NewExtractor(ooxml.Options{}).Extract(context.Background(), input)
	if err != nil {
		t.Fatal(err)
	}
	if doc.MediaType != "application/vnd.openxmlformats-officedocument.presentationml.slideshow" {
		t.Fatalf("MediaType = %q", doc.MediaType)
	}
}

func minimalOfficePackage(t *testing.T, mainPart string) []byte {
	t.Helper()
	return buildOOXML(t, []testPackagePart{
		{name: "[Content_Types].xml", text: basicContentTypes()},
		{name: "_rels/.rels", text: rootDocumentRels(mainPart)},
		{name: mainPart, text: `<?xml version="1.0"?><root/>`},
	})
}
