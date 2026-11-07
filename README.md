# c2tgo - CSV Table Formatter

A command-line tool that takes CSV data from standard input and formats it into a beautiful table for different platforms, including Markdown, ASCII grid, and Slack.

## Features

- **Multiple Formats**: Support for `md`, `grid`, and `slack` output.
- **Page Splitting**: Automatically splits large tables into pages based on a maximum byte size.
- **Customizable Borders**: Highly configurable border characters for the `grid` and `slack` formats.
- **Cell Wrapping**: Automatically wraps long text within cells while maintaining table structure.
- **Flexible Line Breaks**: Customizable replacement for newlines within CSV cells.

## Installation

To build the application, ensure you have Go installed and run:

```bash
go build -o c2tgo main.go
```

## Usage

The tool reads CSV data from standard input and outputs the formatted table to standard output.

```bash
cat data.csv | ./c2tgo [options]
```

### Basic Examples

**Output as Markdown:**
```bash
cat data.csv | ./c2tgo --format=md
```

**Output as ASCII Grid:**
```bash
cat data.csv | ./c2tgo --format=grid
```

**Output as Slack Message:**
```bash
cat data.csv | ./c2tgo --format=slack
```

## Options

| Flag | Description | Default |
|------|-------------|---------|
| `--format` | Output format: `md`, `grid`, or `slack`. | `md` |
| `--line-break` | String to replace newlines inside CSV cells (Markdown format only). | `<br>` |
| `--top-border` | Single character for top border line (grid/slack only). | `-` |
| `--bottom-border` | Single character for bottom border line (grid/slack only). | `-` |
| `--left-border` | Single character for left border line (grid/slack only). | `\|` |
| `--right-border` | Single character for right border line (grid/slack only). | `\|` |
| `--header-border` | Single character for header/data separator line (grid/slack only). | `=` |
| `--corner-char` | Single character for corners and junction points (grid/slack only). | `+` |
| `--inline-h-border` | Single character for inline horizontal borders (grid/slack only). | `-` |
| `--inline-v-border` | Single character for inline vertical borders (grid/slack only). | `\|` |
| `--cell-max-width` | Maximum display width per cell row. `0` for unlimited, minimum 3. | `0` |
| `--md-code-block` | Wrap each Markdown cell content in code blocks. | `false` |
| `--table-max-byte` | Maximum byte size per page of table output. `0` for unlimited. | `0` |
| `--table-max-line` | Maximum number of data rows (excluding header/borders) per page. `0` for unlimited. | `0` |
| `--page-break` | Page separator template. Use `%page` and `%total` as placeholders. Empty string disables the separator. | `<!-- page %page/%total -->` |

## Formatting Details

- **md**: Standard Markdown table. Supports `<br>` for multi-line content within cells.
- **grid**: A classic ASCII table with borders and headers.
- **slack**: A Slack-friendly format using code blocks (```) to preserve alignment and borders.

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
| Name | Age | City |
| --- | --- | --- |
| Alice | 30 | Tokyo<br>Hokkaido |
| Bob | 25 | Osaka |
| Ken | 43 | Kyoto |
```

```
$ cat data.csv | ./c2tgo --line-break " / "
| Name | Age | City |
| --- | --- | --- |
| Alice | 30 | Tokyo / Hokkaido |
| Bob | 25 | Osaka |
| Ken | 43 | Kyoto |
```

```
$ cat data.csv | ./c2tgo --format grid
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
$ cat data.csv | ./c2tgo --cell-max-width 5
| Name | Age | City |
| --- | --- | --- |
| Alice | 30 | To... |
| Bob | 25 | Osaka |
| Ken | 43 | Kyoto |
```

```
$ cat data.csv | ./c2tgo --format grid --cell-max-width 5
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
$ cat data.csv | ./c2tgo --format grid --table-max-byte 200
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
$ cat data.csv | ./c2tgo -format grid \
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