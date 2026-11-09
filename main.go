package main

import (
	"encoding/csv"
	"flag"
	"fmt"
	"io"
	"os"
	"sort"
	"strconv"
	"strings"
	"unicode"
	"unicode/utf8"
)

// version はビルド時に ldflags で設定される。
// 例: go build -ldflags "-X main.version=$(date +%Y%m%d-%H%M%S)"
var version string

// tableData は「完成した1つの表」を構成パーツに分解して保持する。
// headerLines : 各ページの先頭に必ず付く行
//   - md:    ヘッダー行＋区切り行
//   - text:  上罫線＋ヘッダー行＋ヘッダー罫線
//
// units : データ行1件分の行グループ（text ではセル内改行で複数行になりうる）
// inlineBorder : text のデータ行間罫線（ユニットの末尾に付与する）
// borderLast   : text の下罫線（最終ユニットの罫線をこれに差し替える）
//
// 列幅は全データ行から事前計算して共有するため、どのページでも幅は同一になる。
type tableData struct {
	headerLines  []string
	units        [][]string
	inlineBorder string
	borderLast   string
	codeBlock    bool
}

// page は 1 ページ分に割り当てられたデータユニットのインデックス集合
type page struct {
	unitIdx []int
}

// sizedUnit はデータユニットとそのバイト数
type sizedUnit struct {
	lines []string
	size  int64
}

func main() {
	// コマンドライン引数の定義
	showVersion := flag.Bool("version", false, "print version and exit")
	lbReplacer := flag.String("line-break", "", "string to replace newlines inside CSV cells. default is <br> for md and no conversion for text")
	topBorder := flag.String("top-border", "-", "single char for top border line (text only)")
	bottomBorder := flag.String("bottom-border", "-", "single char for bottom border line (text only)")
	leftBorder := flag.String("left-border", "|", "single char for left border line (text only)")
	rightBorder := flag.String("right-border", "|", "single char for right border line (text only)")
	headerBorder := flag.String("header-border", "=", "single char for header/data separator line (text only)")
	cornerChar := flag.String("corner-char", "+", "single char for corners and junction points (text only)")
	inlineHBorder := flag.String("inline-h-border", "-", "single char for inline horizontal borders (text only). empty string disables inline borders")
	inlineVBorder := flag.String("inline-v-border", "|", "single char for inline vertical borders (text only)")
	cellMaxWidth := flag.String("cell-max-width", "0", "maximum display width per cell row. single value applies to all columns, or comma-separated list per column (e.g. 5,6,7). 0 for unlimited, minimum 3")
	mdCodeBlock := flag.Bool("md-code-block", false, "wrap each MD cell content in code blocks")
	tableMaxByte := flag.Int64("table-max-byte", 0, "maximum byte size per page of table output (0 for unlimited)")
	tableMaxLine := flag.Int("table-max-line", 0, "maximum number of data rows (excluding header/borders) per page (0 for unlimited)")
	pageBreak := flag.String("page-break", "<!-- page %page/%total -->", "page separator template between pages. use %page and %total as placeholders. empty string disables separator")
	output := flag.String("output", "text", "output format: md or text")
	flag.StringVar(output, "o", "text", "output format: md or text (shorthand)")
	sortCol := flag.Int("sort", 0, "sort data rows by 1-based column number (0 to disable)")
	header := flag.String("header", "", "CSV string to use as table header. when set, the first row of input data is treated as data, not header")
	filePath := flag.String("file", "", "path to CSV file (default: read from stdin)")
	flag.StringVar(filePath, "f", "", "path to CSV file (shorthand)")
	flag.Parse()

	if *showVersion {
		v := version
		if v == "" {
			v = "dev"
		}
		fmt.Println(v)
		return
	}

	// 半角 1 文字に制限（空文字は許可するオプションあり）
	checkSingleChar("--top-border", *topBorder)
	checkSingleChar("--bottom-border", *bottomBorder)
	checkSingleChar("--left-border", *leftBorder)
	checkSingleChar("--right-border", *rightBorder)
	checkSingleChar("--header-border", *headerBorder)
	checkSingleChar("--corner-char", *cornerChar)
	checkSingleCharOrEmpty("--inline-h-border", *inlineHBorder)
	checkSingleChar("--inline-v-border", *inlineVBorder)

	// --table-max-line の検証
	if *tableMaxLine < 0 {
		fmt.Fprintln(os.Stderr, "Error: --table-max-line must be >= 0 (0 for unlimited)")
		os.Exit(1)
	}

	// 入力ソースの決定
	var input io.Reader = os.Stdin
	if *filePath != "" {
		f, err := os.Open(*filePath)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error opening file: %v\n", err)
			os.Exit(1)
		}
		defer f.Close()
		input = f
	}

	reader := csv.NewReader(input)
	reader.FieldsPerRecord = -1

	var rows [][]string
	maxCols := 0

	for {
		record, err := reader.Read()
		if err == io.EOF {
			break
		}
		if err != nil {
			fmt.Fprintln(os.Stderr, "Error reading CSV:", err)
			os.Exit(1)
		}

		rows = append(rows, record)
		if len(record) > maxCols {
			maxCols = len(record)
		}
	}

	// --header が指定されていれば先頭に追加し、入力1行目はデータ扱いにする
	if *header != "" {
		headerRow, err := parseCSVLine(*header)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error parsing --header: %v\n", err)
			os.Exit(1)
		}
		if len(headerRow) > maxCols {
			maxCols = len(headerRow)
		}
		rows = append([][]string{headerRow}, rows...)
	}

	if len(rows) == 0 {
		return
	}

	// 全ての行のカラム数を最大値（maxCols）に揃える
	for i := range rows {
		for len(rows[i]) < maxCols {
			rows[i] = append(rows[i], "")
		}
	}

	// --cell-max-width をパース・検証
	cellMaxWidths, err := parseCellMaxWidths(*cellMaxWidth, maxCols)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: invalid --cell-max-width: %v\n", err)
		os.Exit(1)
	}
	for i, w := range cellMaxWidths {
		if w != 0 && w < 3 {
			fmt.Fprintf(os.Stderr, "Error: --cell-max-width column %d must be 0 (unlimited) or >= 3\n", i+1)
			os.Exit(1)
		}
	}

	// --sort が指定されていればデータ行をソート（1始まり）
	if *sortCol > 0 {
		idx := *sortCol - 1
		if idx >= maxCols {
			fmt.Fprintf(os.Stderr, "Error: --sort column %d is out of range (max %d)\n", *sortCol, maxCols)
			os.Exit(1)
		}
		if len(rows) > 1 {
			sort.SliceStable(rows[1:], func(i, j int) bool {
				return rows[1+i][idx] < rows[1+j][idx]
			})
		}
	}

	// フォーマットに応じて表データを組み立て、出力
	outFormat := strings.ToLower(*output)

	// --line-break が指定されていなければ、出力形式に応じたデフォルト値を使う
	// md: セル内改行を <br> に変換（Markdown テーブルでの複数行表示用）
	// text: 変換せず、セル内改行をそのまま複数物理行として出力
	lineBreak := *lbReplacer
	if !isFlagSet("line-break") {
		if outFormat == "md" {
			lineBreak = "<br>"
		} else {
			lineBreak = ""
		}
	}

	switch outFormat {
	case "text":
		t := buildTextTable(rows, maxCols, cellMaxWidths,
			*topBorder, *bottomBorder, *leftBorder, *rightBorder, *headerBorder,
			*cornerChar, *inlineHBorder, *inlineVBorder, lineBreak)
		printTable(t, *tableMaxByte, *tableMaxLine, *pageBreak)
	default: // md
		t := buildMarkdownTable(rows, maxCols, lineBreak, cellMaxWidths, *mdCodeBlock)
		printTable(t, *tableMaxByte, *tableMaxLine, *pageBreak)
	}
}

// isFlagSet は指定した名前のコマンドライン引数が明示的に渡されたかを返す。
func isFlagSet(name string) bool {
	set := false
	flag.Visit(func(f *flag.Flag) {
		if f.Name == name {
			set = true
		}
	})
	return set
}

func checkSingleChar(name, value string) {
	if len(value) != 1 {
		fmt.Fprintf(os.Stderr, "Error: %s must be a single character\n", name)
		os.Exit(1)
	}
}

func checkSingleCharOrEmpty(name, value string) {
	if value != "" && len(value) != 1 {
		fmt.Fprintf(os.Stderr, "Error: %s must be a single character or empty\n", name)
		os.Exit(1)
	}
}

// parseCSVLine は与えられた文字列を 1 行の CSV レコードとしてパースする。
func parseCSVLine(s string) ([]string, error) {
	r := csv.NewReader(strings.NewReader(s))
	r.FieldsPerRecord = -1
	record, err := r.Read()
	if err != nil {
		return nil, err
	}
	return record, nil
}

// parseCellMaxWidths は --cell-max-width の文字列を列数に応じてパースする。
// 単一の値の場合は全列に適用し、カンマ区切りの場合は各列に適用する。0 は未制限を表す。
func parseCellMaxWidths(s string, colCount int) ([]int, error) {
	widths := make([]int, colCount)
	if s == "" {
		return widths, nil
	}
	parts := strings.Split(s, ",")
	if len(parts) == 1 {
		w, err := strconv.Atoi(strings.TrimSpace(parts[0]))
		if err != nil {
			return nil, err
		}
		for i := range widths {
			widths[i] = w
		}
		return widths, nil
	}
	if len(parts) != colCount {
		return nil, fmt.Errorf("expected 1 or %d values, got %d", colCount, len(parts))
	}
	for i, p := range parts {
		w, err := strconv.Atoi(strings.TrimSpace(p))
		if err != nil {
			return nil, err
		}
		widths[i] = w
	}
	return widths, nil
}

// ---- 表データの組み立て ----

// Markdown 形式の表データを組み立てる
func buildMarkdownTable(rows [][]string, maxCols int, lineBreak string, cellMaxWidths []int, mdCodeBlock bool) tableData {
	processedRows := make([][]string, len(rows))
	for r, row := range rows {
		processedRow := make([]string, len(row))
		for c, cell := range row {
			s := cell
			s = strings.ReplaceAll(s, "\r\n", lineBreak)
			s = strings.ReplaceAll(s, "\r", lineBreak)
			s = strings.ReplaceAll(s, "\n", lineBreak)
			// 長さ制限を適用（``` は含めない）
			maxW := cellMaxWidths[c]
			if maxW > 0 && getDisplayWidth(s) > maxW {
				s = truncateWithEllipsis(s, maxW)
			}
			// コードブロックで囲む
			if mdCodeBlock {
				s = "```" + s + "```"
			}
			processedRow[c] = s
		}
		processedRows[r] = processedRow
	}

	var t tableData
	// ヘッダー行＋区切り行
	t.headerLines = []string{
		"| " + strings.Join(processedRows[0], " | ") + " |",
		"| " + strings.Join(repeatStr("---", maxCols), " | ") + " |",
	}
	// 各データ行は 1 行ユニット
	for _, row := range processedRows[1:] {
		t.units = append(t.units, []string{"| " + strings.Join(row, " | ") + " |"})
	}
	return t
}

// Text 形式の表データを組み立てる
func buildTextTable(rows [][]string, maxCols int, cellMaxWidths []int, top, bottom, left, right, header, corner, inlineH, inlineV, lineBreak string) tableData {
	// 各セルを「論理行」に分解
	// lineBreak が指定されていればセル内改行を置き換えて 1 行にまとめる。
	// lineBreak が空なら元の改行で分割する。
	cellLines := make([][][]string, len(rows))
	for r, row := range rows {
		cellLines[r] = make([][]string, maxCols)
		for c, cell := range row {
			if lineBreak != "" {
				normalized := strings.ReplaceAll(cell, "\r\n", lineBreak)
				normalized = strings.ReplaceAll(normalized, "\r", lineBreak)
				normalized = strings.ReplaceAll(normalized, "\n", lineBreak)
				cellLines[r][c] = []string{normalized}
			} else {
				normalized := strings.ReplaceAll(cell, "\r\n", "\n")
				normalized = strings.ReplaceAll(normalized, "\r", "\n")
				cellLines[r][c] = strings.Split(normalized, "\n")
			}
		}
	}

	// 各論理行に cellMaxWidth 制限を適用
	for _, row := range cellLines {
		for c := range row {
			maxW := cellMaxWidths[c]
			if maxW > 0 {
				for l := range row[c] {
					if getDisplayWidth(row[c][l]) > maxW {
						row[c][l] = truncateWithEllipsis(row[c][l], maxW)
					}
				}
			}
		}
	}

	// 各カラムの最大文字幅を計算（全データ行から算出するため、どのページでも同じ幅になる）
	colWidths := make([]int, maxCols)
	for _, row := range cellLines {
		for c, lines := range row {
			maxW := cellMaxWidths[c]
			for _, line := range lines {
				w := getDisplayWidth(line)
				if maxW > 0 && w > maxW {
					w = maxW
				}
				if w > colWidths[c] {
					colWidths[c] = w
				}
			}
		}
	}

	var t tableData
	t.inlineBorder = generateBorderRow(colWidths, inlineH, corner)
	t.borderLast = generateBorderRow(colWidths, bottom, corner)

	// ヘッダーブロック: 上罫線＋ヘッダー行＋ヘッダー罫線
	// （ヘッダーセル内に改行があれば複数物理行になるためすべて追加する）
	t.headerLines = []string{
		generateBorderRow(colWidths, top, corner),
	}
	t.headerLines = append(t.headerLines, buildRowLines(cellLines[0], colWidths, inlineV, left, right)...)
	t.headerLines = append(t.headerLines, generateBorderRow(colWidths, header, corner))

	// データユニット: 行そのものの物理行のリスト
	// （行間罫線・下罫線は出力時に動的に付与する）
	for _, row := range cellLines[1:] {
		t.units = append(t.units, buildRowLines(row, colWidths, inlineV, left, right))
	}
	return t
}

// ---- 出力（ページ分割を含む）----

// 行バイト数（改行込み）を計算
func lineBytes(line string) int64 {
	return int64(len(line) + 1) // +1 は "\n"
}

// ユニット（行グループ＋行間罫線1行）のバイト数
func unitBytes(unit []string, withBorder bool, border string) int64 {
	var total int64
	for _, line := range unit {
		total += lineBytes(line)
	}
	if withBorder {
		total += lineBytes(border)
	}
	return total
}

// ヘッダーブロックのバイト数
func headerBytes(t tableData) int64 {
	var total int64
	for _, line := range t.headerLines {
		total += lineBytes(line)
	}
	return total
}

// pageBreakLine はページ区切り行の文字列を組み立てて返す。
// tmpl が空文字列の場合は空文字列を返す。
// %page はページ番号、%total は総ページ数に置き換えられる。
func pageBreakLine(tmpl string, pageNum, totalPages int) string {
	if tmpl == "" {
		return ""
	}
	s := strings.ReplaceAll(tmpl, "%page", fmt.Sprintf("%d", pageNum))
	s = strings.ReplaceAll(s, "%total", fmt.Sprintf("%d", totalPages))
	return s
}

// pageBreakBytes はページ区切り行1行分のバイト数（改行込み）を返す。
func pageBreakBytes(tmpl string, pageNum, totalPages int) int64 {
	line := pageBreakLine(tmpl, pageNum, totalPages)
	return lineBytes(line)
}

// printTable は、maxBytes / maxLines を超えないように表をページ分割して出力する。
// 2 ページ目以降もヘッダーを付け、各ページは必ず完結した表になる。
func printTable(t tableData, maxBytes int64, maxLines int, pageBreak string) {
	// 各ユニットは「データ行＋行間罫線」で構成されるとみなす。
	// 最終ユニットの罫線だけ下罫線文字に差し替える（長さは同一なのでバイト数に影響しない）。
	// inlineBorder が空でも borderLast は表示するため、サイズ計算用の罫線は fallback する。
	borderForSizing := t.inlineBorder
	if borderForSizing == "" {
		borderForSizing = t.borderLast
	}
	withBorder := borderForSizing != ""

	units := make([]sizedUnit, 0, len(t.units))
	for _, u := range t.units {
		size := unitBytes(u, withBorder, borderForSizing)
		units = append(units, sizedUnit{lines: u, size: size})
	}

	// 固定コストの計算
	fenceCost := int64(0)
	if t.codeBlock {
		fenceCost = lineBytes("```") * 2 // 開始＋終了
	}
	hSize := headerBytes(t)

	// ページ区切り行のバイト数は総ページ数（%total の桁数など）に依存し、
	// 総ページ数は区切りのバイト数にも依存する（循環参照）。
	// そのため、まず概算値でページング → 確定した総ページ数から実際の区切りサイズを
	// 計算 → 再ページング、を 2〜3 回繰り返して収束させる。
	reserve := pageBreakBytes(pageBreak, 1, 1) // 初期概算
	var pages []page
	for attempt := 0; attempt < 3; attempt++ {
		pages = paginate(units, hSize, fenceCost, maxBytes, reserve, maxLines)
		if len(pages) <= 1 {
			break
		}
		// 全ページ中で最も長くなる区切り（ページ番号＝総ページ数）を基準に再計算
		longest := pageBreakBytes(pageBreak, len(pages), len(pages))
		if longest == reserve {
			break
		}
		reserve = longest
	}

	// 出力
	totalPages := len(pages)
	for pi, p := range pages {
		if pi > 0 {
			fmt.Println(pageBreakLine(pageBreak, pi+1, totalPages))
		}
		var out []string
		if t.codeBlock {
			out = append(out, "```")
		}
		out = append(out, t.headerLines...)

		for k, ui := range p.unitIdx {
			u := units[ui]
			out = append(out, u.lines...)
			if k == len(p.unitIdx)-1 {
				if t.borderLast != "" {
					// 最終ユニットの罫線は下罫線
					out = append(out, t.borderLast)
				}
			} else {
				if t.inlineBorder != "" {
					out = append(out, t.inlineBorder)
				}
			}
		}

		if t.codeBlock {
			out = append(out, "```")
		}
		for _, line := range out {
			fmt.Println(line)
		}
	}
}

// paginate は、maxBytes または maxLines を超えないようにデータユニットをページへ割り振る。
// reserve はページ区切り行 1 行分の見込みバイト数。
func paginate(units []sizedUnit, hSize, fenceCost, maxBytes, reserve int64, maxLines int) []page {
	var pages []page

	if maxBytes <= 0 && maxLines <= 0 {
		// 制限なし: 1 ページに全部
		p := page{}
		for i := range units {
			p.unitIdx = append(p.unitIdx, i)
		}
		pages = append(pages, p)
		return pages
	}

	var cur []int // 現在のページに割り当てたユニットのインデックス
	var curSize int64
	var curLines int
	flush := func() {
		if len(cur) > 0 {
			pages = append(pages, page{unitIdx: cur})
			cur = nil
			curSize = 0
			curLines = 0
		}
	}

	for i, u := range units {
		needFlush := false

		// maxBytes 制限がある場合
		if maxBytes > 0 {
			// 区切り行が挟まる可能性を見込んで余裕を確保する
			avail := maxBytes - reserve

			// ヘッダーが入らないほど小さい maxBytes は無意味なのでエラーにする
			if avail < hSize+fenceCost {
				fmt.Fprintf(os.Stderr, "Error: --table-max-byte is too small to hold the table header (%d bytes required)\n", hSize+fenceCost)
				os.Exit(1)
			}

			if curSize+u.size > avail-hSize && len(cur) > 0 {
				needFlush = true
			}
		}

		// maxLines 制限がある場合
		if maxLines > 0 && curLines >= maxLines && len(cur) > 0 {
			needFlush = true
		}

		if needFlush {
			flush()
		}

		cur = append(cur, i)
		curSize += u.size
		curLines++

		// ヘッダー＋1 ユニットすら入らない場合は、1 ユニットでページを強制成立させる
		if maxBytes > 0 {
			avail := maxBytes - reserve
			if curSize > avail-hSize {
				flush()
			}
		}

		if maxLines > 0 && curLines >= maxLines {
			flush()
		}
	}
	flush()

	return pages
}

// ---- 行組み立てヘルパ ----

// 1 レコード分の物理行リストを組み立てる
func buildRowLines(cells [][]string, colWidths []int, inlineVChar, leftChar, rightChar string) []string {
	wrapped := make([][]string, len(cells))
	maxLines := 0

	for c, lines := range cells {
		var phys []string
		for _, line := range lines {
			phys = append(phys, wrapLine(line, colWidths[c])...)
		}
		if len(phys) == 0 {
			phys = []string{""}
		}
		wrapped[c] = phys
		if len(phys) > maxLines {
			maxLines = len(phys)
		}
	}

	var out []string
	for i := 0; i < maxLines; i++ {
		parts := make([]string, len(cells))
		for c := range cells {
			s := ""
			if i < len(wrapped[c]) {
				s = wrapped[c][i]
			}
			parts[c] = padRight(s, colWidths[c])
		}
		out = append(out, leftChar+" "+strings.Join(parts, " "+inlineVChar+" ")+" "+rightChar)
	}
	return out
}

func repeatStr(s string, n int) []string {
	out := make([]string, n)
	for i := range out {
		out[i] = s
	}
	return out
}

// 指定幅まで切り捨てて末尾を "..." に置き換える
func truncateWithEllipsis(s string, maxWidth int) string {
	if maxWidth <= 3 {
		return "..."
	}

	width := 0
	endIndex := 0

	for i, r := range s {
		w := runeWidth(r)
		if width+w > maxWidth-3 {
			break
		}
		width += w
		endIndex = i + utf8.RuneLen(r)
	}

	if endIndex == 0 {
		return "..."
	}

	return s[:endIndex] + "..."
}

// 横ボーダー行の生成 (+---+---+)
func generateBorderRow(colWidths []int, horizChar, cornerChar string) string {
	if horizChar == "" {
		return ""
	}
	if len(colWidths) == 0 {
		return cornerChar + cornerChar
	}

	var parts []string
	for _, w := range colWidths {
		parts = append(parts, strings.Repeat(horizChar, w+2))
	}
	return cornerChar + strings.Join(parts, cornerChar) + cornerChar
}

// 論理行が列幅を超える場合の保険として、表示幅で折り返す
func wrapLine(s string, maxWidth int) []string {
	if maxWidth <= 0 {
		return []string{s}
	}

	var out []string
	var cur []rune
	curWidth := 0

	for _, r := range s {
		w := runeWidth(r)
		if curWidth+w > maxWidth && len(cur) > 0 {
			out = append(out, string(cur))
			cur = nil
			curWidth = 0
		}
		cur = append(cur, r)
		curWidth += w
	}
	out = append(out, string(cur))

	return out
}

// 表示幅を基準に右側を半角スペースで埋める
func padRight(s string, width int) string {
	padding := width - getDisplayWidth(s)
	if padding < 0 {
		padding = 0
	}
	return s + strings.Repeat(" ", padding)
}

// rune ごとの幅を取得
func runeWidth(r rune) int {
	if unicode.Is(unicode.Han, r) ||
		unicode.Is(unicode.Hiragana, r) ||
		unicode.Is(unicode.Katakana, r) {
		return 2
	}

	if utf8.RuneLen(r) >= 3 {
		return 2
	}

	return 1
}

// ラインの文字幅を取得
func getRuneWidth(chars []rune) int {
	width := 0
	for _, ch := range chars {
		width += runeWidth(ch)
	}
	return width
}

// 文字列の表示幅を計算
func getDisplayWidth(s string) int {
	return getRuneWidth([]rune(s))
}
