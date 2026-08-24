// Package kernel provides unified, recursive text extraction for PDF, HTML,
// Office Open XML, EML, and Outlook MSG documents.
package kernel

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"github.com/adrianliechti/go-kernel/pkg/eml"
	"github.com/adrianliechti/go-kernel/pkg/extract"
	htmlconv "github.com/adrianliechti/go-kernel/pkg/html"
	"github.com/adrianliechti/go-kernel/pkg/msg"
	"github.com/adrianliechti/go-kernel/pkg/ooxml"
	"github.com/adrianliechti/go-kernel/pkg/pdf"
)

type (
	Format     = extract.Format
	Input      = extract.Input
	Document   = extract.Document
	Attachment = extract.Attachment
	Extractor  = extract.Extractor
)

const (
	FormatUnknown = extract.FormatUnknown
	FormatPDF     = extract.FormatPDF
	FormatDOCX    = extract.FormatDOCX
	FormatXLSX    = extract.FormatXLSX
	FormatPPTX    = extract.FormatPPTX
	FormatHTML    = extract.FormatHTML
	FormatEML     = extract.FormatEML
	FormatMSG     = extract.FormatMSG
)

var (
	ErrUnsupportedFormat = extract.ErrUnsupportedFormat
	ErrResourceLimit     = extract.ErrResourceLimit
)

const (
	defaultMaxDepth           = 8
	defaultMaxDocuments       = 1_000
	defaultMaxAttachmentBytes = 128 << 20
)

// Options configures the unified dispatcher and its built-in extractors.
// Recursive attachment extraction is enabled by default.
type Options struct {
	PDF     pdf.Options
	OOXML   ooxml.Options
	HTML    htmlconv.Options
	Message msg.Options

	// Extractors are tried before the built-ins and can add or override format
	// support. The first extractor whose Supports method returns true wins.
	Extractors []Extractor

	// DisableRecursion leaves supported attachments as raw bytes only.
	DisableRecursion bool

	// MaxDepth bounds recursive attachment nesting. Zero uses 8.
	MaxDepth int

	// MaxDocuments bounds the total extracted document count. Zero uses 1000.
	MaxDocuments int

	// MaxAttachmentBytes bounds a single recursively extracted attachment.
	// Zero uses 128 MiB. A negative value disables this limit.
	MaxAttachmentBytes int64

	// DiscardAttachmentData removes raw attachment bytes after recursive
	// extraction. Attachment metadata and extracted child documents remain.
	DiscardAttachmentData bool
}

// Kernel dispatches inputs to format extractors and recursively processes
// supported attachments.
type Kernel struct {
	extractors         []Extractor
	disableRecursion   bool
	maxDepth           int
	maxDocuments       int
	maxAttachmentBytes int64
	discardData        bool
}

// New constructs a dispatcher with the built-in format extractors.
func New(opts Options) *Kernel {
	extractors := append([]Extractor(nil), opts.Extractors...)
	extractors = append(extractors,
		pdf.NewExtractor(opts.PDF),
		ooxml.NewExtractor(opts.OOXML),
		htmlconv.NewExtractor(opts.HTML),
		eml.NewExtractor(opts.Message),
		msg.NewExtractor(opts.Message),
	)

	maxDepth := opts.MaxDepth
	if maxDepth == 0 {
		maxDepth = defaultMaxDepth
	}
	maxDocuments := opts.MaxDocuments
	if maxDocuments == 0 {
		maxDocuments = defaultMaxDocuments
	}
	maxAttachmentBytes := opts.MaxAttachmentBytes
	if maxAttachmentBytes == 0 {
		maxAttachmentBytes = defaultMaxAttachmentBytes
	}

	return &Kernel{
		extractors:         extractors,
		disableRecursion:   opts.DisableRecursion,
		maxDepth:           maxDepth,
		maxDocuments:       maxDocuments,
		maxAttachmentBytes: maxAttachmentBytes,
		discardData:        opts.DiscardAttachmentData,
	}
}

// Extract uses the built-in dispatcher for one in-memory input.
func Extract(ctx context.Context, input Input, opts Options) (*Document, error) {
	return New(opts).Extract(ctx, input)
}

// ExtractFile reads and recursively extracts a file.
func ExtractFile(ctx context.Context, path string, opts Options) (*Document, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	doc, err := Extract(ctx, Input{Name: filepath.Base(path), Data: data}, opts)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", filepath.Base(path), err)
	}
	return doc, nil
}

// Extract dispatches one input and recursively extracts supported attachments.
func (k *Kernel) Extract(ctx context.Context, input Input) (*Document, error) {
	state := extractionState{}
	return k.extractAt(ctx, input, &state, 0)
}

type extractionState struct {
	documents int
}

func (k *Kernel) extractAt(ctx context.Context, input Input, state *extractionState, depth int) (*Document, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	extractor := k.match(input)
	if extractor == nil {
		return nil, extract.ErrUnsupportedFormat
	}
	if err := k.claimDocument(state); err != nil {
		return nil, err
	}
	doc, err := extractor.Extract(ctx, input)
	if err != nil {
		return nil, err
	}
	if doc.Name == "" {
		doc.Name = input.Name
	}
	if doc.MediaType == "" {
		doc.MediaType = input.MediaType
	}
	if !k.disableRecursion {
		if err := k.enrichAttachments(ctx, doc, state, depth); err != nil {
			return nil, err
		}
	} else {
		clearAttachmentDocuments(doc)
		if k.discardData {
			clearAttachmentData(doc)
		}
	}
	return doc, nil
}

func (k *Kernel) enrichAttachments(ctx context.Context, doc *Document, state *extractionState, depth int) error {
	for i := range doc.Attachments {
		if err := ctx.Err(); err != nil {
			return err
		}
		attachment := &doc.Attachments[i]
		if attachment.Document != nil {
			if k.maxAttachmentBytes >= 0 && int64(len(attachment.Data)) > k.maxAttachmentBytes {
				attachment.Document = nil
				attachment.Error = fmt.Errorf("%w: attachment %q is %d bytes (maximum %d)", extract.ErrResourceLimit, attachment.Name, len(attachment.Data), k.maxAttachmentBytes).Error()
			} else if depth >= k.maxDepth {
				attachment.Document = nil
				attachment.Error = k.depthError().Error()
			} else if err := k.claimDocument(state); err != nil {
				attachment.Document = nil
				attachment.Error = err.Error()
			} else if err := k.enrichAttachments(ctx, attachment.Document, state, depth+1); err != nil {
				return err
			}
		} else if len(attachment.Data) > 0 {
			input := Input{Name: attachment.Name, MediaType: attachment.MediaType, Data: attachment.Data}
			if k.match(input) != nil {
				switch {
				case depth >= k.maxDepth:
					attachment.Error = k.depthError().Error()
				case k.maxAttachmentBytes >= 0 && int64(len(attachment.Data)) > k.maxAttachmentBytes:
					attachment.Error = fmt.Errorf("%w: attachment %q is %d bytes (maximum %d)", extract.ErrResourceLimit, attachment.Name, len(attachment.Data), k.maxAttachmentBytes).Error()
				default:
					child, err := k.extractAt(ctx, input, state, depth+1)
					if err != nil {
						if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
							return err
						}
						attachment.Error = err.Error()
					} else {
						attachment.Document = child
					}
				}
			}
		}
		if k.discardData {
			attachment.Data = nil
		}
	}
	return nil
}

func (k *Kernel) match(input Input) Extractor {
	for _, extractor := range k.extractors {
		if extractor != nil && extractor.Supports(input) {
			return extractor
		}
	}
	return nil
}

func (k *Kernel) claimDocument(state *extractionState) error {
	if state.documents >= k.maxDocuments {
		return fmt.Errorf("%w: document count exceeds %d", extract.ErrResourceLimit, k.maxDocuments)
	}
	state.documents++
	return nil
}

func (k *Kernel) depthError() error {
	return fmt.Errorf("%w: attachment depth exceeds %d", extract.ErrResourceLimit, k.maxDepth)
}

func clearAttachmentData(doc *Document) {
	for i := range doc.Attachments {
		doc.Attachments[i].Data = nil
		if doc.Attachments[i].Document != nil {
			clearAttachmentData(doc.Attachments[i].Document)
		}
	}
}

func clearAttachmentDocuments(doc *Document) {
	for i := range doc.Attachments {
		doc.Attachments[i].Document = nil
	}
}
