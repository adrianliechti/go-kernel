# go-kernel

`go-kernel` is a unified, local text-extraction library for:

- PDF
- Word (`.docx`, `.docm`, `.dotx`, `.dotm`)
- Excel (`.xlsx`, `.xlsm`, `.xltx`, `.xltm`)
- PowerPoint (`.pptx`, `.pptm`, `.ppsx`, `.ppsm`, `.potx`, `.potm`)
- Rich Text Format (`.rtf`)
- ZIP, TAR, and GZIP archives, including nested archives
- Plain text and Markdown (`.txt`, `.md`, `.markdown`)
- HTML documents and fragments
- Internet email (`.eml`)
- Outlook messages (`.msg`)

All formats produce Markdown through one interface. Attachments and archive
entries are recursively dispatched through the same extractors, so chains such
as EML → ZIP → TAR/GZIP → Office document yield a document tree.

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
non-fatal `Attachment.Error`. Archive extraction additionally limits entry
count, per-entry inflated bytes, and total inflated bytes.

The format-specific APIs remain available under `pkg/archive`, `pkg/pdf`,
`pkg/ooxml`, `pkg/rtf`, `pkg/text`, `pkg/html`, `pkg/eml`, and `pkg/msg`. Each
package also exposes an `Extractor` implementing `pkg/extract.Extractor` for
custom registries.

HTML can also be converted directly:

```go
import htmlconv "github.com/adrianliechti/go-kernel/pkg/html"

markdown, err := htmlconv.ToMarkdown(
	[]byte(`<h1>Report</h1><p>Ready to publish.</p>`),
	htmlconv.Options{BaseURL: "https://example.com/"},
)
```

## WebAssembly example

The complete extractor can run locally in a browser through Go WebAssembly.
Build and serve the included file-upload and HTML demo with:

```sh
task --dir examples/wasm serve
```

Then open <http://localhost:8080>. See [`examples/wasm`](examples/wasm) for
details.
