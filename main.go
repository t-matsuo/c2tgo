package main

import (
	"encoding/csv"
	"flag"
	"fmt"
	"io"
	"os"
	"strings"
	"unicode"
	"unicode/utf8"
)

// tableData は「完成した1つの表」を構成パーツに分解して保持する。
// headerLines : 各ページの先頭に必ず付く行
//   - md:          ヘッダー行＋区切り行
//   - grid/slack:  上罫線＋ヘッダー行＋ヘッダー罫線
//
// units : データ行1件分の行グループ（grid/slack ではセル内改行で複数行になりうる）
// inlineBorder : grid/slack のデータ行間罫線（ユニットの末尾に付与する）
// borderLast   : grid/slack の下罫線（最終ユニットの罫線をこれに差し替える）
// codeBlock    : slack のようにページ全体を ``` で囲むか
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
	lbReplacer := flag.String("line-break", "<br>", "string to replace newlines inside CSV cells (md format only)")
	topBorder := flag.String("top-border", "-", "single char for top border line (grid/slack only)")
	bottomBorder := flag.String("bottom-border", "-", "single char for bottom border line (grid/slack only)")
	leftBorder := flag.String("left-border", "|", "single char for left border line (grid/slack only)")
	rightBorder := flag.String("right-border", "|", "single char for right border line (grid/slack only)")
	headerBorder := flag.String("header-border", "=", "single char for header/data separator line (grid/slack only)")
	cornerChar := flag.String("corner-char", "+", "single char for corners and junction points (grid/slack only)")
	inlineHBorder := flag.String("inline-h-border", "-", "single char for inline horizontal borders (grid/slack only)")
	inlineVBorder := flag.String("inline-v-border", "|", "single char for inline vertical borders (grid/slack only)")
	cellMaxWidth := flag.Int("cell-max-width", 0, "maximum display width per cell row (0 for unlimited, minimum 3)")
	mdCodeBlock := flag.Bool("md-code-block", false, "wrap each MD cell content in code blocks")
	tableMaxByte := flag.Int64("table-max-byte", 0, "maximum byte size per page of table output (0 for unlimited)")
	tableMaxLine := flag.Int("table-max-line", 0, "maximum number of data rows (excluding header/borders) per page (0 for unlimited)")
	pageBreak := flag.String("page-break", "<!-- page %page/%total -->", "page separator template between pages. use %page and %total as placeholders. empty string disables separator")
	format := flag.String("format", "md", "output format: md, grid, slack")
	flag.Parse()

	// 半角 1 文字に制限
	checkSingleChar("--top-border", *topBorder)
	checkSingleChar("--bottom-border", *bottomBorder)
	checkSingleChar("--left-border", *leftBorder)
	checkSingleChar("--right-border", *rightBorder)
	checkSingleChar("--header-border", *headerBorder)
	checkSingleChar("--corner-char", *cornerChar)
	checkSingleChar("--inline-h-border", *inlineHBorder)
	checkSingleChar("--inline-v-border", *inlineVBorder)

	// --cell-max-width の検証
	if *cellMaxWidth != 0 && *cellMaxWidth < 3 {
		fmt.Fprintln(os.Stderr, "Error: --cell-max-width must be 0 (unlimited) or >= 3")
		os.Exit(1)
	}

	// --table-max-line の検証
	if *tableMaxLine < 0 {
		fmt.Fprintln(os.Stderr, "Error: --table-max-line must be >= 0 (0 for unlimited)")
		os.Exit(1)
	}

	reader := csv.NewReader(os.Stdin)
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

	if len(rows) == 0 {
		return
	}

	// 全ての行のカラム数を最大値（maxCols）に揃える
	for i := range rows {
		for len(rows[i]) < maxCols {
			rows[i] = append(rows[i], "")
		}
	}

	// フォーマットに応じて表データを組み立て、出力
	switch strings.ToLower(*format) {
	case "grid":
		t := buildGridSlackTable(rows, maxCols, *cellMaxWidth,
			*topBorder, *bottomBorder, *leftBorder, *rightBorder, *headerBorder,
			*cornerChar, *inlineHBorder, *inlineVBorder, false)
		printTable(t, *tableMaxByte, *tableMaxLine, *pageBreak)
	case "slack":
		t := buildGridSlackTable(rows, maxCols, *cellMaxWidth,
			*topBorder, *bottomBorder, *leftBorder, *rightBorder, *headerBorder,
			*cornerChar, *inlineHBorder, *inlineVBorder, true)
		printTable(t, *tableMaxByte, *tableMaxLine, *pageBreak)
	default: // md
		t := buildMarkdownTable(rows, maxCols, *lbReplacer, *cellMaxWidth, *mdCodeBlock)
		printTable(t, *tableMaxByte, *tableMaxLine, *pageBreak)
	}
}

func checkSingleChar(name, value string) {
	if len(value) != 1 {
		fmt.Fprintf(os.Stderr, "Error: %s must be a single character\n", name)
		os.Exit(1)
	}
}

// ---- 表データの組み立て ----

// Markdown 形式の表データを組み立てる
func buildMarkdownTable(rows [][]string, maxCols int, lineBreak string, cellMaxWidth int, mdCodeBlock bool) tableData {
	processedRows := make([][]string, len(rows))
	for r, row := range rows {
		processedRow := make([]string, len(row))
		for c, cell := range row {
			s := cell
			s = strings.ReplaceAll(s, "\r\n", lineBreak)
			s = strings.ReplaceAll(s, "\r", lineBreak)
			s = strings.ReplaceAll(s, "\n", lineBreak)
			// 長さ制限を適用（``` は含めない）
			if cellMaxWidth > 0 && getDisplayWidth(s) > cellMaxWidth {
				s = truncateWithEllipsis(s, cellMaxWidth)
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

// Grid / Slack 形式の表データを組み立てる
func buildGridSlackTable(rows [][]string, maxCols int, cellMaxWidth int, top, bottom, left, right, header, corner, inlineH, inlineV string, isSlack bool) tableData {
	// 各セルを「論理行」に分解（セル内改行 = 論理行の区切り）
	cellLines := make([][][]string, len(rows))
	for r, row := range rows {
		cellLines[r] = make([][]string, maxCols)
		for c, cell := range row {
			normalized := strings.ReplaceAll(cell, "\r\n", "\n")
			normalized = strings.ReplaceAll(normalized, "\r", "\n")
			cellLines[r][c] = strings.Split(normalized, "\n")
		}
	}

	// 各論理行に cellMaxWidth 制限を適用
	if cellMaxWidth > 0 {
		for _, row := range cellLines {
			for c := range row {
				for l := range row[c] {
					if getDisplayWidth(row[c][l]) > cellMaxWidth {
						row[c][l] = truncateWithEllipsis(row[c][l], cellMaxWidth)
					}
				}
			}
		}
	}

	// 各カラムの最大文字幅を計算（全データ行から算出するため、どのページでも同じ幅になる）
	colWidths := make([]int, maxCols)
	for _, row := range cellLines {
		for c, lines := range row {
			for _, line := range lines {
				if w := getDisplayWidth(line); w > colWidths[c] {
					colWidths[c] = w
				}
			}
		}
	}

	var t tableData
	t.codeBlock = isSlack
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
	withBorder := t.inlineBorder != ""

	units := make([]sizedUnit, 0, len(t.units))
	for _, u := range t.units {
		size := unitBytes(u, withBorder, t.inlineBorder)
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
			if withBorder {
				if k == len(p.unitIdx)-1 {
					// 最終ユニットの罫線は下罫線
					out = append(out, t.borderLast)
				} else {
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
