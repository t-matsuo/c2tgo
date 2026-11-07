# c2tgo - CSV Table Formatter

CSVデータを標準入力から受け取り、Markdown、ASCIIグリッド、Slackなど、さまざまなプラットフォーム向けに美しいテーブルにフォーマットするコマンドラインツールです。

## 機能

- **複数フォーマット対応**: `md`、`grid`、`slack` の各出力をサポートします。
- **ページ分割**: 最大バイトサイズに基づいて、大きなテーブルを自動的にページに分割します。
- **カスタマイズ可能な罫線**: `grid` および `slack` フォーマットで、罫線文字を細かく設定できます。
- **セル折り返し**: 長いテキストをセル内で自動的に折り返し、テーブル構造を維持します。
- **柔軟な改行処理**: CSVセル内の改行を置き換える文字列をカスタマイズできます。

## インストール

アプリケーションをビルドするには、Goがインストールされていることを確認し、以下を実行してください：

```bash
go build -o c2tgo main.go
```

## 使い方

このツールは、CSVデータを標準入力から読み込み、フォーマットされたテーブルを標準出力に出力します。

```bash
cat data.csv | ./c2tgo [オプション]
```

### 基本的な例

**Markdownとして出力：**
```bash
cat data.csv | ./c2tgo --format=md
```

**ASCIIグリッドとして出力：**
```bash
cat data.csv | ./c2tgo --format=grid
```

**Slackメッセージとして出力：**
```bash
cat data.csv | ./c2tgo --format=slack
```

## オプション

| フラグ | 説明 | デフォルト値 |
|------|-------------|---------|
| `--format` | 出力形式：`md`、`grid`、または `slack`。 | `md` |
| `--line-break` | Markdown形式のみ。CSVセル内の改行を置き換える文字列。 | `<br>` |
| `--top-border` | `grid`/`slack` の上罫線に使用する1文字。 | `-` |
| `--bottom-border` | `grid`/`slack` の下罫線に使用する1文字。 | `-` |
| `--left-border` | `grid`/`slack` の左罫線に使用する1文字。 | `\|` |
| `--right-border` | `grid`/`slack` の右罫線に使用する1文字。 | `\|` |
| `--header-border` | `grid`/`slack` のヘッダー/データ区切り線に使用する1文字。 | `=` |
| `--corner-char` | `grid`/`slack` の角および接合点に使用する1文字。 | `+` |
| `--inline-h-border` | `grid`/`slack` の内部水平罫線に使用する1文字。 | `-` |
| `--inline-v-border` | `grid`/`slack` の内部垂直罫線に使用する1文字。 | `\|` |
| `--cell-max-width` | セル行あたりの最大表示幅。`0` で無制限、最小値は3。 | `0` |
| `--md-code-block` | Markdownの各セルの内容をコードブロックで囲む。 | `false` |
| `--table-max-byte` | テーブル出力1ページあたりの最大バイトサイズ。`0` で無制限。 | `0` |
| `--table-max-line` | 1ページあたりの最大データ行数（ヘッダー・罫線除く）。`0` で無制限。 | `0` |
| `--page-break` | ページ区切り文字列テンプレート。`%page` をページ番号、`%total` を総ページ数に置き換える。空文字列で区切りを無効化。 | `<!-- page %page/%total -->` |

## フォーマットの詳細

- **md**: 標準的なMarkdownテーブル。セル内の複数行コンテンツに `<br>` をサポートします。
- **grid**: 罫線とヘッダー付きのクラシックなASCIIテーブルです。
- **slack**: コードブロック（```）を使用して、配置と罫線を保持するSlack向けフォーマットです。

## 出力例
 
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