package kernel_test

import (
	"archive/zip"
	"bytes"
	"context"
	"encoding/base64"
	"fmt"
	"strings"
	"testing"

	kernel "github.com/adrianliechti/go-kernel"
)

func TestExtractRecursivelyExtractsEmailAttachments(t *testing.T) {
	pdfData := buildPDF("Attached PDF text")
	docxData := buildDOCX(t, "Attached Word text")
	nested := buildEML("Nested message", []mailAttachment{
		{name: "inside.pdf", mediaType: "application/pdf", data: pdfData},
	})
	outer := buildEML("Outer message", []mailAttachment{
		{name: "report.docx", mediaType: "application/vnd.openxmlformats-officedocument.wordprocessingml.document", data: docxData},
		{name: "page.html", mediaType: "text/html", data: []byte("<html><head><title>Attached page</title></head><body><h2>HTML attachment text</h2></body></html>")},
		{name: "nested.eml", mediaType: "message/rfc822", data: nested},
		{name: "notes.bin", mediaType: "application/octet-stream", data: []byte("unsupported")},
	})

	doc, err := kernel.Extract(context.Background(), kernel.Input{Name: "outer.eml", Data: outer}, kernel.Options{})
	if err != nil {
		t.Fatal(err)
	}
	if doc.Format != kernel.FormatEML {
		t.Fatalf("Format = %q, want eml", doc.Format)
	}

	docx := attachmentNamed(t, doc, "report.docx")
	if docx.Document == nil || docx.Document.Format != kernel.FormatDOCX {
		t.Fatalf("DOCX attachment document = %#v", docx.Document)
	}
	if !strings.Contains(docx.Document.Markdown, "Attached Word text") {
		t.Fatalf("DOCX Markdown = %q", docx.Document.Markdown)
	}

	htmlAttachment := attachmentNamed(t, doc, "page.html")
	if htmlAttachment.Document == nil || htmlAttachment.Document.Format != kernel.FormatHTML {
		t.Fatalf("HTML attachment document = %#v", htmlAttachment.Document)
	}
	if !strings.Contains(htmlAttachment.Document.Markdown, "## HTML attachment text") {
		t.Fatalf("HTML Markdown = %q", htmlAttachment.Document.Markdown)
	}

	nestedMessage := attachmentNamed(t, doc, "nested.eml")
	if nestedMessage.Document == nil || nestedMessage.Document.Format != kernel.FormatEML {
		t.Fatalf("nested EML document = %#v", nestedMessage.Document)
	}
	nestedPDF := attachmentNamed(t, nestedMessage.Document, "inside.pdf")
	if nestedPDF.Document == nil || nestedPDF.Document.Format != kernel.FormatPDF {
		t.Fatalf("nested PDF document = %#v, error = %q", nestedPDF.Document, nestedPDF.Error)
	}
	if !strings.Contains(nestedPDF.Document.Markdown, "Attached PDF text") {
		t.Fatalf("PDF Markdown = %q", nestedPDF.Document.Markdown)
	}

	unsupported := attachmentNamed(t, doc, "notes.bin")
	if unsupported.Document != nil || unsupported.Error != "" || string(unsupported.Data) != "unsupported" {
		t.Fatalf("unsupported attachment = %#v", unsupported)
	}
}

func TestExtractAppliesRecursiveDepthLimitNonFatally(t *testing.T) {
	nested := buildEML("Nested message", []mailAttachment{
		{name: "inside.pdf", mediaType: "application/pdf", data: buildPDF("too deep")},
	})
	outer := buildEML("Outer message", []mailAttachment{
		{name: "nested.eml", mediaType: "message/rfc822", data: nested},
	})

	doc, err := kernel.Extract(context.Background(), kernel.Input{Name: "outer.eml", Data: outer}, kernel.Options{MaxDepth: 1})
	if err != nil {
		t.Fatal(err)
	}
	nestedMessage := attachmentNamed(t, doc, "nested.eml")
	if nestedMessage.Document == nil {
		t.Fatal("first attachment level was not extracted")
	}
	nestedPDF := attachmentNamed(t, nestedMessage.Document, "inside.pdf")
	if nestedPDF.Document != nil || !strings.Contains(nestedPDF.Error, "attachment depth exceeds 1") {
		t.Fatalf("depth-limited attachment = %#v", nestedPDF)
	}
}

func TestExtractCanDisableRecursionAndDiscardBytes(t *testing.T) {
	outer := buildEML("Outer message", []mailAttachment{
		{name: "nested.eml", mediaType: "message/rfc822", data: buildEML("Nested", nil)},
	})

	doc, err := kernel.Extract(context.Background(), kernel.Input{Name: "outer.eml", Data: outer}, kernel.Options{
		DisableRecursion:      true,
		DiscardAttachmentData: true,
	})
	if err != nil {
		t.Fatal(err)
	}
	attachment := attachmentNamed(t, doc, "nested.eml")
	if attachment.Document != nil || attachment.Data != nil {
		t.Fatalf("attachment = %#v, want metadata only", attachment)
	}
}

func attachmentNamed(t *testing.T, doc *kernel.Document, name string) *kernel.Attachment {
	t.Helper()
	for i := range doc.Attachments {
		if doc.Attachments[i].Name == name {
			return &doc.Attachments[i]
		}
	}
	t.Fatalf("attachment %q not found in %#v", name, doc.Attachments)
	return nil
}

type mailAttachment struct {
	name      string
	mediaType string
	data      []byte
}

func buildEML(subject string, attachments []mailAttachment) []byte {
	const boundary = "go-kernel-boundary"
	var message strings.Builder
	fmt.Fprintf(&message, "From: sender@example.com\r\nTo: recipient@example.com\r\nSubject: %s\r\nMIME-Version: 1.0\r\nContent-Type: multipart/mixed; boundary=%q\r\n\r\n", subject, boundary)
	fmt.Fprintf(&message, "--%s\r\nContent-Type: text/plain; charset=utf-8\r\n\r\nBody for %s\r\n", boundary, subject)
	for _, attachment := range attachments {
		fmt.Fprintf(&message, "--%s\r\nContent-Type: %s; name=%q\r\nContent-Disposition: attachment; filename=%q\r\nContent-Transfer-Encoding: base64\r\n\r\n%s\r\n", boundary, attachment.mediaType, attachment.name, attachment.name, base64.StdEncoding.EncodeToString(attachment.data))
	}
	fmt.Fprintf(&message, "--%s--\r\n", boundary)
	return []byte(message.String())
}

func buildDOCX(t *testing.T, text string) []byte {
	t.Helper()
	var data bytes.Buffer
	writer := zip.NewWriter(&data)
	parts := map[string]string{
		"[Content_Types].xml": `<Types xmlns="http://schemas.openxmlformats.org/package/2006/content-types"><Default Extension="xml" ContentType="application/xml"/><Default Extension="rels" ContentType="application/vnd.openxmlformats-package.relationships+xml"/></Types>`,
		"_rels/.rels":         `<Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships"><Relationship Id="rDoc" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/officeDocument" Target="word/document.xml"/></Relationships>`,
		"word/document.xml":   `<w:document xmlns:w="http://schemas.openxmlformats.org/wordprocessingml/2006/main"><w:body><w:p><w:r><w:t>` + text + `</w:t></w:r></w:p></w:body></w:document>`,
	}
	for name, content := range parts {
		part, err := writer.Create(name)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := part.Write([]byte(content)); err != nil {
			t.Fatal(err)
		}
	}
	if err := writer.Close(); err != nil {
		t.Fatal(err)
	}
	return data.Bytes()
}

func buildPDF(text string) []byte {
	content := fmt.Sprintf("BT\n/F1 18 Tf\n72 720 Td\n(%s) Tj\nET\n", strings.NewReplacer("\\", "\\\\", "(", "\\(", ")", "\\)").Replace(text))
	objects := []string{
		"<< /Type /Catalog /Pages 2 0 R >>",
		"<< /Type /Pages /Kids [3 0 R] /Count 1 >>",
		"<< /Type /Page /Parent 2 0 R /MediaBox [0 0 612 792] /Resources << /Font << /F1 5 0 R >> >> /Contents 4 0 R >>",
		fmt.Sprintf("<< /Length %d >>\nstream\n%sendstream", len(content), content),
		"<< /Type /Font /Subtype /Type1 /BaseFont /Helvetica >>",
	}

	var pdf bytes.Buffer
	pdf.WriteString("%PDF-1.4\n")
	offsets := make([]int, len(objects)+1)
	for i, object := range objects {
		offsets[i+1] = pdf.Len()
		fmt.Fprintf(&pdf, "%d 0 obj\n%s\nendobj\n", i+1, object)
	}
	xref := pdf.Len()
	fmt.Fprintf(&pdf, "xref\n0 %d\n0000000000 65535 f \n", len(offsets))
	for i := 1; i < len(offsets); i++ {
		fmt.Fprintf(&pdf, "%010d 00000 n \n", offsets[i])
	}
	fmt.Fprintf(&pdf, "trailer\n<< /Size %d /Root 1 0 R >>\nstartxref\n%d\n%%%%EOF\n", len(offsets), xref)
	return pdf.Bytes()
}
