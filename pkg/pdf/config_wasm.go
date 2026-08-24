//go:build js && wasm

package pdf

import "github.com/pdfcpu/pdfcpu/pkg/pdfcpu/model"

func init() {
	// Browsers do not provide the user configuration directory that pdfcpu
	// initializes by default. Use pdfcpu's embedded defaults instead.
	model.ConfigPath = "disable"
}
