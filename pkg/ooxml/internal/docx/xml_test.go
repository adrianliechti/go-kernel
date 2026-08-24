package docx

import (
	"encoding/xml"
	"strings"
	"testing"
)

func TestParagraphTrackedMovesUseFinalView(t *testing.T) {
	const input = `<w:p xmlns:w="http://schemas.openxmlformats.org/wordprocessingml/2006/main">
		<w:r><w:t xml:space="preserve">Keep </w:t></w:r>
		<w:ins w:id="1"><w:r><w:t>added</w:t></w:r></w:ins>
		<w:moveFrom w:id="2"><w:r><w:t> moved away</w:t></w:r></w:moveFrom>
		<w:moveTo w:id="2"><w:r><w:t> and moved here</w:t></w:r></w:moveTo>
		<w:del w:id="3"><w:r><w:delText> removed</w:delText></w:r></w:del>
	</w:p>`

	var p paragraph
	if err := xml.NewDecoder(strings.NewReader(input)).Decode(&p); err != nil {
		t.Fatal(err)
	}

	var got strings.Builder
	for _, item := range p.Content {
		if item.Run != nil {
			got.WriteString(item.Run.Text)
		}
	}
	if want := "Keep added and moved here"; got.String() != want {
		t.Fatalf("tracked-change text = %q, want %q", got.String(), want)
	}
}
