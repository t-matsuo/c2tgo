package main

import (
	"encoding/csv"
	"flag"
	"fmt"
	"io"
	"os"
	"reflect"
	"runtime"
	"sort"
	"strconv"
	"strings"
	"unicode"
	"unicode/utf8"
)

// version はビルド時に ldflags で設定される。
// 例: go build -ldflags "-X main.version=$(date +%Y%m%d-%H%M%S)"
var version string

// commit はビルド時に ldflags で設定される。
// 例: go build -ldflags "-X main.commit=$(git rev-parse HEAD)"
var commit string

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

// sizedUnit はデータユニットとそのバイト数・表示行数
type sizedUnit struct {
	lines     []string
	size      int64
	lineCount int
}

// cmdFlags は c2tgo のコマンドラインフラグを保持する FlagSet。
// 登録順を維持することで、ヘルプ表示で関連オプションを近くに配置する。
var cmdFlags *flag.FlagSet

// flagOrder はフラグを登録した順序を保持する。
// 標準の flag.FlagSet は登録順を保証しないため、ヘルプ出力時にこのスライスを使う。
var flagOrder []string

// printGroupedDefaults は、同じ変数に束縛されているフラグ（長い名前とショートハンド）を
// 1 行にまとめてヘルプを出力する。標準の flag.PrintDefaults と同じ見た目になるよう
// フォーマットする。
func printGroupedDefaults() {
	// Value が同一（同じポインタ変数に束縛されている）フラグをグループ化する。
	groups := make(map[flag.Value][]*flag.Flag)
	cmdFlags.VisitAll(func(f *flag.Flag) {
		groups[f.Value] = append(groups[f.Value], f)
	})

	// 出力順は flagOrder（登録順）で最初に現れたフラグを代表とする。
	seen := make(map[flag.Value]bool)
	for _, name := range flagOrder {
		rep := cmdFlags.Lookup(name)
		if rep == nil || seen[rep.Value] {
			continue
		}
		seen[rep.Value] = true
		group := groups[rep.Value]

		// 名前を長い順、同じ長さなら辞書順で並べる。
		names := make([]string, len(group))
		for i, f := range group {
			names[i] = f.Name
		}
		sort.Slice(names, func(i, j int) bool {
			if len(names[i]) != len(names[j]) {
				return len(names[i]) > len(names[j])
			}
			return names[i] < names[j]
		})

		// 説明文はグループ内で最も長い名前のフラグ（通常は長い名前の本体）から選ぶ。
		usage := pickUsage(group)

		// パラメータ名と説明文を抽出。
		tmp := &flag.Flag{Name: rep.Name, Usage: usage, Value: rep.Value, DefValue: rep.DefValue}
		argName, usage := flag.UnquoteUsage(tmp)

		var b strings.Builder
		fmt.Fprintf(&b, "  -%s", strings.Join(names, ", -"))
		if len(argName) > 0 {
			fmt.Fprintf(&b, " %s", argName)
		}

		// 名前が短い（1文字）場合は同じ行に説明を、長い場合は次の行に説明を書く。
		if b.Len() <= 4 {
			b.WriteString("\t")
		} else {
			b.WriteString("\n    \t")
		}
		b.WriteString(strings.ReplaceAll(usage, "\n", "\n    \t"))

		// デフォルト値が型のゼロ値でなければ出力する。
		if isZero, err := isZeroValue(rep, rep.DefValue); err == nil && !isZero {
			if reflect.TypeOf(rep.Value).String() == "*flag.stringValue" {
				fmt.Fprintf(&b, " (default %q)", rep.DefValue)
			} else {
				fmt.Fprintf(&b, " (default %v)", rep.DefValue)
			}
		}

		fmt.Fprint(cmdFlags.Output(), b.String(), "\n")
	}
}

// pickUsage はフラググループからヘルプに表示する説明文を選ぶ。
// 最も名前が長いフラグ（通常は長い名前の本体）の説明文を採用し、
// 念のため "(shorthand...)" マーカーが残っていれば取り除く。
func pickUsage(group []*flag.Flag) string {
	// 長い名前の方をメインの説明文として採用する（ショートハンドは 1 文字のため）。
	var best *flag.Flag
	for _, f := range group {
		if best == nil || len(f.Name) > len(best.Name) || (len(f.Name) == len(best.Name) && f.Name < best.Name) {
			best = f
		}
	}
	if best == nil {
		return ""
	}
	u := best.Usage
	// 念のため "(shorthand...)" マーカーが残っていれば取り除く。
	if i := strings.Index(u, " (shorthand"); i >= 0 {
		u = strings.TrimRight(u[:i], ", ")
	}
	return u
}

// isZeroValue は value がフラグ f の型のゼロ値の文字列表現かどうかを返す。
// 標準パッケージの同名関数と同じ動作をする。
func isZeroValue(f *flag.Flag, value string) (ok bool, err error) {
	typ := reflect.TypeOf(f.Value)
	var z reflect.Value
	if typ.Kind() == reflect.Pointer {
		z = reflect.New(typ.Elem())
	} else {
		z = reflect.Zero(typ)
	}
	defer func() {
		if e := recover(); e != nil {
			if typ.Kind() == reflect.Pointer {
				typ = typ.Elem()
			}
			err = fmt.Errorf("panic calling String method on zero %v for flag %s: %v", typ, f.Name, e)
		}
	}()
	return value == z.Interface().(flag.Value).String(), nil
}

// boolFlag は FlagSet に長い名前のフラグとショートハンドを同じ変数に束縛して登録する。
// 説明文は重複させず、ヘルプ出力は printGroupedDefaults で 1 行にまとめる。
func boolFlag(fs *flag.FlagSet, name, shorthand string, value bool, usage string) *bool {
	p := fs.Bool(name, value, usage)
	if shorthand != "" {
		fs.BoolVar(p, shorthand, value, usage)
	}
	flagOrder = append(flagOrder, name)
	return p
}

// stringFlag は boolFlag と同様に string 型のフラグとショートハンドを登録する。
func stringFlag(fs *flag.FlagSet, name, shorthand string, value string, usage string) *string {
	p := fs.String(name, value, usage)
	if shorthand != "" {
		fs.StringVar(p, shorthand, value, usage)
	}
	flagOrder = append(flagOrder, name)
	return p
}

// intFlag は boolFlag と同様に int 型のフラグとショートハンドを登録する。
func intFlag(fs *flag.FlagSet, name, shorthand string, value int, usage string) *int {
	p := fs.Int(name, value, usage)
	if shorthand != "" {
		fs.IntVar(p, shorthand, value, usage)
	}
	flagOrder = append(flagOrder, name)
	return p
}

// int64Flag は boolFlag と同様に int64 型のフラグを登録する（ショートハンド非対応）。
func int64Flag(fs *flag.FlagSet, name string, value int64, usage string) *int64 {
	p := fs.Int64(name, value, usage)
	flagOrder = append(flagOrder, name)
	return p
}

func main() {
	// コマンドライン引数の定義
	// 専用の FlagSet を使うことで、登録順を維持し、ヘルプで関連オプションを近くに表示する。
	cmdFlags = flag.NewFlagSet(os.Args[0], flag.ExitOnError)
	cmdFlags.SetOutput(os.Stderr)

	// --- 入出力・基本制御 ---
	filePath := stringFlag(cmdFlags, "file", "f", "", "path to CSV file (default: read from stdin)")
	output := stringFlag(cmdFlags, "output", "o", "text", "output format: md or text")
	header := stringFlag(cmdFlags, "header", "", "", "CSV string to use as table header. when set, the first row of input data is treated as data, not header")
	sortCol := intFlag(cmdFlags, "sort", "", 0, "sort data rows by 1-based column number (0 to disable)")
	reverseSort := boolFlag(cmdFlags, "reverse", "r", false, "reverse the sort order (used with --sort)")

	// --- テーブルサイズ・区切り ---
	pageBreak := stringFlag(cmdFlags, "page-break", "", "<!-- page %page/%total -->", "page separator template between pages. use %page and %total as placeholders. empty string disables separator")
	tableMaxWidth := intFlag(cmdFlags, "table-max-width", "w", 0, "maximum display width of a single text table row. behaves like --auto-fit but uses the given width instead of the terminal width. ignores --cell-max-width (text only)")
	tableMaxRows := intFlag(cmdFlags, "table-max-rows", "R", 0, "maximum number of data rows per page (0 for unlimited)")
	tableMaxLines := intFlag(cmdFlags, "table-max-lines", "L", 0, "maximum number of displayed data lines per page, accounting for newlines inside cells (0 for unlimited)")
	tableMaxByte := int64Flag(cmdFlags, "table-max-byte", 0, "maximum byte size per page of table output (0 for unlimited)")

	// --- セル・幅制御 ---
	cellMaxWidth := stringFlag(cmdFlags, "cell-max-width", "", "0", "maximum display width per cell row. single value applies to all columns, or comma-separated list per column (e.g. 5,6,7). 0 for unlimited, minimum 3")
	autoFit := boolFlag(cmdFlags, "auto-fit", "a", false, "auto-fit text table width to terminal (text only). ignores --cell-max-width")

	// --- Markdown 関連 ---
	mdCodeBlock := boolFlag(cmdFlags, "md-code-block", "", false, "wrap each MD cell content in code blocks")
	mdAlignBorder := boolFlag(cmdFlags, "md-align-border", "", false, "align Markdown table vertical borders per column")

	// --- 改行処理 ---
	lbReplacer := stringFlag(cmdFlags, "line-break", "", "", "string to replace newlines inside CSV cells. default is <br> for md and no conversion for text")

	// --- テキストテーブル外枠 ---
	topBorder := stringFlag(cmdFlags, "top-border", "", "-", "single char for top border line (text only)")
	bottomBorder := stringFlag(cmdFlags, "bottom-border", "", "-", "single char for bottom border line (text only)")
	leftBorder := stringFlag(cmdFlags, "left-border", "", "|", "single char for left border line (text only)")
	rightBorder := stringFlag(cmdFlags, "right-border", "", "|", "single char for right border line (text only)")
	headerBorder := stringFlag(cmdFlags, "header-border", "", "=", "single char for header/data separator line (text only)")
	inlineHBorder := stringFlag(cmdFlags, "inline-h-border", "", "-", "single char for inline horizontal borders (text only). empty string disables inline borders")
	inlineVBorder := stringFlag(cmdFlags, "inline-v-border", "", "|", "single char for inline vertical borders (text only)")
	cornerChar := stringFlag(cmdFlags, "corner-char", "", "+", "single char for corners and junction points (text only)")
	minimal := boolFlag(cmdFlags, "minimal", "m", false, "omit top, bottom, and inline horizontal borders in text output")

	// --- その他 ---
	showVersion := boolFlag(cmdFlags, "version", "V", false, "print version and exit")

	cmdFlags.Usage = func() {
		fmt.Fprintln(os.Stderr, "A command-line tool that takes CSV data from standard input or a file and formats it into a beautiful table for different platforms, including Markdown, ASCII text table.")
		fmt.Fprintln(os.Stderr)
		fmt.Fprintln(os.Stderr, "Find more information at: https://github.com/t-matsuo/c2tgo")
		fmt.Fprintln(os.Stderr)
		fmt.Fprintf(os.Stderr, "Usage of %s:\n", os.Args[0])
		printGroupedDefaults()
	}

	cmdFlags.Parse(os.Args[1:])

	if *showVersion {
		v := version
		if v == "" {
			v = "dev"
		}
		rev := commit
		if rev == "" {
			rev = "unknown"
		} else if len(rev) > 7 {
			rev = rev[:7]
		}
		fmt.Printf("Version: %s\nRevision: %s\nOS: %s\nArch: %s\n", v, rev, runtime.GOOS, runtime.GOARCH)
		return
	}

	// --minimal は上罫線・下罫線・内部水平罫線を空文字にする（個別指定があればそこを優先）
	if *minimal {
		if !isFlagSet("top-border") {
			*topBorder = ""
		}
		if !isFlagSet("bottom-border") {
			*bottomBorder = ""
		}
		if !isFlagSet("inline-h-border") {
			*inlineHBorder = ""
		}
	}

	// 半角 1 文字に制限（空文字は許可するオプションあり）
	checkSingleCharOrEmpty("--top-border", *topBorder)
	checkSingleCharOrEmpty("--bottom-border", *bottomBorder)
	checkSingleChar("--left-border", *leftBorder)
	checkSingleChar("--right-border", *rightBorder)
	checkSingleChar("--header-border", *headerBorder)
	checkSingleChar("--corner-char", *cornerChar)
	checkSingleCharOrEmpty("--inline-h-border", *inlineHBorder)
	checkSingleChar("--inline-v-border", *inlineVBorder)

	if *tableMaxRows < 0 {
		fmt.Fprintln(os.Stderr, "Error: --table-max-rows must be >= 0 (0 for unlimited)")
		os.Exit(1)
	}
	if *tableMaxLines < 0 {
		fmt.Fprintln(os.Stderr, "Error: --table-max-lines must be >= 0 (0 for unlimited)")
		os.Exit(1)
	}

	// --table-max-rows はヘッダー＋1データ行を確保できる値が必要
	if *tableMaxRows > 0 && *tableMaxRows < 2 {
		fmt.Fprintln(os.Stderr, "Error: --table-max-rows must be at least 2 (header + 1 data row)")
		os.Exit(1)
	}

	// --table-max-width の検証
	if *tableMaxWidth < 0 {
		fmt.Fprintln(os.Stderr, "Error: --table-max-width must be >= 0 (0 for unlimited)")
		os.Exit(1)
	}
	if *autoFit && *tableMaxWidth > 0 {
		fmt.Fprintln(os.Stderr, "Error: --auto-fit and --table-max-width are mutually exclusive")
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
				if *reverseSort {
					return rows[1+i][idx] > rows[1+j][idx]
				}
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

	// text 出力の場合の幅合わせ対象値を決定する
	// --auto-fit が指定されていればターミナル幅、--table-max-width が指定されていればその値を使用する
	fitWidth := 0
	if *autoFit {
		fitWidth = getTerminalWidth()
	} else if *tableMaxWidth > 0 {
		fitWidth = *tableMaxWidth
	}

	switch outFormat {
	case "text":
		t := buildTextTable(rows, maxCols, cellMaxWidths,
			*topBorder, *bottomBorder, *leftBorder, *rightBorder, *headerBorder,
			*cornerChar, *inlineHBorder, *inlineVBorder, lineBreak, *autoFit, fitWidth)
		printTable(t, *tableMaxByte, *tableMaxRows, *tableMaxLines, *pageBreak)
	default: // md
		t := buildMarkdownTable(rows, maxCols, lineBreak, cellMaxWidths, *mdCodeBlock, *mdAlignBorder)
		printTable(t, *tableMaxByte, *tableMaxRows, *tableMaxLines, *pageBreak)
	}
}

// isFlagSet は指定した名前のコマンドライン引数が明示的に渡されたかを返す。
func isFlagSet(name string) bool {
	set := false
	cmdFlags.Visit(func(f *flag.Flag) {
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
func buildMarkdownTable(rows [][]string, maxCols int, lineBreak string, cellMaxWidths []int, mdCodeBlock, mdAlignBorder bool) tableData {
	// 各セルを論理行に分解する。
	// lineBreak が空の場合は改行をそのまま複数行として出力する。
	// それ以外の場合は 1 行にまとめる。
	cellLines := make([][][]string, len(rows))
	for r, row := range rows {
		cellLines[r] = make([][]string, maxCols)
		for c, cell := range row {
			var lines []string
			if lineBreak != "" {
				s := cell
				s = strings.ReplaceAll(s, "\r\n", lineBreak)
				s = strings.ReplaceAll(s, "\r", lineBreak)
				s = strings.ReplaceAll(s, "\n", lineBreak)
				lines = []string{s}
			} else {
				normalized := strings.ReplaceAll(cell, "\r\n", "\n")
				normalized = strings.ReplaceAll(normalized, "\r", "\n")
				lines = strings.Split(normalized, "\n")
			}
			for i, s := range lines {
				// 長さ制限を適用（``` は含めない）
				maxW := cellMaxWidths[c]
				if maxW > 0 && getDisplayWidth(s) > maxW {
					s = truncateWithEllipsis(s, maxW)
				}
				// コードブロックで囲む
				if mdCodeBlock {
					s = "```" + s + "```"
				}
				lines[i] = s
			}
			cellLines[r][c] = lines
		}
	}

	// 罫線を揃える場合は各列の表示幅を計算する（Markdown 区切り行は最低 3 文字必要）
	var colWidths []int
	if mdAlignBorder {
		colWidths = make([]int, maxCols)
		for _, row := range cellLines {
			for c, lines := range row {
				for _, line := range lines {
					if w := getDisplayWidth(line); w > colWidths[c] {
						colWidths[c] = w
					}
				}
			}
		}
		for c := range colWidths {
			if colWidths[c] < 3 {
				colWidths[c] = 3
			}
		}
	}

	var t tableData

	// ヘッダー行
	headerCells := make([]string, maxCols)
	for c := 0; c < maxCols; c++ {
		s := ""
		if len(cellLines[0][c]) > 0 {
			s = cellLines[0][c][0]
		}
		if mdAlignBorder {
			s = padRight(s, colWidths[c])
		}
		headerCells[c] = s
	}
	t.headerLines = append(t.headerLines, "| "+strings.Join(headerCells, " | ")+" |")

	// 区切り行
	sepCells := make([]string, maxCols)
	for c := 0; c < maxCols; c++ {
		w := 3
		if mdAlignBorder {
			w = colWidths[c]
		}
		sepCells[c] = strings.Repeat("-", w)
	}
	t.headerLines = append(t.headerLines, "| "+strings.Join(sepCells, " | ")+" |")

	// データ行：セル内改行があれば追加の行を出力する。
	// 1 つの CSV 行から生成された複数行は 1 ユニットとしてまとめ、
	// ページ分割時に同じセルのデータが分断されないようにする。
	for _, row := range cellLines[1:] {
		maxLines := 0
		for _, lines := range row {
			if len(lines) > maxLines {
				maxLines = len(lines)
			}
		}
		unit := make([]string, 0, maxLines)
		for i := 0; i < maxLines; i++ {
			cells := make([]string, maxCols)
			for c := 0; c < maxCols; c++ {
				s := ""
				if i < len(row[c]) {
					s = row[c][i]
				}
				if mdAlignBorder {
					s = padRight(s, colWidths[c])
				}
				cells[c] = s
			}
			unit = append(unit, "| "+strings.Join(cells, " | ")+" |")
		}
		t.units = append(t.units, unit)
	}
	return t
}

// Text 形式の表データを組み立てる
func buildTextTable(rows [][]string, maxCols int, cellMaxWidths []int, top, bottom, left, right, header, corner, inlineH, inlineV, lineBreak string, autoFit bool, fitWidth int) tableData {
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

	// 各カラムの最大文字幅を計算（全データ行から算出するため、どのページでも同じ幅になる）
	naturalWidths := make([]int, maxCols)
	for _, row := range cellLines {
		for c, lines := range row {
			for _, line := range lines {
				if w := getDisplayWidth(line); w > naturalWidths[c] {
					naturalWidths[c] = w
				}
			}
		}
	}

	// 列幅を決定する
	colWidths := make([]int, maxCols)
	if fitWidth > 0 {
		colWidths = calculateAutoFitWidths(naturalWidths, maxCols, left, right, inlineV, corner, fitWidth, !autoFit)
	} else {
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

		// cellMaxWidth の制限を反映して列幅を計算
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
	}

	var t tableData
	t.inlineBorder = generateBorderRow(colWidths, inlineH, corner)
	t.borderLast = generateBorderRow(colWidths, bottom, corner)

	// ヘッダーブロック: 上罫線（空でなければ）＋ヘッダー行＋ヘッダー罫線
	// （ヘッダーセル内に改行があれば複数物理行になるためすべて追加する）
	if top != "" {
		t.headerLines = append(t.headerLines, generateBorderRow(colWidths, top, corner))
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

// calculateAutoFitWidths は指定された幅に収まるように列幅を計算する。
// 自然幅の合計が指定幅に収まる場合は自然幅をそのまま使用する。
// 収まらない場合は、幅の広い列から優先的に縮小しつつ、各列が自然幅に対して
// 極端に短くならないように最小幅を保証する。
func calculateAutoFitWidths(natural []int, maxCols int, left, right, inlineV, corner string, fitWidth int, enforceMin bool) []int {
	overhead := tableOverhead(maxCols, left, right, inlineV, corner)
	available := fitWidth - overhead
	if available < 0 {
		available = 0
	}

	// 縮小が必要な場合、各列の最小幅を自然幅の 30% または 3 の大きい方で決定
	minWidths := make([]int, maxCols)
	for c, w := range natural {
		minW := int(float64(w) * 0.3)
		if minW < 3 {
			minW = 3
		}
		minWidths[c] = minW
	}

	// 指定幅が圧倒的に小さく、最小幅を保証できない場合はエラーにする
	// （--auto-fit の場合はターミナル幅に合わせてオーバーフローさせるため、ここでは検証しない）
	minRequired := overhead + sumInts(minWidths)
	if enforceMin && fitWidth < minRequired {
		fmt.Fprintf(os.Stderr, "Error: --table-max-width %d is too small to hold the table (minimum %d required)\n", fitWidth, minRequired)
		os.Exit(1)
	}

	widths := append([]int(nil), natural...)
	total := sumInts(widths)
	if total <= available {
		return widths
	}

	// 最も幅の広い列から優先的に 1 文字ずつ縮小する。
	// これにより、改行は幅の広いセルに優先的に入る。
	for total > available {
		widest := -1
		for c := 0; c < maxCols; c++ {
			if widths[c] > minWidths[c] {
				if widest == -1 || widths[c] > widths[widest] {
					widest = c
				}
			}
		}
		if widest == -1 {
			// これ以上縮小できない場合ははみ出すが仕方ない
			break
		}
		widths[widest]--
		total--
	}

	return widths
}

// tableOverhead は text 形式のテーブルにおいて、列コンテンツ以外の文字幅を返す。
// 例: "| a | b |" では列幅の合計に加えて左右の境界とスペース、区切り文字が必要。
func tableOverhead(maxCols int, left, right, inlineV, corner string) int {
	// 行のオーバーヘッド: left + right + 2*maxCols スペース + (maxCols-1) inlineV
	rowOverhead := 1 + 1 + 2*maxCols + (maxCols - 1)
	// 罫線行のオーバーヘッド: (maxCols+1) 角文字 + 2*maxCols の罫線パディング
	borderOverhead := (maxCols + 1) + 2*maxCols
	// 同じ幅になるはずだが、安全の大きい方を採用
	if rowOverhead > borderOverhead {
		return rowOverhead
	}
	return borderOverhead
}

// sumInts はスライスの合計を返す。
func sumInts(xs []int) int {
	total := 0
	for _, x := range xs {
		total += x
	}
	return total
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

// printTable は、maxBytes / maxRows / maxLines を超えないように表をページ分割して出力する。
// 2 ページ目以降もヘッダーを付け、各ページは必ず完結した表になる。
func printTable(t tableData, maxBytes int64, maxRows, maxLines int, pageBreak string) {
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
		units = append(units, sizedUnit{lines: u, size: size, lineCount: len(u)})
	}

	// 固定コストの計算
	fenceCost := int64(0)
	if t.codeBlock {
		fenceCost = lineBytes("```") * 2 // 開始＋終了
	}
	hSize := headerBytes(t)

	// 表示行数の固定コストを計算する
	headerLines := len(t.headerLines)
	bottomBorderLines := 0
	if t.borderLast != "" {
		bottomBorderLines = 1
	}
	codeBlockFences := 0
	if t.codeBlock {
		codeBlockFences = 2
	}
	fixedLines := headerLines + bottomBorderLines + codeBlockFences

	// 最小のデータユニットの表示行数を求める
	minDataLines := 0
	for _, u := range units {
		if minDataLines == 0 || u.lineCount < minDataLines {
			minDataLines = u.lineCount
		}
	}

	// --table-max-lines がヘッダー＋1データ行を表示できる値か検証
	if maxLines > 0 {
		minRequired := fixedLines
		if minDataLines > 0 {
			minRequired += minDataLines
		}
		if maxLines < minRequired {
			fmt.Fprintf(os.Stderr, "Error: --table-max-lines must be at least %d to hold the table header and one data row\n", minRequired)
			os.Exit(1)
		}
	}

	// ページ区切り行のバイト数は総ページ数（%total の桁数など）に依存し、
	// 総ページ数は区切りのバイト数にも依存する（循環参照）。
	// そのため、まず概算値でページング → 確定した総ページ数から実際の区切りサイズを
	// 計算 → 再ページング、を 2〜3 回繰り返して収束させる。
	reserve := pageBreakBytes(pageBreak, 1, 1) // 初期概算
	var pages []page
	for attempt := 0; attempt < 3; attempt++ {
		pages = paginate(t, units, hSize, fenceCost, maxBytes, reserve, maxRows, maxLines, fixedLines)
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

// paginate は、maxBytes / maxRows / maxLines を超えないようにデータユニットをページへ割り振る。
// reserve はページ区切り行 1 行分の見込みバイト数。
// maxRows はヘッダー行を含む論理行数、maxLines はヘッダー・罫線も含む実際の表示行数を制限する。
func paginate(t tableData, units []sizedUnit, hSize, fenceCost, maxBytes, reserve int64, maxRows, maxLines, fixedLines int) []page {
	var pages []page

	if maxBytes <= 0 && maxRows <= 0 && maxLines <= 0 {
		// 制限なし: 1 ページに全部
		p := page{}
		for i := range units {
			p.unitIdx = append(p.unitIdx, i)
		}
		pages = append(pages, p)
		return pages
	}

	// 表示行数関連のコストを計算する
	headerRows := 1
	inlineBorderLines := 0
	if t.inlineBorder != "" {
		inlineBorderLines = 1
	}

	var cur []int // 現在のページに割り当てたユニットのインデックス
	var curSize int64
	var curRows int
	var curLines int // 現在のページのデータ行＋内部罫線の合計
	flush := func() {
		if len(cur) > 0 {
			pages = append(pages, page{unitIdx: cur})
			cur = nil
			curSize = 0
			curRows = 0
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

		// maxRows 制限がある場合（ヘッダー行を含む論理行数）
		if maxRows > 0 && headerRows+curRows+1 > maxRows && len(cur) > 0 {
			needFlush = true
		}

		// maxLines 制限がある場合（ヘッダー・罫線を含む表示行数）
		if maxLines > 0 {
			projectedUnits := len(cur) + 1
			projectedLines := curLines + u.lineCount + inlineBorderLines
			actualProjected := fixedLines + projectedLines
			if projectedUnits > 0 && inlineBorderLines > 0 {
				actualProjected -= inlineBorderLines // 最終ユニットの内部罫線は下罫線に置き換わるため
			}
			if actualProjected > maxLines && len(cur) > 0 {
				needFlush = true
			}
		}

		if needFlush {
			flush()
		}

		cur = append(cur, i)
		curSize += u.size
		curRows++
		curLines += u.lineCount + inlineBorderLines

		// ヘッダー＋1 ユニットすら入らない場合は、1 ユニットでページを強制成立させる
		if maxBytes > 0 {
			avail := maxBytes - reserve
			if curSize > avail-hSize {
				flush()
			}
		}

		if maxRows > 0 && headerRows+curRows >= maxRows {
			flush()
		}

		if maxLines > 0 {
			k := len(cur)
			actualCurrent := fixedLines + curLines
			if k > 0 && inlineBorderLines > 0 {
				actualCurrent -= inlineBorderLines
			}
			if actualCurrent >= maxLines {
				flush()
			}
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
