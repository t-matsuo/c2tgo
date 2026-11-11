package main

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// buildBinary はテスト対象の c2tgo バイナリを一時ディレクトリにビルドしてパスを返す。
func buildBinary(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	bin := filepath.Join(dir, "c2tgo")
	out, err := exec.Command("go", "build", "-o", bin, ".").CombinedOutput()
	if err != nil {
		t.Fatalf("failed to build binary: %v\n%s", err, out)
	}
	return bin
}

// run は c2tgo バイナリをサブプロセスで実行する。
func run(t *testing.T, bin string, input string, args ...string) string {
	t.Helper()
	return runWithEnv(t, bin, os.Environ(), input, args...)
}

// envWithColumns は COLUMNS 環境変数を指定した値に設定した環境変数リストを返す。
// 既存の COLUMNS があれば置き換える。
func envWithColumns(cols int) []string {
	env := os.Environ()
	var out []string
	for _, e := range env {
		if !strings.HasPrefix(e, "COLUMNS=") {
			out = append(out, e)
		}
	}
	out = append(out, fmt.Sprintf("COLUMNS=%d", cols))
	return out
}

// runWithEnv は指定した環境変数で c2tgo バイナリをサブプロセスで実行する。
func runWithEnv(t *testing.T, bin string, env []string, input string, args ...string) string {
	t.Helper()
	cmd := exec.Command(bin, args...)
	cmd.Env = env
	cmd.Stdin = strings.NewReader(input)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		t.Fatalf("command failed: %v\nstderr: %s", err, stderr.String())
	}
	return stdout.String()
}

func TestDefaultOutputIsText(t *testing.T) {
	bin := buildBinary(t)
	input := "Name,Age\nAlice,30\nBob,25\n"
	stdout := run(t, bin, input)
	want := []string{
		"+-------+-----+",
		"| Name  | Age |",
		"+=======+=====+",
		"| Alice | 30  |",
		"+-------+-----+",
		"| Bob   | 25  |",
		"+-------+-----+",
	}
	got := strings.Split(strings.TrimRight(stdout, "\n"), "\n")
	if len(got) != len(want) {
		t.Fatalf("unexpected line count: got %d, want %d\n%s", len(got), len(want), stdout)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("line %d mismatch:\n got: %s\nwant: %s", i, got[i], want[i])
		}
	}
}

func TestOutputMarkdown(t *testing.T) {
	bin := buildBinary(t)
	input := "Name,Age\nAlice,30\n"
	stdout := run(t, bin, input, "--output=md")
	want := "| Name | Age |\n| --- | --- |\n| Alice | 30 |\n"
	if stdout != want {
		t.Errorf("unexpected output:\n%s\nwant:\n%s", stdout, want)
	}
}

func TestOutputMarkdownShorthand(t *testing.T) {
	bin := buildBinary(t)
	input := "Name,Age\nAlice,30\n"
	stdout := run(t, bin, input, "-o", "md")
	want := "| Name | Age |\n| --- | --- |\n| Alice | 30 |\n"
	if stdout != want {
		t.Errorf("unexpected output:\n%s\nwant:\n%s", stdout, want)
	}
}

func TestHeaderOption(t *testing.T) {
	bin := buildBinary(t)
	input := "Alice,30\nBob,25\n"
	stdout := run(t, bin, input, "--header=Name,Age", "--output=text")
	lines := strings.Split(strings.TrimRight(stdout, "\n"), "\n")
	if len(lines) < 2 || lines[1] != "| Name  | Age |" {
		t.Errorf("header not applied correctly:\n%s", stdout)
	}
	if !strings.Contains(stdout, "| Alice | 30  |") {
		t.Errorf("first data row missing:\n%s", stdout)
	}
}

func TestSortOption(t *testing.T) {
	bin := buildBinary(t)
	input := "Name,Age\nBob,25\nAlice,30\n"
	stdout := run(t, bin, input, "--sort=1", "--output=text")
	lines := strings.Split(strings.TrimRight(stdout, "\n"), "\n")
	if len(lines) < 6 {
		t.Fatalf("unexpected output:\n%s", stdout)
	}
	if !strings.Contains(lines[3], "Alice") {
		t.Errorf("expected Alice at first data row, got: %s", lines[3])
	}
	if !strings.Contains(lines[5], "Bob") {
		t.Errorf("expected Bob at second data row, got: %s", lines[5])
	}
}

func TestReverseSortOption(t *testing.T) {
	bin := buildBinary(t)
	input := "Name,Age\nBob,25\nAlice,30\n"
	stdout := run(t, bin, input, "--sort=1", "--reverse", "--output=text")
	lines := strings.Split(strings.TrimRight(stdout, "\n"), "\n")
	if len(lines) < 6 {
		t.Fatalf("unexpected output:\n%s", stdout)
	}
	if !strings.Contains(lines[3], "Bob") {
		t.Errorf("expected Bob at first data row with --reverse, got: %s", lines[3])
	}
	if !strings.Contains(lines[5], "Alice") {
		t.Errorf("expected Alice at second data row with --reverse, got: %s", lines[5])
	}
}

func TestReverseSortShorthand(t *testing.T) {
	bin := buildBinary(t)
	input := "Name,Age\nBob,25\nAlice,30\n"
	stdout := run(t, bin, input, "--sort=1", "-r", "--output=text")
	lines := strings.Split(strings.TrimRight(stdout, "\n"), "\n")
	if len(lines) < 6 {
		t.Fatalf("unexpected output:\n%s", stdout)
	}
	if !strings.Contains(lines[3], "Bob") {
		t.Errorf("expected Bob at first data row with -r, got: %s", lines[3])
	}
}

func TestFileOption(t *testing.T) {
	bin := buildBinary(t)
	dir := t.TempDir()
	path := filepath.Join(dir, "data.csv")
	if err := os.WriteFile(path, []byte("Name,Age\nAlice,30\n"), 0644); err != nil {
		t.Fatal(err)
	}
	stdout := run(t, bin, "", "--file", path, "--output=text")
	if !strings.Contains(stdout, "| Alice | 30  |") {
		t.Errorf("file input not processed correctly:\n%s", stdout)
	}
}

func TestEmptyInlineHBorder(t *testing.T) {
	bin := buildBinary(t)
	input := "Name,Age\nAlice,30\nBob,25\n"
	stdout := run(t, bin, input, "--inline-h-border=", "--output=text")
	lines := strings.Split(strings.TrimRight(stdout, "\n"), "\n")
	// 上罫線 + ヘッダー行 + ヘッダー区切り + データ行2 + 下罫線 = 6 行
	wantCount := 6
	if len(lines) != wantCount {
		t.Fatalf("expected %d lines, got %d:\n%s", wantCount, len(lines), stdout)
	}
	// データ行間の罫線はないが、下罫線は表示される
	if lines[0] != "+-------+-----+" {
		t.Errorf("missing top border: %s", lines[0])
	}
	if lines[len(lines)-1] != "+-------+-----+" {
		t.Errorf("missing bottom border: %s", lines[len(lines)-1])
	}
}

func TestEmptyTopBottomBorder(t *testing.T) {
	bin := buildBinary(t)
	input := "Name,Age\nAlice,30\nBob,25\n"
	stdout := run(t, bin, input, "--output=text", "--top-border=", "--bottom-border=")
	want := []string{
		"| Name  | Age |",
		"+=======+=====+",
		"| Alice | 30  |",
		"+-------+-----+",
		"| Bob   | 25  |",
	}
	got := strings.Split(strings.TrimRight(stdout, "\n"), "\n")
	if len(got) != len(want) {
		t.Fatalf("unexpected line count: got %d, want %d\n%s", len(got), len(want), stdout)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("line %d mismatch:\n got: %s\nwant: %s", i, got[i], want[i])
		}
	}
}

func TestMinimalFlag(t *testing.T) {
	bin := buildBinary(t)
	input := "Name,Age\nAlice,30\nBob,25\n"
	stdout := run(t, bin, input, "--output=text", "--minimal")
	want := []string{
		"| Name  | Age |",
		"+=======+=====+",
		"| Alice | 30  |",
		"| Bob   | 25  |",
	}
	got := strings.Split(strings.TrimRight(stdout, "\n"), "\n")
	if len(got) != len(want) {
		t.Fatalf("unexpected line count: got %d, want %d\n%s", len(got), len(want), stdout)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("line %d mismatch:\n got: %s\nwant: %s", i, got[i], want[i])
		}
	}
}

func TestMinimalShorthand(t *testing.T) {
	bin := buildBinary(t)
	input := "Name,Age\nAlice,30\nBob,25\n"
	stdout := run(t, bin, input, "--output=text", "-m")
	want := []string{
		"| Name  | Age |",
		"+=======+=====+",
		"| Alice | 30  |",
		"| Bob   | 25  |",
	}
	got := strings.Split(strings.TrimRight(stdout, "\n"), "\n")
	if len(got) != len(want) {
		t.Fatalf("unexpected line count: got %d, want %d\n%s", len(got), len(want), stdout)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("line %d mismatch:\n got: %s\nwant: %s", i, got[i], want[i])
		}
	}
}

func TestMinimalRespectsExplicitTopBorder(t *testing.T) {
	bin := buildBinary(t)
	input := "Name,Age\nAlice,30\n"
	stdout := run(t, bin, input, "--output=text", "--minimal", "--top-border", "^")
	want := []string{
		"+^^^^^^^+^^^^^+",
		"| Name  | Age |",
		"+=======+=====+",
		"| Alice | 30  |",
	}
	got := strings.Split(strings.TrimRight(stdout, "\n"), "\n")
	if len(got) != len(want) {
		t.Fatalf("unexpected line count: got %d, want %d\n%s", len(got), len(want), stdout)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("line %d mismatch:\n got: %s\nwant: %s", i, got[i], want[i])
		}
	}
}

func TestMarkdownAlignBorder(t *testing.T) {
	bin := buildBinary(t)
	input := "Name,Age,City\nAlice,30,Tokyo\n"
	stdout := run(t, bin, input, "--output=md", "--md-align-border")
	want := "| Name  | Age | City  |\n| ----- | --- | ----- |\n| Alice | 30  | Tokyo |\n"
	if stdout != want {
		t.Errorf("unexpected output:\n%s\nwant:\n%s", stdout, want)
	}
}

func TestMarkdownLineBreakEmpty(t *testing.T) {
	bin := buildBinary(t)
	input := "Name,City\nAlice,\"Tokyo\nOsaka\"\n"
	stdout := run(t, bin, input, "--output=md", "--line-break=")
	want := "| Name | City |\n| --- | --- |\n| Alice | Tokyo |\n|  | Osaka |\n"
	if stdout != want {
		t.Errorf("unexpected output:\n%s\nwant:\n%s", stdout, want)
	}
}

func TestMarkdownAlignBorderWithEmptyLineBreak(t *testing.T) {
	bin := buildBinary(t)
	input := "Name,City\nAlice,\"Tokyo\nOsaka\"\nBob,Kyoto\n"
	stdout := run(t, bin, input, "--output=md", "--md-align-border", "--line-break=")
	want := "| Name  | City  |\n| ----- | ----- |\n| Alice | Tokyo |\n|       | Osaka |\n| Bob   | Kyoto |\n"
	if stdout != want {
		t.Errorf("unexpected output:\n%s\nwant:\n%s", stdout, want)
	}
}

func TestMarkdownEmptyLineBreakDoesNotSplitCellAcrossPages(t *testing.T) {
	bin := buildBinary(t)
	input := "Name,Age,City\nAlice,30,\"Tokyo\nHokkaido\"\nBob,25,Osaka\nKen,43,Kyoto\n"
	stdout := run(t, bin, input, "--output=md", "--md-align-border", "--line-break=", "--table-max-lines=4")
	pages := strings.Split(stdout, "<!-- page ")
	// 1ページ目: ヘッダー2行 + Alice（2行）= 4行
	// 2ページ目: ヘッダー2行 + Bob（1行）+ Ken（1行）= 4行
	// よりページ区切りは 1 つだけで、総ページ数は 2
	if len(pages) != 2 {
		t.Fatalf("expected 2 pages (cell should not be split), got %d\n%s", len(pages), stdout)
	}
	// Tokyo と Hokkaido が同じページにあることを確認
	if !strings.Contains(pages[0], "Tokyo") || !strings.Contains(pages[0], "Hokkaido") {
		t.Errorf("Tokyo and Hokkaido should be on the same page:\n%s", stdout)
	}
}

func TestVersion(t *testing.T) {
	bin := buildBinary(t)
	cmd := exec.Command(bin, "--version")
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("--version failed: %v\n%s", err, out)
	}
	got := strings.TrimSpace(string(out))
	if got == "" {
		t.Errorf("--version output is empty")
	}
}

func TestVersionWithBuildTime(t *testing.T) {
	dir := t.TempDir()
	bin := filepath.Join(dir, "c2tgo")
	ldflags := "-X main.version=20261109-215633 -X main.commit=eee119cc4d8f1f3a7a0b8c2e5d6f7a8b9c0d1e2f"
	out, err := exec.Command("go", "build", "-ldflags", ldflags, "-o", bin, ".").CombinedOutput()
	if err != nil {
		t.Fatalf("failed to build versioned binary: %v\n%s", err, out)
	}
	got, err := exec.Command(bin, "--version").CombinedOutput()
	if err != nil {
		t.Fatalf("--version failed: %v\n%s", err, got)
	}
	output := string(got)
	if !strings.Contains(output, "Version: 20261109-215633") {
		t.Errorf("expected version in output, got: %s", output)
	}
	if !strings.Contains(output, "Revision: eee119c") {
		t.Errorf("expected short revision in output, got: %s", output)
	}
	if !strings.Contains(output, "OS:") {
		t.Errorf("expected OS in output, got: %s", output)
	}
	if !strings.Contains(output, "Arch:") {
		t.Errorf("expected Arch in output, got: %s", output)
	}
}

func TestVersionShorthandV(t *testing.T) {
	bin := buildBinary(t)
	cmd := exec.Command(bin, "-V")
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("-V failed: %v\n%s", err, out)
	}
	got := strings.TrimSpace(string(out))
	if got == "" {
		t.Errorf("-V output is empty")
	}
	if !strings.Contains(got, "Version:") {
		t.Errorf("expected Version: in -V output, got: %s", got)
	}
}

func TestHelpGroupsAliases(t *testing.T) {
	bin := buildBinary(t)
	cmd := exec.Command(bin, "-h")
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("-h failed: %v\n%s", err, out)
	}
	help := string(out)

	// 同じ変数に束縛された長い名前とショートハンドが 1 行にまとまっている。
	if !strings.Contains(help, "-version, -V") {
		t.Errorf("expected -version and -V to be grouped on one line\n%s", help)
	}
	if !strings.Contains(help, "-output, -o") {
		t.Errorf("expected -output and -o to be grouped on one line\n%s", help)
	}

	// ショートハンド専用の重複した説明文がない。
	if strings.Contains(help, "print version and exit (shorthand)") {
		t.Errorf("expected shorthand-only description to be omitted\n%s", help)
	}
}

func TestSortOutOfRange(t *testing.T) {
	bin := buildBinary(t)
	input := "Name,Age\nAlice,30\n"
	cmd := exec.Command(bin, "--sort=5", "--output=text")
	cmd.Stdin = strings.NewReader(input)
	out, err := cmd.CombinedOutput()
	if err == nil {
		t.Fatalf("expected error for out-of-range sort column")
	}
	if !strings.Contains(string(out), "out of range") {
		t.Errorf("unexpected error message: %s", out)
	}
}

func TestCellMaxWidthSingleValue(t *testing.T) {
	bin := buildBinary(t)
	input := "Name\nAlice\n"
	stdout := run(t, bin, input, "--cell-max-width=3", "--output=md")
	want := "| ... |\n| --- |\n| ... |\n"
	if stdout != want {
		t.Errorf("unexpected output:\n%s\nwant:\n%s", stdout, want)
	}
}

func TestCellMaxWidthPerColumn(t *testing.T) {
	bin := buildBinary(t)
	input := "Name,Age,City\nAlice,30,Tokyo\n"
	stdout := run(t, bin, input, "--cell-max-width=5,0,4", "--output=text")
	lines := strings.Split(strings.TrimRight(stdout, "\n"), "\n")
	if len(lines) < 4 {
		t.Fatalf("unexpected output:\n%s", stdout)
	}
	// 1列目は5文字、3列目は4文字（1文字 + "..."）に制限される
	if !strings.Contains(lines[3], "| Alice | 30  | T... |") {
		t.Errorf("unexpected per-column width:\n%s", stdout)
	}
}

func TestCellMaxWidthPerColumnUnlimited(t *testing.T) {
	bin := buildBinary(t)
	input := "Name,Age\nAlice,30\n"
	stdout := run(t, bin, input, "--cell-max-width=0,3", "--output=md")
	want := "| Name | Age |\n| --- | --- |\n| Alice | 30 |\n"
	if stdout != want {
		t.Errorf("unexpected output:\n%s\nwant:\n%s", stdout, want)
	}
}

func TestLineBreakReplacementMarkdown(t *testing.T) {
	bin := buildBinary(t)
	input := "Name,City\nAlice,\"Tokyo\nOsaka\"\n"
	stdout := run(t, bin, input, "--output=md", "--line-break= / ")
	if !strings.Contains(stdout, "Tokyo / Osaka") {
		t.Errorf("line break not replaced correctly:\n%s", stdout)
	}
}

func TestLineBreakReplacementText(t *testing.T) {
	bin := buildBinary(t)
	input := "Name,City\nAlice,\"Tokyo\nOsaka\"\n"
	stdout := run(t, bin, input, "--output=text", "--line-break= / ")
	if !strings.Contains(stdout, "| Alice | Tokyo / Osaka |") {
		t.Errorf("line break not replaced correctly in text output:\n%s", stdout)
	}
}

func TestDefaultLineBreakText(t *testing.T) {
	bin := buildBinary(t)
	input := "Name,City\nAlice,\"Tokyo\nOsaka\"\n"
	stdout := run(t, bin, input, "--output=text")
	if !strings.Contains(stdout, "| Alice | Tokyo |") {
		t.Errorf("expected Tokyo on first physical line:\n%s", stdout)
	}
	if !strings.Contains(stdout, "|       | Osaka |") {
		t.Errorf("expected Osaka on second physical line:\n%s", stdout)
	}
	if strings.Contains(stdout, "<br>") {
		t.Errorf("default text output should not convert newlines to <br>:\n%s", stdout)
	}
}

func TestDefaultLineBreakMarkdown(t *testing.T) {
	bin := buildBinary(t)
	input := "Name,City\nAlice,\"Tokyo\nOsaka\"\n"
	stdout := run(t, bin, input, "--output=md")
	if !strings.Contains(stdout, "| Alice | Tokyo<br>Osaka |") {
		t.Errorf("expected default <br> conversion in markdown:\n%s", stdout)
	}
}

func TestAutoFitText(t *testing.T) {
	bin := buildBinary(t)
	input := "Name,Description\nAlice,This is a very long description text\n"
	stdout := runWithEnv(t, bin, envWithColumns(40), input, "--output=text", "--auto-fit")
	lines := strings.Split(strings.TrimRight(stdout, "\n"), "\n")
	for _, line := range lines {
		if w := getDisplayWidth(line); w > 40 {
			t.Errorf("line exceeds terminal width 40: %q (width=%d)", line, w)
		}
	}
}

func TestAutoFitTextShorthand(t *testing.T) {
	bin := buildBinary(t)
	input := "Name,Description\nAlice,This is a very long description text\n"
	stdout := runWithEnv(t, bin, envWithColumns(40), input, "--output=text", "-a")
	lines := strings.Split(strings.TrimRight(stdout, "\n"), "\n")
	for _, line := range lines {
		if w := getDisplayWidth(line); w > 40 {
			t.Errorf("line exceeds terminal width 40 with -a shorthand: %q (width=%d)", line, w)
		}
	}
}

func TestAutoFitIgnoresCellMaxWidth(t *testing.T) {
	bin := buildBinary(t)
	input := "Name,Description\nAlice,This is a very long description text\n"
	stdout := runWithEnv(t, bin, envWithColumns(100), input, "--output=text", "--auto-fit", "--cell-max-width=5")
	if strings.Contains(stdout, "...") {
		t.Errorf("--cell-max-width should be ignored with --auto-fit:\n%s", stdout)
	}
	if !strings.Contains(stdout, "very long description text") {
		t.Errorf("description should not be truncated with --auto-fit:\n%s", stdout)
	}
}

func TestTableMaxWidthText(t *testing.T) {
	bin := buildBinary(t)
	input := "Name,Description\nAlice,This is a very long description text\n"
	stdout := run(t, bin, input, "--output=text", "--table-max-width=40")
	lines := strings.Split(strings.TrimRight(stdout, "\n"), "\n")
	for _, line := range lines {
		if w := getDisplayWidth(line); w > 40 {
			t.Errorf("line exceeds --table-max-width 40: %q (width=%d)", line, w)
		}
	}
}

func TestTableMaxWidthShorthand(t *testing.T) {
	bin := buildBinary(t)
	input := "Name,Description\nAlice,This is a very long description text\n"
	stdout := run(t, bin, input, "--output=text", "-w=40")
	lines := strings.Split(strings.TrimRight(stdout, "\n"), "\n")
	for _, line := range lines {
		if w := getDisplayWidth(line); w > 40 {
			t.Errorf("line exceeds -w 40: %q (width=%d)", line, w)
		}
	}
}

func TestTableMaxWidthTooSmall(t *testing.T) {
	bin := buildBinary(t)
	input := "Name,Description\nAlice,This is a very long description text\n"
	cmd := exec.Command(bin, "--output=text", "--table-max-width=5")
	cmd.Stdin = strings.NewReader(input)
	out, err := cmd.CombinedOutput()
	if err == nil {
		t.Fatalf("expected error for too small --table-max-width")
	}
	if !strings.Contains(string(out), "too small") {
		t.Errorf("unexpected error message: %s", out)
	}
}

func TestTableMaxWidthAutoFitExclusive(t *testing.T) {
	bin := buildBinary(t)
	input := "Name,Age\nAlice,30\n"
	cmd := exec.Command(bin, "--output=text", "--auto-fit", "--table-max-width=40")
	cmd.Stdin = strings.NewReader(input)
	out, err := cmd.CombinedOutput()
	if err == nil {
		t.Fatalf("expected error when --auto-fit and --table-max-width are used together")
	}
	if !strings.Contains(string(out), "mutually exclusive") {
		t.Errorf("unexpected error message: %s", out)
	}
}

func TestTableMaxWidthIgnoresCellMaxWidth(t *testing.T) {
	bin := buildBinary(t)
	input := "Name,Description\nAlice,This is a very long description text\n"
	stdout := run(t, bin, input, "--output=text", "--table-max-width=100", "--cell-max-width=5")
	if strings.Contains(stdout, "...") {
		t.Errorf("--cell-max-width should be ignored with --table-max-width:\n%s", stdout)
	}
	if !strings.Contains(stdout, "very long description text") {
		t.Errorf("description should not be truncated with --table-max-width:\n%s", stdout)
	}
}

func TestTableMaxRowsCountsHeader(t *testing.T) {
	bin := buildBinary(t)
	input := "Name,City\nAlice,Tokyo\nBob,Kyoto\nKen,Nagoya\n"
	// --table-max-rows はヘッダー行もカウントするため、3 で「ヘッダー＋データ2行」まで
	stdout := run(t, bin, input, "--output=text", "--table-max-rows=3")
	pages := strings.Split(stdout, "<!-- page ")
	if len(pages) != 2 {
		t.Fatalf("expected 2 pages (header counted), got %d\n%s", len(pages), stdout)
	}
	// 4 であればヘッダー＋データ3行で 1 ページに収まる
	stdout = run(t, bin, input, "--output=text", "--table-max-rows=4")
	pages = strings.Split(stdout, "<!-- page ")
	if len(pages) != 1 {
		t.Fatalf("expected 1 page with maxRows=4, got %d\n%s", len(pages), stdout)
	}
}

func TestTableMaxRowsShorthand(t *testing.T) {
	bin := buildBinary(t)
	input := "Name,City\nAlice,Tokyo\nBob,Kyoto\nKen,Nagoya\n"
	stdout := run(t, bin, input, "--output=text", "-R", "3")
	pages := strings.Split(stdout, "<!-- page ")
	if len(pages) != 2 {
		t.Fatalf("expected 2 pages with -R, got %d\n%s", len(pages), stdout)
	}
}

func TestTableMaxLinesCountsHeaderAndBorders(t *testing.T) {
	bin := buildBinary(t)
	// Alice が 2 行、Bob が 1 行の表示データ行を持つ
	// text テーブル全体ではヘッダー3行 + Alice2行 + 内部罫線1行 + Bob1行 + 下罫線1行 = 8行
	input := "Name,City\nAlice,\"Tokyo\nOsaka\"\nBob,Kyoto\n"
	stdout := run(t, bin, input, "--output=text", "--table-max-lines=8")
	pages := strings.Split(stdout, "<!-- page ")
	if len(pages) != 1 {
		t.Fatalf("expected 1 page with maxLines=8, got %d\n%s", len(pages), stdout)
	}
	stdout = run(t, bin, input, "--output=text", "--table-max-lines=7")
	pages = strings.Split(stdout, "<!-- page ")
	if len(pages) != 2 {
		t.Fatalf("expected 2 pages with maxLines=7, got %d\n%s", len(pages), stdout)
	}
}

func TestTableMaxLinesShorthand(t *testing.T) {
	bin := buildBinary(t)
	input := "Name,City\nAlice,\"Tokyo\nOsaka\"\nBob,Kyoto\n"
	stdout := run(t, bin, input, "--output=text", "-L", "7")
	pages := strings.Split(stdout, "<!-- page ")
	if len(pages) != 2 {
		t.Fatalf("expected 2 pages with -L, got %d\n%s", len(pages), stdout)
	}
}

func TestTableMaxRowsTooSmall(t *testing.T) {
	bin := buildBinary(t)
	cmd := exec.Command(bin, "--output=text", "--table-max-rows=1")
	cmd.Stdin = strings.NewReader("Name,City\nAlice,Tokyo\n")
	out, err := cmd.CombinedOutput()
	if err == nil {
		t.Fatalf("expected error for too small --table-max-rows")
	}
	if !strings.Contains(string(out), "at least 2") {
		t.Errorf("unexpected error message: %s", out)
	}
}

func TestTableMaxRowsNegative(t *testing.T) {
	bin := buildBinary(t)
	cmd := exec.Command(bin, "--output=text", "--table-max-rows=-1")
	cmd.Stdin = strings.NewReader("Name,City\nAlice,Tokyo\n")
	out, err := cmd.CombinedOutput()
	if err == nil {
		t.Fatalf("expected error for negative --table-max-rows")
	}
	if !strings.Contains(string(out), "--table-max-rows") {
		t.Errorf("unexpected error message: %s", out)
	}
}

func TestTableMaxLinesTooSmall(t *testing.T) {
	bin := buildBinary(t)
	// text 出力の最低必要行数はヘッダー3 + 下罫線1 + データ1 = 5
	cmd := exec.Command(bin, "--output=text", "--table-max-lines=4")
	cmd.Stdin = strings.NewReader("Name,City\nAlice,Tokyo\n")
	out, err := cmd.CombinedOutput()
	if err == nil {
		t.Fatalf("expected error for too small --table-max-lines")
	}
	if !strings.Contains(string(out), "at least 5") {
		t.Errorf("unexpected error message: %s", out)
	}
}

func TestTableMaxLinesNegative(t *testing.T) {
	bin := buildBinary(t)
	cmd := exec.Command(bin, "--output=text", "--table-max-lines=-1")
	cmd.Stdin = strings.NewReader("Name,City\nAlice,Tokyo\n")
	out, err := cmd.CombinedOutput()
	if err == nil {
		t.Fatalf("expected error for negative --table-max-lines")
	}
	if !strings.Contains(string(out), "--table-max-lines") {
		t.Errorf("unexpected error message: %s", out)
	}
}

// TestMain は go test 時にテストバイナリのエントリポイントとして機能する。
func TestMain(m *testing.M) {
	fmt.Println("running tests...")
	os.Exit(m.Run())
}
