# Dev Image Chat

Claude Code での会話内容に合わせて、リアルタイムにキャラクター画像を自動生成し、ブラウザに表示するツールです。

Claude Code の Assistant が応答するたびに、会話の内容を読み取り、Gemini API で画像生成プロンプトを作成、画像生成バックエンド（Stable Diffusion または Gemini）で画像を生成してブラウザに配信します。

![スクリーンショット](assets/ss.jpg)

## 注意

このアプリケーションは Gemini API を使用します。
利用頻度によっては API 利用料金が高額になる可能性があるため、利用状況をご自身で定期的にチェックしてください。

特に画像生成を Gemini で行う場合にご注意ください。継続使用する場合は Stable Duffision WebUI の導入を推奨します。

## 必要なもの

- **Go 1.24 以上**
- **Google Gemini API キー**
  - [Google AI Studio](https://aistudio.google.com/apikey) から取得できます
  - 画像生成プロンプト(文字列)を生成するために使います
- **画像生成バックエンド**（以下のいずれか）
  - **Gemini** — Gemini API キーがあればすぐに使えます（追加セットアップ不要）
  - **Stable Diffusion WebUI** — AUTOMATIC1111 の [stable-diffusion-webui](https://github.com/AUTOMATIC1111/stable-diffusion-webui) など。`--api` オプション付きで起動し、API が有効になっていること

## インストール

### 1. Go のインストール

Go がまだインストールされていない場合は、以下のいずれかの方法でインストールしてください。

**macOS (Homebrew):**

```bash
brew install go
```

**その他の環境:**

[Go 公式サイト](https://go.dev/dl/) からダウンロードしてインストールしてください。

インストール後、バージョンを確認します。

```bash
go version
# go1.24.0 以上が表示されればOK
```

### 2. リポジトリの取得

```bash
git clone https://github.com/egawata/dev-image-chat.git
cd dev-image-chat
```

### 3. ビルド

```bash
go build -o dev-image-chat .
```

`dev-image-chat` という実行ファイルが作成されます。

### 4. 設定ファイルの作成

```bash
cp .env.example .env
```

`.env` ファイルを開いて、`GEMINI_API_KEY` に Gemini API キーを設定します。

```
GEMINI_API_KEY=your-api-key-here
```

その他の設定はデフォルト値のままで動作しますが、必要に応じて変更できます。

## 起動方法

### Gemini バックエンドの場合

`.env` に以下を設定するだけで、すぐに使えます。

```
GEMINI_API_KEY=your-api-key-here
IMAGE_GENERATOR=gemini
```

```bash
./dev-image-chat
```

### Stable Diffusion バックエンドの場合

まず Stable Diffusion WebUI を API 有効の状態で起動してください。

```bash
# stable-diffusion-webui のディレクトリで
./webui.sh --api
```

デフォルトで `http://localhost:7860` で起動します。

`.env` の `IMAGE_GENERATOR` はデフォルトで `sd` なので、そのまま起動できます。

```bash
./dev-image-chat
```

### 起動確認

以下のようなログが出れば起動成功です。

```
Claude Code Image Chat started
  Web UI: http://localhost:8080
  Watching: /Users/<username>/.claude/projects
  Generate interval: 1m0s
```

### ブラウザで Web UI を開く

`http://localhost:8080` にアクセスすると、画像表示画面が開きます。

あとは普段通り Claude Code を使ってください。Assistant が応答するたびに、会話内容に合った画像が自動的に生成・表示されます。(デフォルトでは60秒のインターバルがあります)

## 設定項目

`.env` ファイルまたは環境変数で設定できます。

### 必須

| 環境変数 | 説明 |
|---------|------|
| `GEMINI_API_KEY` | Google Gemini API キー |

### オプション

| 環境変数 | デフォルト | 説明 |
|---------|----------|------|
| `IMAGE_GENERATOR` | `sd` | 画像生成バックエンド（`sd` or `gemini`） |
| `GEMINI_MODEL` | `gemini-2.5-flash` | プロンプト生成に使用する Gemini モデル |
| `GEMINI_IMAGE_MODEL` | `gemini-2.5-flash-image` | Gemini 画像生成モデル（`IMAGE_GENERATOR=gemini` 時に使用） |
| `SD_BASE_URL` | `http://localhost:7860` | Stable Diffusion WebUI の URL |
| `SERVER_PORT` | `8080` | Web UI のポート番号 |
| `CLAUDE_PROJECTS_DIR` | `~/.claude/projects` | Claude Code のプロジェクトディレクトリ |
| `CHARACTERS_DIR` | `characters` | キャラクター設定ファイルのディレクトリ |
| `CHARACTER_FILE` | *(なし)* | キャラクター設定ファイルのパス（`CHARACTERS_DIR` が空の場合のフォールバック） |
| `GENERATE_INTERVAL` | `60` | 画像生成の最小間隔（秒） |
| `DEBUG` | `false` | デバッグログの有効化（`1` or `true`） |

### Stable Diffusion 画像生成パラメータ

`IMAGE_GENERATOR=sd`（デフォルト）のときに有効です。

| 環境変数 | デフォルト | 説明 |
|---------|----------|------|
| `IMGCHAT_SD_STEPS` | `28` | 生成ステップ数 |
| `IMGCHAT_SD_WIDTH` | `512` | 画像の幅（px） |
| `IMGCHAT_SD_HEIGHT` | `768` | 画像の高さ（px） |
| `IMGCHAT_SD_CFG_SCALE` | `5.0` | CFG スケール |
| `IMGCHAT_SD_SAMPLER_NAME` | `Euler a` | サンプラー名 |
| `IMGCHAT_SD_EXTRA_PROMPT` | *(なし)* | 全画像に追加するプロンプト |

## キャラクター設定

`characters` ディレクトリに `.md` ファイルを配置すると、生成される画像にキャラクターの外見や雰囲気を反映させることができます。複数のキャラクターファイルを配置でき、セッションごとに1つのキャラクターが自動的に選ばれます。

### キャラクターファイルの配置（推奨）

`characters/` ディレクトリに `.md` ファイルを作成します。

```
characters/
├── chara1.md
└── chara2.md
```

設定ファイルの例（`characters/chara1.md`）:

```markdown
- 女子高校生(2年)
- 身長: 165cm
- 髪型: 黒髪ロング、前髪ぱっつん
- 瞳の色: 深い茶色
- 服装: 学校制服、ブレザー、赤いリボン、黒のチェック入りのプリーツスカート、黒ソックス
- スタイル: スレンダー、落ち着いた清楚系
- 話し方: 元気な話し方。丁寧語を使う
- 場所: 学校の教室
```

髪型、服装などの外見的特徴をなるべく細かく指定すると、画像ごとの雰囲気に統一感が出るのでおすすめです。場所も指定したほうがいいでしょう。

ディレクトリは `CHARACTERS_DIR` 環境変数で変更できます（デフォルト: `characters`）。

## トラブルシューティング

### `GEMINI_API_KEY is required` と表示される

`.env` ファイルに `GEMINI_API_KEY` が設定されているか確認してください。

### 画像が生成されない

- `DEBUG=1` で起動して詳細ログを確認してください。
- **Stable Diffusion の場合**: WebUI が `--api` オプション付きで起動しているか、`SD_BASE_URL` が正しいか確認してください。
- **Gemini の場合**: `IMAGE_GENERATOR=gemini` が設定されているか、`GEMINI_API_KEY` が正しいか確認してください。

### 画像の生成間隔が長い

- `.env` ファイル内で `GENERATE_INTERVAL` の値を指定できます。(単位は秒)
- デフォルトは60秒ですが、高速に画像生成できる環境をお使いならもっと短い値でもいいかもしれません。

### ブラウザに画像が表示されない

- Web UI (`http://localhost:8080`) が開けるか確認してください。
- ブラウザの開発者ツールで WebSocket 接続エラーがないか確認してください。

## TODO

- Gemini 以外 (OpenAI, Anthropic, Grok...) も選択可能にする
- セッションごとに画面を分けて表示
  - 複数セッションを並列で動かして画面1つだとシーンが混ざって見づらい
