package report

import (
	"bytes"
	"fmt"
	"strconv"
	"strings"
)

const maxPDFBytes = 16 << 20

// RenderPDF writes a deterministic, paginated PDF 1.4 document with no
// external font or native runtime dependency. All text is WinAnsi encoded.
func RenderPDF(document Document) ([]byte, error) {
	lines := BuildLines(document)
	pages := [][]string{{}}
	y := 790
	for _, line := range lines {
		fontSize, step := 10, 14
		if line.Style == "title" {
			fontSize, step = 18, 26
		}
		if line.Style == "heading" {
			fontSize, step = 13, 21
		}
		if line.Style == "subheading" {
			fontSize, step = 10, 17
		}
		width := 96
		if fontSize == 18 {
			width = 54
		}
		if fontSize == 13 {
			width = 75
		}
		for _, part := range wrapText(line.Text, width) {
			if y < 55 {
				pages = append(pages, []string{})
				y = 790
			}
			command := fmt.Sprintf("BT /F1 %d Tf 42 %d Td (%s) Tj ET\n", fontSize, y, escapePDFText(part))
			pages[len(pages)-1] = append(pages[len(pages)-1], command)
			y -= step
			fontSize, step = 10, 14
		}
	}
	objects := []string{"", "", "<< /Type /Font /Subtype /Type1 /BaseFont /Helvetica /Encoding /WinAnsiEncoding >>"}
	kids := []string{}
	for index, commands := range pages {
		pageID := len(objects) + 1
		contentID := pageID + 1
		kids = append(kids, fmt.Sprintf("%d 0 R", pageID))
		stream := strings.Join(commands, "") + fmt.Sprintf("BT /F1 9 Tf 42 30 Td (Pagina %d de %d) Tj ET\n", index+1, len(pages))
		objects = append(objects, fmt.Sprintf("<< /Type /Page /Parent 2 0 R /MediaBox [0 0 595 842] /Resources << /Font << /F1 3 0 R >> >> /Contents %d 0 R >>", contentID), fmt.Sprintf("<< /Length %d >>\nstream\n%sendstream", len(stream), stream))
	}
	objects[0] = "<< /Type /Catalog /Pages 2 0 R >>"
	objects[1] = fmt.Sprintf("<< /Type /Pages /Kids [%s] /Count %d >>", strings.Join(kids, " "), len(pages))
	var out bytes.Buffer
	out.WriteString("%PDF-1.4\n%\xe2\xe3\xcf\xd3\n")
	offsets := []int{0}
	for index, object := range objects {
		offsets = append(offsets, out.Len())
		fmt.Fprintf(&out, "%d 0 obj\n%s\nendobj\n", index+1, object)
		if out.Len() > maxPDFBytes {
			return nil, fmt.Errorf("report exceeds %d bytes", maxPDFBytes)
		}
	}
	xref := out.Len()
	fmt.Fprintf(&out, "xref\n0 %d\n0000000000 65535 f \n", len(offsets))
	for _, offset := range offsets[1:] {
		fmt.Fprintf(&out, "%010d 00000 n \n", offset)
	}
	fmt.Fprintf(&out, "trailer\n<< /Size %d /Root 1 0 R >>\nstartxref\n%d\n%%%%EOF\n", len(offsets), xref)
	if out.Len() > maxPDFBytes {
		return nil, fmt.Errorf("report exceeds %d bytes", maxPDFBytes)
	}
	return out.Bytes(), nil
}

func wrapText(value string, width int) []string {
	value = strings.Join(strings.Fields(value), " ")
	if value == "" {
		return []string{""}
	}
	words := strings.Fields(value)
	out := []string{}
	line := ""
	for _, word := range words {
		for len([]rune(word)) > width {
			if line != "" {
				out = append(out, line)
				line = ""
			}
			runes := []rune(word)
			out = append(out, string(runes[:width]))
			word = string(runes[width:])
		}
		if len([]rune(line))+len([]rune(word))+1 > width {
			out = append(out, line)
			line = ""
		}
		if line != "" {
			line += " "
		}
		line += word
	}
	if line != "" {
		out = append(out, line)
	}
	return out
}

func escapePDFText(value string) string {
	var out strings.Builder
	for _, r := range value {
		b := byte('?')
		if r >= 32 && r <= 255 {
			b = byte(r)
		}
		switch b {
		case '\\', '(', ')':
			out.WriteByte('\\')
			out.WriteByte(b)
		case '\n', '\r':
			out.WriteByte(' ')
		default:
			out.WriteByte(b)
		}
	}
	return out.String()
}

// PageCount is intentionally small and useful to tests without a PDF parser.
func PageCount(pdf []byte) int { return strings.Count(string(pdf), "/Type /Page /Parent") }

func ValidatePDF(pdf []byte) error {
	if !bytes.HasPrefix(pdf, []byte("%PDF-1.4")) || !bytes.HasSuffix(pdf, []byte("%%EOF\n")) {
		return fmt.Errorf("invalid PDF envelope")
	}
	if PageCount(pdf) < 1 {
		return fmt.Errorf("PDF has no pages")
	}
	_, err := strconv.Atoi(strings.TrimSpace(string(pdf[bytes.LastIndex(pdf, []byte("startxref\n"))+10 : bytes.LastIndex(pdf, []byte("\n%%EOF"))])))
	return err
}
