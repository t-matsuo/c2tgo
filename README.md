# c2tgo - CSV Table Formatter

A command-line tool that takes CSV data from standard input or a file and formats it into a beautiful table for different platforms, including Markdown, ASCII text table, and Slack.

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
go build -o c2tgo main.go
```

To embed the build date/time into `--version`, build with:

```bash
go build -ldflags "-X main.version=$(date +%Y%m%d-%H%M%S)" -o c2tgo main.go
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
| `--version` | Print the version number and exit. | - |
| `--output`, `-o` | Output format: `md` or `text`. | `text` |
| `--line-break` | String to replace newlines inside CSV cells. Default is `<br>` for `md` and no conversion for `text`. | `<br>` (`md` only) |
| `--top-border` | Single character for top border line (text only). | `-` |
| `--bottom-border` | Single character for bottom border line (text only). | `-` |
| `--left-border` | Single character for left border line (text only). | `\|` |
| `--right-border` | Single character for right border line (text only). | `\|` |
| `--header-border` | Single character for header/data separator line (text only). | `=` |
| `--corner-char` | Single character for corners and junction points (text only). | `+` |
| `--inline-h-border` | Single character for inline horizontal borders (text only). Empty string disables inline horizontal borders. | `-` |
| `--inline-v-border` | Single character for inline vertical borders (text only). | `\|` |
| `--cell-max-width` | Maximum display width per cell row. A single value applies to all columns. A comma-separated list specifies width per column (e.g. `5,6,7`). Use `0` for unlimited per column. Minimum 3. | `0` |
| `--md-code-block` | Wrap each Markdown cell content in code blocks. | `false` |
| `--table-max-byte` | Maximum byte size per page of table output. `0` for unlimited. | `0` |
| `--table-max-line` | Maximum number of data rows (excluding header/borders) per page. `0` for unlimited. | `0` |
| `--page-break` | Page separator template. Use `%page` and `%total` as placeholders. Empty string disables the separator. | `<!-- page %page/%total -->` |
| `--sort` | Sort data rows by 1-based column number. `0` disables sorting. | `0` |
| `--header` | CSV string to use as the table header. When set, the first row of input data is treated as data, not header. | `""` |
| `--file`, `-f` | Path to a CSV file to read instead of stdin. | `""` |

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