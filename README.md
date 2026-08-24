# go-kernel

`go-kernel` is a unified, local text-extraction library for:

- PDF
- Word (`.docx`), Excel (`.xlsx`), and PowerPoint (`.pptx`)
- HTML documents and fragments
- Internet email (`.eml`)
- Outlook messages (`.msg`)

All formats produce Markdown through one interface. Email attachments are
recursively dispatched through the same extractors, so an EML containing a
PDF, an Office document, or another email yields a document tree.

```go
package main

import (
	"context"
	"fmt"
	"log"

	kernel "github.com/adrianliechti/go-kernel"
)

func main() {
	doc, err := kernel.ExtractFile(context.Background(), "message.eml", kernel.Options{})
	if err != nil {
		log.Fatal(err)
	}

	fmt.Print(doc.Markdown)
	for _, attachment := range doc.Attachments {
		if attachment.Document != nil {
			fmt.Printf("%s -> %s\n", attachment.Name, attachment.Document.Format)
		}
	}
}
```

Recursive extraction is on by default and bounded by depth, document-count,
and per-attachment size limits. Unsupported attachments remain available as
raw `Attachment.Data`; a supported attachment that fails extraction records a
non-fatal `Attachment.Error`.

The format-specific APIs remain available under `pkg/pdf`, `pkg/ooxml`,
`pkg/html`, `pkg/eml`, and `pkg/msg`. Each package also exposes an `Extractor`
implementing `pkg/extract.Extractor` for custom registries.

HTML can also be converted directly:

```go
import htmlconv "github.com/adrianliechti/go-kernel/pkg/html"

markdown, err := htmlconv.ToMarkdown(
	[]byte(`<h1>Report</h1><p>Ready to publish.</p>`),
	htmlconv.Options{BaseURL: "https://example.com/"},
)
```
