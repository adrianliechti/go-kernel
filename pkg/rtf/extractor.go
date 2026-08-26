package rtf

import (
	"context"

	"github.com/adrianliechti/go-kernel/pkg/extract"
)

// Extractor adapts RTF conversion to the unified extraction interface.
type Extractor struct {
	Options Options
}

// NewExtractor returns a unified RTF extractor.
func NewExtractor(opts Options) *Extractor {
	return &Extractor{Options: opts}
}

// Supports reports whether input has an RTF document header.
func (e *Extractor) Supports(input extract.Input) bool {
	return Detect(input.Data)
}

// Extract converts RTF into a format-neutral document.
func (e *Extractor) Extract(ctx context.Context, input extract.Input) (*extract.Document, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	doc, err := Convert(input.Data, e.Options)
	if err != nil {
		return nil, err
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	return &extract.Document{
		Name:      input.Name,
		Format:    extract.FormatRTF,
		MediaType: "application/rtf",
		Markdown:  doc.Markdown,
	}, nil
}

var _ extract.Extractor = (*Extractor)(nil)
