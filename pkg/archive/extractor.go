package archive

import (
	"context"
	"strconv"

	"github.com/adrianliechti/go-kernel/pkg/extract"
)

// Extractor adapts archive extraction to the unified extraction interface.
type Extractor struct {
	Options Options
}

// NewExtractor returns a unified ZIP, TAR, and GZIP extractor.
func NewExtractor(opts Options) *Extractor {
	return &Extractor{Options: opts}
}

// Supports reports whether input has a recognizable supported archive format.
func (e *Extractor) Supports(input extract.Input) bool {
	return Detect(input.Data) != FormatUnknown
}

// Extract exposes regular archive entries as recursively dispatched
// attachments.
func (e *Extractor) Extract(ctx context.Context, input extract.Input) (*extract.Document, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	doc, err := convert(input.Data, input.Name, e.Options)
	if err != nil {
		return nil, err
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}

	attachments := make([]extract.Attachment, 0, len(doc.Entries))
	for _, entry := range doc.Entries {
		attachment := extract.Attachment{
			Name:      entry.Name,
			MediaType: entry.MediaType,
			Data:      entry.Data,
		}
		if entry.OriginalName != "" {
			attachment.Metadata = map[string]string{"original_name": entry.OriginalName}
		}
		attachments = append(attachments, attachment)
	}

	return &extract.Document{
		Name:      input.Name,
		Format:    unifiedFormat(doc.Format),
		MediaType: mediaType(doc.Format),
		Markdown:  doc.Markdown,
		Metadata: map[string]string{
			"entry_count":          strconv.Itoa(len(doc.Entries)),
			"total_inflated_bytes": strconv.FormatUint(doc.TotalBytes, 10),
		},
		Attachments: attachments,
	}, nil
}

func unifiedFormat(format Format) extract.Format {
	switch format {
	case FormatZIP:
		return extract.FormatZIP
	case FormatTAR:
		return extract.FormatTAR
	case FormatGZIP:
		return extract.FormatGZIP
	default:
		return extract.FormatUnknown
	}
}

func mediaType(format Format) string {
	switch format {
	case FormatZIP:
		return "application/zip"
	case FormatTAR:
		return "application/x-tar"
	case FormatGZIP:
		return "application/gzip"
	default:
		return "application/octet-stream"
	}
}

var _ extract.Extractor = (*Extractor)(nil)
