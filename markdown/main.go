// Package markdown provides a Markdown renderer.
package markdown

import (
	"bytes"
	"fmt"
	"go/format"
	"io"
	"io/ioutil"
	"strings"

	md "github.com/gomarkdown/markdown"
	"github.com/gomarkdown/markdown/ast"
	"github.com/gomarkdown/markdown/parser"
	"github.com/mattn/go-runewidth"
	"github.com/shurcooL/go/indentwriter"
)

// TODO: copy indentwriter locally to minimize dependencies

// RenderNodeFunc allows reusing most of Renderer logic and replacing
// rendering of some nodes. If it returns false, Renderer.RenderNode
// will execute its logic. If it returns true, Renderer.RenderNode will
// skip rendering this node and will return WalkStatus
type RenderNodeFunc func(w io.Writer, node ast.Node, entering bool) (ast.WalkStatus, bool)

// Options specifies options for formatting.
type Options struct {
	// Terminal specifies if ANSI escape codes are emitted for styling.
	Terminal bool

	// if set, called at the start of RenderNode(). Allows replacing
	// rendering of some nodes
	RenderNodeHook RenderNodeFunc
}

// Renderer is a markdown renderer
type Renderer struct {
	//normalTextMarker   map[*bytes.Buffer]int
	orderedListCounter map[int]int
	paragraph          map[int]bool // Used to keep track of whether a given list item uses a paragraph for large spacing.
	listDepth          int
	lastNormalText     string

	// TODO: Clean these up.
	headers       []string
	columnAligns  []int
	columnWidths  []int
	cells         []string
	lastOutputLen int

	opts Options

	// stringWidth is used internally to calculate visual width of a string.
	stringWidth func(s string) (width int)
}

// NewRenderer returns a Markdown renderer.
// If opt is nil the defaults are used.
func NewRenderer(opts *Options) *Renderer {
	r := &Renderer{
		// normalTextMarker:   make(map[*bytes.Buffer]int),
		orderedListCounter: make(map[int]int),
		paragraph:          make(map[int]bool),

		stringWidth: runewidth.StringWidth,
	}
	if opts != nil {
		r.opts = *opts
	}
	if r.opts.Terminal {
		r.stringWidth = terminalStringWidth
	}
	return r
}

func (r *Renderer) out(w io.Writer, d []byte) {
	r.lastOutputLen = len(d)
	w.Write(d)
}

func (r *Renderer) outs(w io.Writer, s string) {
	r.lastOutputLen = len(s)
	io.WriteString(w, s)
}

func (r *Renderer) cr(w io.Writer) {
	if r.lastOutputLen > 0 {
		r.outs(w, "\n")
	}
}

func (r *Renderer) doubleSpace(w io.Writer) {
	// TODO: need to remember number of written bytes
	//if out.Len() > 0 {
	r.outs(w, "\n")
	//}
}

// RenderNode renders markdown node to cleaned-up markdown
func (r *Renderer) RenderNode(w io.Writer, node ast.Node, entering bool) ast.WalkStatus {
	if r.opts.RenderNodeHook != nil {
		status, didHandle := r.opts.RenderNodeHook(w, node, entering)
		if didHandle {
			return status
		}
	}

	switch node := node.(type) {
	case *ast.Text:
		r.text(w, node)
	case *ast.Softbreak:
		r.cr(w)
	case *ast.Hardbreak:
		r.outs(w, "  \n")
	case *ast.Emph:
		r.emphasis(w, node)
	case *ast.Strong:
		r.strong(w, node)
	case *ast.Del:
		r.del(w, node)
	case *ast.BlockQuote:
		r.blockQuote(w, node)
	case *ast.Link:
		r.link(w, node)
	case *ast.Image:
		r.image(w, node)
	case *ast.Code:
		r.code(w, node)
	case *ast.CodeBlock:
		r.codeBlock(w, node)
	case *ast.Document:
		// do nothing
	case *ast.Paragraph:
		r.doParagraph(w, node)
	case *ast.HTMLSpan:
		r.htmlSpan(w, node)
	case *ast.HTMLBlock:
		r.htmlBlock(w, node)
	case *ast.Heading:
		r.heading(w, node)
	case *ast.HorizontalRule:
		r.horizontalRule(w)
	case *ast.List:
		r.list(w, node)
	case *ast.ListItem:
		r.listItem(w, node)

		/*
			case *ast.Table:
				r.outOneOfCr(w, entering, "<table>", "</table>")
			case *ast.TableCell:
				r.tableCell(w, node, entering)
			case *ast.TableHead:
				r.outOneOfCr(w, entering, "<thead>", "</thead>")
			case *ast.TableBody:
				r.tableBody(w, node, entering)
			case *ast.TableRow:
				r.outOneOfCr(w, entering, "<tr>", "</tr>")
		*/
	default:
		// panic(fmt.Sprintf("Unknown node %T", node))
	}

	return ast.GoToNext
}

func (r *Renderer) list(w io.Writer, node *ast.List) {
	flags := node.ListFlags
	// TODO:
	// marker := out.Len()
	r.doubleSpace(w)

	r.listDepth++
	defer func() { r.listDepth-- }()
	if flags&ast.ListTypeOrdered != 0 {
		r.orderedListCounter[r.listDepth] = 1
	}
	// TODO:
	/*
		if !text() {
			out.Truncate(marker)
			return
		}
	*/
}

func (r *Renderer) listItem(w io.Writer, node *ast.ListItem) {
	// out *bytes.Buffer, text []byte, flags int) {
	flags := node.ListFlags
	text := node.Literal

	if flags&ast.ListTypeOrdered != 0 {
		fmt.Fprintf(w, "%d.", r.orderedListCounter[r.listDepth])
		indentwriter.New(w, 1).Write(text)
		r.orderedListCounter[r.listDepth]++
	} else {
		r.outs(w, "-")
		indentwriter.New(w, 1).Write(text)
	}
	r.outs(w, "\n")
	if r.paragraph[r.listDepth] {
		if flags&ast.ListItemEndOfList == 0 {
			r.outs(w, "\n")
		}
		r.paragraph[r.listDepth] = false
	}
}

func (r *Renderer) horizontalRule(w io.Writer) {
	r.doubleSpace(w)
	r.outs(w, "---\n")
}

func (r *Renderer) heading(w io.Writer, node *ast.Heading) {
	// TODO:
	//marker := out.Len()
	r.doubleSpace(w)

	level := node.Level

	if level >= 3 {
		s := strings.Repeat("#", level) + " "
		r.outs(w, s)
	}

	// TODO:
	/*
		textMarker := out.Len()
		if !text() {
			out.Truncate(marker)
			return
		}*/

	// TODO: need to handle this
	/*
		switch level {
		case 1:
			len := r.stringWidth(out.String()[textMarker:])
			fmt.Fprint(out, "\n", strings.Repeat("=", len))
		case 2:
			len := r.stringWidth(out.String()[textMarker:])
			fmt.Fprint(out, "\n", strings.Repeat("-", len))
		}
		r.outs(w, "\n")
	*/
}

func (r *Renderer) htmlSpan(w io.Writer, node *ast.HTMLSpan) {
	r.out(w, node.Literal)
}

func (r *Renderer) htmlBlock(w io.Writer, node *ast.HTMLBlock) {
	r.doubleSpace(w)
	r.out(w, node.Literal)
	r.outs(w, "\n")
}

// TODO: rename to para()
func (r *Renderer) doParagraph(w io.Writer, node *ast.Paragraph) {
	// marker := out.Len()
	r.doubleSpace(w)

	r.paragraph[r.listDepth] = true

	/*
		if !text() {
			out.Truncate(marker)
			return
		}
	*/
	r.outs(w, "\n")
}

// TODO: push this to caller to minimize dependencies
func formatCode(lang string, text []byte) (formattedCode []byte, ok bool) {
	switch lang {
	case "Go", "go":
		gofmt, err := format.Source(text)
		if err != nil {
			return nil, false
		}
		return gofmt, true
	default:
		return nil, false
	}
}

func (r *Renderer) codeBlock(w io.Writer, node *ast.CodeBlock) {
	r.doubleSpace(w)
	text := node.Literal
	lang := string(node.Info)
	// Parse out the language name.
	count := 0
	for _, elt := range strings.Fields(lang) {
		if elt[0] == '.' {
			elt = elt[1:]
		}
		if len(elt) == 0 {
			continue
		}
		r.outs(w, "```")
		r.outs(w, elt)
		count++
		break
	}

	if count == 0 {
		r.outs(w, "```")
	}
	r.outs(w, "\n")

	if formattedCode, ok := formatCode(lang, text); ok {
		r.out(w, formattedCode)
	} else {
		r.out(w, text)
	}

	r.outs(w, "```\n")
}

func (r *Renderer) code(w io.Writer, node *ast.Code) {
	r.outs(w, "`")
	r.out(w, node.Literal)
	r.outs(w, "`")
}

func (r *Renderer) image(w io.Writer, node *ast.Image) {
	link := node.Destination
	title := node.Title
	// alt := node. ??
	var alt []byte
	r.outs(w, "![")
	r.out(w, alt)
	r.outs(w, "](")
	r.out(w, escape(link))
	if len(title) != 0 {
		r.outs(w, ` "`)
		r.out(w, title)
		r.outs(w, `"`)
	}
	r.outs(w, ")")
}

func (r *Renderer) link(w io.Writer, node *ast.Link) {
	link := node.Destination
	title := node.Title
	content := node.Literal
	r.outs(w, "[")
	r.out(w, content)
	r.outs(w, "](")
	r.out(w, escape(link))
	if len(title) != 0 {
		r.outs(w, ` "`)
		r.out(w, title)
		r.outs(w, `"`)
	}
	r.outs(w, ")")
}

func (r *Renderer) blockQuote(w io.Writer, node *ast.BlockQuote) {
	text := node.Literal
	r.doubleSpace(w)
	lines := bytes.Split(text, []byte("\n"))
	for i, line := range lines {
		if i == len(lines)-1 {
			continue
		}
		r.outs(w, ">")
		if len(line) != 0 {
			r.outs(w, " ")
			r.out(w, line)
		}
		r.outs(w, "\n")
	}
}

func (r *Renderer) del(w io.Writer, node *ast.Del) {
	r.outs(w, "~~")
	r.out(w, node.Literal)
	r.outs(w, "~~")
}

func (r *Renderer) strong(w io.Writer, node *ast.Strong) {
	text := node.Literal
	if r.opts.Terminal {
		r.outs(w, "\x1b[1m") // bold
	}
	r.outs(w, "**")
	r.out(w, text)
	r.outs(w, "**")
	if r.opts.Terminal {
		r.outs(w, "\x1b[0m") // reset
	}
}

func (r *Renderer) emphasis(w io.Writer, node *ast.Emph) {
	text := node.Literal
	if len(text) == 0 {
		return
	}
	r.outs(w, "*")
	r.out(w, text)
	r.outs(w, "*")
}

func (r *Renderer) skipSpaceIfNeededNormalText(w io.Writer, cleanString string) bool {
	if cleanString[0] != ' ' {
		return false
	}

	return false
	//  TODO: what did it mean to do?
	// we no longer use *bytes.Buffer for out, so whatever this tracked,
	// it has to be done in a different wy
	/*
		if _, ok := r.normalTextMarker[out]; !ok {
			r.normalTextMarker[out] = -1
		}
		return r.normalTextMarker[out] == out.Len()
	*/
}

func (r *Renderer) text(w io.Writer, text *ast.Text) {
	lit := text.Literal
	normalText := string(text.Literal)
	if needsEscaping(lit, r.lastNormalText) {
		lit = append([]byte("\\"), lit...)
	}
	r.lastNormalText = normalText
	if r.listDepth > 0 && string(lit) == "\n" {
		// TODO: See if this can be cleaned up... It's needed for lists.
		return
	}
	cleanString := cleanWithoutTrim(string(lit))
	if cleanString == "" {
		return
	}
	// Skip first space if last character is already a space (i.e., no need for a 2nd space in a row).
	if r.skipSpaceIfNeededNormalText(w, cleanString) {
		cleanString = cleanString[1:]
	}
	r.outs(w, cleanString)
	// If it ends with a space, make note of that.
	if len(cleanString) >= 1 && cleanString[len(cleanString)-1] == ' ' {
		// TODO: write equivalent of this
		// r.normalTextMarker[out] = out.Len()
	}
}

// RenderHeader renders a header
func (*Renderer) RenderHeader(w io.Writer, ast ast.Node) {}

// RenderFooter renders a footer
func (*Renderer) RenderFooter(w io.Writer, ast ast.Node) {}

/*

func (r *Renderer) Table(out *bytes.Buffer, header []byte, body []byte, columnData []int) {
	doubleSpace(out)
	for column, cell := range r.headers {
		out.WriteByte('|')
		out.WriteByte(' ')
		r.outs(w, cell)
		for i := r.stringWidth(cell); i < r.columnWidths[column]; i++ {
			out.WriteByte(' ')
		}
		out.WriteByte(' ')
	}
	r.outs(w, "|\n")
	for column, width := range r.columnWidths {
		out.WriteByte('|')
		if r.columnAligns[column]&blackfriday.TABLE_ALIGNMENT_LEFT != 0 {
			out.WriteByte(':')
		} else {
			out.WriteByte('-')
		}
		for ; width > 0; width-- {
			out.WriteByte('-')
		}
		if r.columnAligns[column]&blackfriday.TABLE_ALIGNMENT_RIGHT != 0 {
			out.WriteByte(':')
		} else {
			out.WriteByte('-')
		}
	}
	r.outs(w, "|\n")
	for i := 0; i < len(r.cells); {
		for column := range r.headers {
			cell := []byte(r.cells[i])
			i++
			out.WriteByte('|')
			out.WriteByte(' ')
			switch r.columnAligns[column] {
			default:
				fallthrough
			case blackfriday.TABLE_ALIGNMENT_LEFT:
				r.out(w, cell)
				for i := r.stringWidth(string(cell)); i < r.columnWidths[column]; i++ {
					out.WriteByte(' ')
				}
			case blackfriday.TABLE_ALIGNMENT_CENTER:
				spaces := r.columnWidths[column] - r.stringWidth(string(cell))
				for i := 0; i < spaces/2; i++ {
					out.WriteByte(' ')
				}
				r.out(w, cell)
				for i := 0; i < spaces-(spaces/2); i++ {
					out.WriteByte(' ')
				}
			case blackfriday.TABLE_ALIGNMENT_RIGHT:
				for i := r.stringWidth(string(cell)); i < r.columnWidths[column]; i++ {
					out.WriteByte(' ')
				}
				r.out(w, cell)
			}
			out.WriteByte(' ')
		}
		r.outs(w, "|\n")
	}

	r.headers = nil
	r.columnAligns = nil
	r.columnWidths = nil
	r.cells = nil
}
func (_ *Renderer) TableRow(out *bytes.Buffer, text []byte) {
}
func (r *Renderer) TableHeaderCell(out *bytes.Buffer, text []byte, align int) {
	r.columnAligns = append(r.columnAligns, align)
	columnWidth := r.stringWidth(string(text))
	r.columnWidths = append(r.columnWidths, columnWidth)
	r.headers = append(r.headers, string(text))
}
func (r *Renderer) TableCell(out *bytes.Buffer, text []byte, align int) {
	columnWidth := r.stringWidth(string(text))
	column := len(r.cells) % len(r.headers)
	if columnWidth > r.columnWidths[column] {
		r.columnWidths[column] = columnWidth
	}
	r.cells = append(r.cells, string(text))
}

// Span-level callbacks.
func (_ *Renderer) AutoLink(out *bytes.Buffer, link []byte, kind int) {
	r.out(w, escape(link))
}
*/

// cleanWithoutTrim is like clean, but doesn't trim blanks.
func cleanWithoutTrim(s string) string {
	var b []byte
	var p byte
	for i := 0; i < len(s); i++ {
		q := s[i]
		if q == '\n' || q == '\r' || q == '\t' {
			q = ' '
		}
		if q != ' ' || p != ' ' {
			b = append(b, q)
			p = q
		}
	}
	return string(b)
}

// escape replaces instances of backslash with escaped backslash in text.
func escape(text []byte) []byte {
	return bytes.Replace(text, []byte(`\`), []byte(`\\`), -1)
}

func isNumber(data []byte) bool {
	for _, b := range data {
		if b < '0' || b > '9' {
			return false
		}
	}
	return true
}

func needsEscaping(text []byte, lastNormalText string) bool {
	switch string(text) {
	case `\`,
		"`",
		"*",
		"_",
		"{", "}",
		"[", "]",
		"(", ")",
		"#",
		"+",
		"-":
		return true
	case "!":
		return false
	case ".":
		// Return true if number, because a period after a number must be escaped to not get parsed as an ordered list.
		return isNumber([]byte(lastNormalText))
	case "<", ">":
		return true
	default:
		return false
	}
}

// terminalStringWidth returns width of s, taking into account possible ANSI escape codes
// (which don't count towards string width).
func terminalStringWidth(s string) (width int) {
	width = runewidth.StringWidth(s)
	width -= strings.Count(s, "\x1b[1m") * len("[1m") // HACK, TODO: Find a better way of doing this.
	width -= strings.Count(s, "\x1b[0m") * len("[0m") // HACK, TODO: Find a better way of doing this.
	return width
}

// Process formats Markdown.
// If opt is nil the defaults are used.
// Error can only occur when reading input from filename rather than src.
func Process(filename string, src []byte, opts *Options) ([]byte, error) {
	// Get source.
	input, err := readSource(filename, src)
	if err != nil {
		return nil, err
	}

	// extensions for GitHub Flavored Markdown-like parsing.
	const extensions = parser.NoIntraEmphasis |
		parser.Tables |
		parser.FencedCode |
		parser.Autolink |
		parser.Strikethrough |
		parser.SpaceHeadings |
		parser.NoEmptyLineBeforeBlock

	p := parser.NewWithExtensions(extensions)
	doc := p.Parse(input)
	r := NewRenderer(opts)
	output := md.Render(doc, r)
	return output, nil
}

// If src != nil, readSource returns src.
// If src == nil, readSource returns the result of reading the file specified by filename.
func readSource(filename string, src []byte) ([]byte, error) {
	if src != nil {
		return src, nil
	}
	return ioutil.ReadFile(filename)
}
