package ooxml

import (
	"context"
	"strconv"
	"strings"

	"github.com/adrianliechti/go-kernel/pkg/extract"
)

// Extractor adapts the OOXML converter to the unified extraction interface.
type Extractor struct {
	Options Options
}

// NewExtractor returns a unified DOCX, XLSX, and PPTX extractor.
func NewExtractor(opts Options) *Extractor {
	return &Extractor{Options: opts}
}

// Supports reports whether input is a supported Office Open XML package.
func (e *Extractor) Supports(input extract.Input) bool {
	_, err := Detect(input.Data)
	return err == nil
}

// Extract converts an OOXML package into a format-neutral document.
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

	format, mediaType := unifiedFormat(doc.Format)
	metadata := make(map[string]string)
	if doc.Title != "" {
		metadata["title"] = doc.Title
	}
	if len(doc.SheetNames) > 0 {
		metadata["sheet_names"] = strings.Join(doc.SheetNames, ",")
	}
	if doc.SlideCount > 0 {
		metadata["slide_count"] = strconv.Itoa(doc.SlideCount)
	}
	attachments := make([]extract.Attachment, 0, len(doc.Images))
	for _, image := range doc.Images {
		attachments = append(attachments, extract.Attachment{
			Name:      image.Name,
			MediaType: image.ContentType,
			Inline:    true,
			Data:      image.Data,
			Metadata:  map[string]string{"source_part": image.Part},
		})
	}

	return &extract.Document{
		Name:        input.Name,
		Format:      format,
		MediaType:   mediaType,
		Markdown:    doc.Markdown,
		Metadata:    metadata,
		Attachments: attachments,
	}, nil
}

func unifiedFormat(format Format) (extract.Format, string) {
	switch format {
	case FormatDocx:
		return extract.FormatDOCX, "application/vnd.openxmlformats-officedocument.wordprocessingml.document"
	case FormatXlsx:
		return extract.FormatXLSX, "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet"
	case FormatPptx:
		return extract.FormatPPTX, "application/vnd.openxmlformats-officedocument.presentationml.presentation"
	default:
		return extract.FormatUnknown, "application/zip"
	}
}

var _ extract.Extractor = (*Extractor)(nil)
