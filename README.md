# Dev Image Chat

[日本語ドキュメント](README_ja.md)

A tool that automatically generates character images in real time based on your Claude Code conversations and displays them in the browser.

Each time the Claude Code Assistant responds, it reads the conversation content, creates an image generation prompt via the Gemini API, generates an image using an image generation backend (Stable Diffusion or Gemini), and delivers it to the browser.

![Screenshot](assets/ss.jpg)

## Caution

This application uses the Gemini API.
Depending on usage frequency, API costs may become significant, so please monitor your usage regularly.

Be especially careful when using Gemini for image generation. For continuous use, we recommend setting up Stable Diffusion WebUI.

## Requirements

- **Go 1.24 or later**
- **Google Gemini API Key**
  - Available from [Google AI Studio](https://aistudio.google.com/apikey)
  - Used to generate image generation prompts (text)
- **Image Generation Backend** (one of the following)
  - **Gemini** — Ready to use with just a Gemini API key (no additional setup required)
  - **Stable Diffusion WebUI** — Such as AUTOMATIC1111's [stable-diffusion-webui](https://github.com/AUTOMATIC1111/stable-diffusion-webui). Must be launched with the `--api` option to enable the API

## Installation

### 1. Install Go

If Go is not yet installed, use one of the following methods.

**macOS (Homebrew):**

```bash
brew install go
```

**Other platforms:**

Download and install from the [official Go website](https://go.dev/dl/).

After installation, verify the version:

```bash
go version
# Should show go1.24.0 or later
```

### 2. Clone the Repository

```bash
git clone https://github.com/egawata/dev-image-chat.git
cd dev-image-chat
```

### 3. Build

```bash
go build -o dev-image-chat .
```

This creates the `dev-image-chat` executable.

### 4. Create Configuration File

```bash
cp .env.example .env
```

Open the `.env` file and set your Gemini API key in `GEMINI_API_KEY`.

```
GEMINI_API_KEY=your-api-key-here
```

Other settings work with their default values, but can be changed as needed.

## Usage

### With Gemini Backend

Just set the following in `.env` and you're ready to go.

```
GEMINI_API_KEY=your-api-key-here
IMAGE_GENERATOR=gemini
```

```bash
./dev-image-chat
```

### With Stable Diffusion Backend

First, start Stable Diffusion WebUI with the API enabled.

```bash
# In the stable-diffusion-webui directory
./webui.sh --api
```

By default, it starts at `http://localhost:7860`.

Since `IMAGE_GENERATOR` defaults to `sd` in `.env`, you can start directly.

```bash
./dev-image-chat
```

### Verifying Startup

If you see the following log output, the startup was successful.

```
Claude Code Image Chat started
  Web UI: http://localhost:8080
  Watching: /Users/<username>/.claude/projects
  Generate interval: 1m0s
```

### Open the Web UI in Your Browser

Access `http://localhost:8080` to open the image display screen.

Then use Claude Code as usual. Each time the Assistant responds, an image matching the conversation content will be automatically generated and displayed. (There is a 60-second interval by default.)

## Configuration

Settings can be configured via the `.env` file or environment variables.

### Required

| Environment Variable | Description |
|---------------------|-------------|
| `GEMINI_API_KEY` | Google Gemini API key |

### Optional

| Environment Variable | Default | Description |
|---------------------|---------|-------------|
| `IMAGE_GENERATOR` | `sd` | Image generation backend (`sd` or `gemini`) |
| `GEMINI_MODEL` | `gemini-2.5-flash` | Gemini model used for prompt generation |
| `GEMINI_IMAGE_MODEL` | `gemini-2.5-flash-image` | Gemini image generation model (used when `IMAGE_GENERATOR=gemini`) |
| `SD_BASE_URL` | `http://localhost:7860` | Stable Diffusion WebUI URL |
| `SERVER_PORT` | `8080` | Web UI port number |
| `CLAUDE_PROJECTS_DIR` | `~/.claude/projects` | Claude Code projects directory |
| `CHARACTERS_DIR` | `characters` | Directory for character configuration files |
| `CHARACTER_FILE` | *(none)* | Path to character configuration file (fallback when `CHARACTERS_DIR` is empty) |
| `GENERATE_INTERVAL` | `60` | Minimum interval between image generations (seconds) |
| `DEBUG` | `false` | Enable debug logging (`1` or `true`) |

### Stable Diffusion Image Generation Parameters

Effective when `IMAGE_GENERATOR=sd` (default).

| Environment Variable | Default | Description |
|---------------------|---------|-------------|
| `IMGCHAT_SD_STEPS` | `28` | Number of generation steps |
| `IMGCHAT_SD_WIDTH` | `512` | Image width (px) |
| `IMGCHAT_SD_HEIGHT` | `768` | Image height (px) |
| `IMGCHAT_SD_CFG_SCALE` | `5.0` | CFG scale |
| `IMGCHAT_SD_SAMPLER_NAME` | `Euler a` | Sampler name |
| `IMGCHAT_SD_EXTRA_PROMPT` | *(none)* | Additional prompt appended to all images |

## Character Configuration

Place `.md` files in the `characters` directory to reflect character appearance and atmosphere in the generated images. Multiple character files can be placed, and one character is automatically selected per session.

### Placing Character Files (Recommended)

Create `.md` files in the `characters/` directory.

```
characters/
├── chara1.md
└── chara2.md
```

Example configuration file (`characters/chara1.md`):

```markdown
- High school girl (2nd year)
- Height: 165cm
- Hair: Long black hair, straight bangs
- Eye color: Deep brown
- Outfit: School uniform, blazer, red ribbon, black checkered pleated skirt, black socks
- Style: Slender, calm and elegant
- Speech: Energetic manner of speaking, uses polite language
- Location: School classroom
```

We recommend specifying visual characteristics such as hairstyle and clothing in as much detail as possible to maintain a consistent look across images. Specifying the location is also recommended.

The directory can be changed with the `CHARACTERS_DIR` environment variable (default: `characters`).

## Troubleshooting

### `GEMINI_API_KEY is required` is displayed

Check that `GEMINI_API_KEY` is set in the `.env` file.

### Images are not being generated

- Start with `DEBUG=1` to check detailed logs.
- **For Stable Diffusion**: Verify that WebUI is started with the `--api` option and that `SD_BASE_URL` is correct.
- **For Gemini**: Verify that `IMAGE_GENERATOR=gemini` is set and that `GEMINI_API_KEY` is correct.

### Image generation interval is too long

- You can set the `GENERATE_INTERVAL` value in the `.env` file (in seconds).
- The default is 60 seconds, but you may use a shorter value if your environment can generate images quickly.

### Images are not displayed in the browser

- Check that the Web UI (`http://localhost:8080`) is accessible.
- Check the browser developer tools for WebSocket connection errors.

## TODO

- Add support for providers other than Gemini (OpenAI, Anthropic, Grok, etc.)
