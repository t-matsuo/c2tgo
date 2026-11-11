# c2tgo - CSV Table Formatter

[🇺🇸 English version is here](README.md)

CSVデータを標準入力またはファイルから受け取り、Markdown、ASCIIテキストテーブルなど、さまざまなプラットフォーム向けに美しいテーブルにフォーマットするコマンドラインツールです。

## 機能

- **複数フォーマット対応**: `md`、`text` の各出力をサポートします。
- **ページ分割**: 最大バイトサイズに基づいて、大きなテーブルを自動的にページに分割します。
- **カスタマイズ可能な罫線**: `text` フォーマットで、罫線文字を細かく設定できます。
- **セル折り返し**: 長いテキストをセル内で自動的に折り返し、テーブル構造を維持します。
- **柔軟な改行処理**: CSVセル内の改行を置き換える文字列をカスタマイズできます。
- **ソート**: 指定した列でデータ行をソートできます。
- **カスタムヘッダー**: コマンドラインオプションでヘッダー行を指定できます。
- **ファイル入力**: 標準入力の代わりにCSVファイルを読み込めます。

## インストール

アプリケーションをビルドするには、Goがインストールされていることを確認し、以下を実行してください：

```bash
go build -o c2tgo .
```

ビルド日時とコミットハッシュを `--version` に埋め込む場合は、以下のようにビルドしてください：

```bash
go build -ldflags "-X main.version=$(date +%Y%m%d-%H%M%S) -X main.commit=$(git rev-parse HEAD)" -o c2tgo .
```

## 使い方

このツールは、CSVデータを標準入力（デフォルト）またはファイルから読み込み、フォーマットされたテーブルを標準出力に出力します。

```bash
cat data.csv | ./c2tgo [オプション]
```

ファイルから読み込む場合：

```bash
./c2tgo --file data.csv [オプション]
```

### 基本的な例

**Markdownとして出力：**
```bash
cat data.csv | ./c2tgo --output=md
```

**ASCIIテキストテーブルとして出力：**
```bash
cat data.csv | ./c2tgo --output=text
```

**短縮オプションを使用：**
```bash
cat data.csv | ./c2tgo -o md
```

## オプション

| フラグ | 説明 | デフォルト値 |
|------|-------------|---------|
| `--version`, `-V` | バージョン、リビジョン、OS、Arch を表示して終了します。 | - |
| `--output`, `-o` | 出力形式：`md` または `text`。 | `text` |
| `--line-break` | CSVセル内の改行を置き換える文字列。`md` のデフォルトは `<br>`、`text` のデフォルトは変換しない。`md` で空文字を指定すると、改行を新しい行として出力する。 | `<br>`（`md` のみ） |
| `--top-border` | `text` の上罫線に使用する1文字。空文字列で上罫線を非表示。 | `-` |
| `--bottom-border` | `text` の下罫線に使用する1文字。空文字列で下罫線を非表示。 | `-` |
| `--left-border` | `text` の左罫線に使用する1文字。 | `\|` |
| `--right-border` | `text` の右罫線に使用する1文字。 | `\|` |
| `--header-border` | `text` のヘッダー/データ区切り線に使用する1文字。 | `=` |
| `--corner-char` | `text` の角および接合点に使用する1文字。 | `+` |
| `--inline-h-border` | `text` の内部水平罫線に使用する1文字。空文字列で内部水平罫線を無効化。 | `-` |
| `--inline-v-border` | `text` の内部垂直罫線に使用する1文字。 | `\|` |
| `--minimal`, `-m` | `text` 出力時に上罫線・下罫線・内部水平罫線を空文字に指定したのと同じ効果にする。 | `false` |
| `--md-align-border` | `md` 出力時に各列で縦の罫線（`|`）の位置を揃える。 | `false` |
| `--cell-max-width` | セル行あたりの最大表示幅。値が1つの場合は全列に適用。カンマ区切りで各列の幅を指定可能（例：`5,6,7`）。`0` は未制限を表す。最小値は3。 | `0` |
| `--auto-fit`, `-a` | `text` 出力時にテーブル幅をターミナル幅に合わせる。`--cell-max-width` は無視される。 | `false` |
| `--table-max-width`, `-w` | `text` の表1行あたりの最大表示幅。`--auto-fit` と同じ動作をしますが、ターミナル幅の代わりに指定した幅を使用します。`--auto-fit` と排他。`--cell-max-width` は無視される。 | `0` |
| `--md-code-block` | Markdownの各セルの内容をコードブロックで囲む。 | `false` |
| `--table-max-byte` | テーブル出力1ページあたりの最大バイトサイズ。`0` で無制限。 | `0` |
| `--table-max-rows`, `-R` | 1ページあたりの最大行数。ヘッダー行もカウント対象。罫線はカウントしない。`0` で無制限。 | `0` |
| `--table-max-lines`, `-L` | 1ページあたりの最大表示行数。ヘッダー・罫線・セル内の改行や折り返し後の行数をすべて考慮する。`0` で無制限。 | `0` |
| `--page-break` | ページ区切り文字列テンプレート。`%page` をページ番号、`%total` を総ページ数に置き換える。空文字列で区切りを無効化。 | `<!-- page %page/%total -->` |
| `--sort` | 1始まりの列番号でデータ行をソート。`0` でソート無効。 | `0` |
| `--reverse`, `-r` | `--sort` 指定時にソート順を逆にする。 | `false` |
| `--header` | テーブルヘッダーとして使用するCSV文字列。指定時、入力データの1行目はヘッダーではなくデータとして扱われる。 | `""` |
| `--file`, `-f` | 標準入力の代わりに読み込むCSVファイルのパス。 | `""` |

> **注:** `--table-max-line` は非推奨となり、`--table-max-rows` のエイリアスになりました。

## フォーマットの詳細

- **md**: 標準的なMarkdownテーブル。セル内の複数行コンテンツに `<br>` をサポートします。
- **text**: 罫線とヘッダー付きのクラシックなASCIIテーブルです。

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