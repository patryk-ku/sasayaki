# Sasayaki

A small CLI tool for automating video transcription and translation using AI. Written in Go, it uses [**faster-whisper**](https://github.com/SYSTRAN/faster-whisper) for transcription and translation (optional translation with Google Gemini). Just enter the video link or file path to get translated subtitles in .srt format.

The name Sasayaki (ささやき) means "whisper" in Japanese.

## Requirements

For now, this tool only works on Linux and requires these packages to be installed:

-   python
-   ffmpeg
-   yt-dlp

## Installation

Download the latest executable from the [Releases](https://github.com/patryk-ku/sasayaki/releases) page.

```sh
chmod +x sasayaki
./sasayaki --install
```

Optional:

Open `config.toml` and insert here your Gemini API key.

## Usage

```sh
./sasayaki [args] <url>
```

Possible urls:

-   yt-dlp compatibile url
-   local video or audio file
-   .srt file created by this program

Available args:

```
  --config
        Use to create or reset config file
  --debug
        Print debug info in stdout
  -gemini
        Translate using Google Gemini instead of Whisper
  --install
        Use to install program and needed dependencies in user home folder
  --uninstall
        Use to remove program files and its dependencies from user home folder
  --verbose
        Print commands output in stdout
  --ytdlp
        Download remote video using yt-dlp
```

## Compile from source

```sh
go mod tidy
go build -ldflags "-w -s"
```
