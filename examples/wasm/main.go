//go:build js && wasm

package main

import (
	"context"
	"encoding/json"
	"fmt"
	"syscall/js"

	kernel "github.com/adrianliechti/go-kernel"
)

type browserResult struct {
	Document *browserDocument `json:"document,omitempty"`
	Error    string           `json:"error,omitempty"`
}

type browserDocument struct {
	Name        string              `json:"name,omitempty"`
	Format      kernel.Format       `json:"format"`
	MediaType   string              `json:"mediaType,omitempty"`
	Markdown    string              `json:"markdown"`
	Metadata    map[string]string   `json:"metadata,omitempty"`
	Attachments []browserAttachment `json:"attachments,omitempty"`
}

type browserAttachment struct {
	Name      string            `json:"name,omitempty"`
	MediaType string            `json:"mediaType,omitempty"`
	Inline    bool              `json:"inline,omitempty"`
	Size      int               `json:"size"`
	Metadata  map[string]string `json:"metadata,omitempty"`
	Document  *browserDocument  `json:"document,omitempty"`
	Error     string            `json:"error,omitempty"`
}

var extractFunc js.Func

func main() {
	extractFunc = js.FuncOf(extract)

	api := js.Global().Get("Object").New()
	api.Set("extract", extractFunc)
	js.Global().Set("goKernel", api)

	// Keep the Go runtime alive so JavaScript can call the exported function.
	select {}
}

func extract(_ js.Value, args []js.Value) any {
	if len(args) != 3 {
		return resolvedPromise(browserResult{Error: "extract expects name, media type, and Uint8Array arguments"})
	}

	data, err := copyBytes(args[2])
	if err != nil {
		return resolvedPromise(browserResult{Error: err.Error()})
	}

	name := args[0].String()
	mediaType := args[1].String()
	return extractionPromise(func() (result browserResult) {
		defer func() {
			if recovered := recover(); recovered != nil {
				result = browserResult{Error: fmt.Sprintf("go-kernel panic: %v", recovered)}
			}
		}()

		doc, err := kernel.Extract(context.Background(), kernel.Input{
			Name:      name,
			MediaType: mediaType,
			Data:      data,
		}, kernel.Options{})
		if err != nil {
			return browserResult{Error: err.Error()}
		}

		return browserResult{Document: toBrowserDocument(doc)}
	})
}

func copyBytes(value js.Value) ([]byte, error) {
	uint8Array := js.Global().Get("Uint8Array")
	if value.Type() != js.TypeObject || !value.InstanceOf(uint8Array) {
		return nil, fmt.Errorf("document data must be a Uint8Array")
	}

	data := make([]byte, value.Get("byteLength").Int())
	if copied := js.CopyBytesToGo(data, value); copied != len(data) {
		return nil, fmt.Errorf("copied %d of %d input bytes", copied, len(data))
	}
	return data, nil
}

func toBrowserDocument(doc *kernel.Document) *browserDocument {
	if doc == nil {
		return nil
	}

	result := &browserDocument{
		Name:      doc.Name,
		Format:    doc.Format,
		MediaType: doc.MediaType,
		Markdown:  doc.Markdown,
		Metadata:  doc.Metadata,
	}
	for _, attachment := range doc.Attachments {
		result.Attachments = append(result.Attachments, browserAttachment{
			Name:      attachment.Name,
			MediaType: attachment.MediaType,
			Inline:    attachment.Inline,
			Size:      len(attachment.Data),
			Metadata:  attachment.Metadata,
			Document:  toBrowserDocument(attachment.Document),
			Error:     attachment.Error,
		})
	}
	return result
}

func encodeResult(result browserResult) any {
	data, err := json.Marshal(result)
	if err != nil {
		data = []byte(fmt.Sprintf(`{"error":%q}`, err.Error()))
	}
	return js.Global().Get("JSON").Call("parse", string(data))
}

func resolvedPromise(result browserResult) js.Value {
	return js.Global().Get("Promise").Call("resolve", encodeResult(result))
}

func extractionPromise(extract func() browserResult) js.Value {
	var executor js.Func
	executor = js.FuncOf(func(_ js.Value, args []js.Value) any {
		resolve := args[0]
		go func() {
			resolve.Invoke(encodeResult(extract()))
		}()
		return nil
	})

	promise := js.Global().Get("Promise").New(executor)
	executor.Release()
	return promise
}
