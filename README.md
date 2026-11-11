# c2tgo - CSV Table Formatter

[🇯🇵 日本語版はこちら](README-ja.md)

A command-line tool that takes CSV data from standard input or a file and formats it into a beautiful table for different platforms, including Markdown, ASCII text table.

## Features

- **Multiple Formats**: Support for `md` and `text` output.
- **Page Splitting**: Automatically splits large tables into pages based on a maximum byte size.
- **Customizable Borders**: Highly configurable border characters for the `text` format.
- **Cell Wrapping**: Automatically wraps long text within cells while maintaining table structure.
- **Flexible Line Breaks**: Customizable replacement for newlines within CSV cells.
- **Sort**: Sort data rows by a specified column.
- **Custom Header**: Provide a header row via command-line option.
- **File Input**: Read CSV data from a file instead of stdin.

## Installation

To build the application, ensure you have Go installed and run:

```bash
go build -o c2tgo .
```

To embed the build date/time and commit hash into `--version`, build with:

```bash
go build -ldflags "-X main.version=$(date +%Y%m%d-%H%M%S) -X main.commit=$(git rev-parse HEAD)" -o c2tgo .
```

## Usage

The tool reads CSV data from standard input (default) or a file and outputs the formatted table to standard output.

```bash
cat data.csv | ./c2tgo [options]
```

Or read from a file:

```bash
./c2tgo --file data.csv [options]
```

### Basic Examples

**Output as Markdown:**
```bash
cat data.csv | ./c2tgo --output=md
```

**Output as ASCII Text Table:**
```bash
cat data.csv | ./c2tgo --output=text
```

**Use shorthand option:**
```bash
cat data.csv | ./c2tgo -o md
```

## Options

| Flag | Description | Default |
|------|-------------|---------|
| `--version`, `-V` | Print the version, revision, OS, and architecture, then exit. | - |
| `--output`, `-o` | Output format: `md` or `text`. | `text` |
| `--line-break` | String to replace newlines inside CSV cells. Default is `<br>` for `md` and no conversion for `text`. For `md`, an empty value emits newlines as additional rows instead of replacing them. | `<br>` (`md` only) |
| `--top-border` | Single character for top border line (text only). Empty string hides the top border. | `-` |
| `--bottom-border` | Single character for bottom border line (text only). Empty string hides the bottom border. | `-` |
| `--left-border` | Single character for left border line (text only). | `\|` |
| `--right-border` | Single character for right border line (text only). | `\|` |
| `--header-border` | Single character for header/data separator line (text only). | `=` |
| `--corner-char` | Single character for corners and junction points (text only). | `+` |
| `--inline-h-border` | Single character for inline horizontal borders (text only). Empty string disables inline horizontal borders. | `-` |
| `--inline-v-border` | Single character for inline vertical borders (text only). | `\|` |
| `--minimal`, `-m` | For `text` output, equivalent to setting `--top-border`, `--bottom-border`, and `--inline-h-border` to empty strings. | `false` |
| `--md-align-border` | For `md` output, align the vertical border (`|`) positions per column. | `false` |
| `--cell-max-width` | Maximum display width per cell row. A single value applies to all columns. A comma-separated list specifies width per column (e.g. `5,6,7`). Use `0` for unlimited per column. Minimum 3. | `0` |
| `--auto-fit`, `-a` | Fit the `text` table width to the terminal width. Ignores `--cell-max-width`. | `false` |
| `--table-max-width`, `-w` | Maximum display width of a single `text` table row. Behaves like `--auto-fit` but uses the given width instead of the terminal width. Mutually exclusive with `--auto-fit`. Ignores `--cell-max-width`. | `0` |
| `--md-code-block` | Wrap each Markdown cell content in code blocks. | `false` |
| `--table-max-byte` | Maximum byte size per page of table output. `0` for unlimited. | `0` |
| `--table-max-rows`, `-R` | Maximum number of rows per page, including the header row. Borders are not counted. `0` for unlimited. | `0` |
| `--table-max-lines`, `-L` | Maximum number of displayed lines per page, including the header, borders, and newlines inside cells. `0` for unlimited. | `0` |
| `--page-break` | Page separator template. Use `%page` and `%total` as placeholders. Empty string disables the separator. | `<!-- page %page/%total -->` |
| `--sort` | Sort data rows by 1-based column number. `0` disables sorting. | `0` |
| `--reverse`, `-r` | Reverse the sort order when `--sort` is specified. | `false` |
| `--header` | CSV string to use as the table header. When set, the first row of input data is treated as data, not header. | `""` |
| `--file`, `-f` | Path to a CSV file to read instead of stdin. | `""` |

> **Note:** `--table-max-line` is deprecated and is now an alias for `--table-max-rows`.

## Formatting Details

- **md**: Standard Markdown table. Supports `<br>` for multi-line content within cells.
- **text**: A classic ASCII table with borders and headers.

## Output examples
 
```
$ cat data.csv
Name,Age,City
Alice,30,"Tokyo
Hokkaido"
Bob,25,Osaka
Ken,43,Kyoto
```

```
$ cat data.csv | ./c2tgo
+-------+-----+----------+
| Name  | Age | City     |
+=======+=====+==========+
| Alice | 30  | Tokyo    |
|       |     | Hokkaido |
+-------+-----+----------+
| Bob   | 25  | Osaka    |
+-------+-----+----------+
| Ken   | 43  | Kyoto    |
+-------+-----+----------+
```

```
$ cat data.csv | ./c2tgo --output=md
| Name | Age | City |
| --- | --- | --- |
| Alice | 30 | Tokyo<br>Hokkaido |
| Bob | 25 | Osaka |
| Ken | 43 | Kyoto |
```

```
$ cat data.csv | ./c2tgo --output=md --line-break " / "
| Name | Age | City |
| --- | --- | --- |
| Alice | 30 | Tokyo / Hokkaido |
| Bob | 25 | Osaka |
| Ken | 43 | Kyoto |
```

```
$ cat data.csv | ./c2tgo --output=text
+-------+-----+----------+
| Name  | Age | City     |
+=======+=====+==========+
| Alice | 30  | Tokyo    |
|       |     | Hokkaido |
+-------+-----+----------+
| Bob   | 25  | Osaka    |
+-------+-----+----------+
| Ken   | 43  | Kyoto    |
+-------+-----+----------+
```

```
$ cat data.csv | ./c2tgo --output=md --cell-max-width 5
| Name | Age | City |
| --- | --- | --- |
| Alice | 30 | To... |
| Bob | 25 | Osaka |
| Ken | 43 | Kyoto |
```

```
$ cat data.csv | ./c2tgo --output=text --cell-max-width 5
+-------+-----+-------+
| Name  | Age | City  |
+=======+=====+=======+
| Alice | 30  | Tokyo |
|       |     | Ho... |
+-------+-----+-------+
| Bob   | 25  | Osaka |
+-------+-----+-------+
| Ken   | 43  | Kyoto |
+-------+-----+-------+
```

```
$ cat data.csv | ./c2tgo --output=text --table-max-byte 200
+-------+-----+----------+
| Name  | Age | City     |
+=======+=====+==========+
| Alice | 30  | Tokyo    |
|       |     | Hokkaido |
+-------+-----+----------+
<!-- page 2/3 -->
+-------+-----+----------+
| Name  | Age | City     |
+=======+=====+==========+
| Bob   | 25  | Osaka    |
+-------+-----+----------+
<!-- page 3/3 -->
+-------+-----+----------+
| Name  | Age | City     |
+=======+=====+==========+
| Ken   | 43  | Kyoto    |
+-------+-----+----------+
```

```
$ cat data.csv | ./c2tgo --output=text --inline-h-border ""
+-------+-----+----------+
| Name  | Age | City     |
+=======+=====+==========+
| Alice | 30  | Tokyo    |
|       |     | Hokkaido |
| Bob   | 25  | Osaka    |
| Ken   | 43  | Kyoto    |
```

```
$ cat data.csv | ./c2tgo --header "Name,Age,City" --sort=1 --output=text
+-------+-----+----------+
| Name  | Age | City     |
+=======+=====+==========+
| Alice | 30  | Tokyo    |
|       |     | Hokkaido |
+-------+-----+----------+
| Bob   | 25  | Osaka    |
+-------+-----+----------+
| Ken   | 43  | Kyoto    |
+-------+-----+----------+
```

```
$ cat data.csv | ./c2tgo -output text \
    -top-border "^" \
    -bottom-border "'" \
    -left-border "[" \
    -right-border "]" \
    -corner-char "@" \
    -inline-h-border "." \
    -inline-v-border "!" \
    -header-border "#"
@^^^^^^^@^^^^^@^^^^^^^^^^@
[ Name  ! Age ! City     ]
@#######@#####@##########@
[ Alice ! 30  ! Tokyo    ]
[       !     ! Hokkaido ]
@.......@.....@..........@
[ Bob   ! 25  ! Osaka    ]
@.......@.....@..........@
[ Ken   ! 43  ! Kyoto    ]
@'''''''@'''''@''''''''''@
```