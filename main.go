package main

import (
	"context"
	"embed"
	"flag"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path"
	"strings"
	"time"

	"github.com/BurntSushi/toml"
	"github.com/google/generative-ai-go/genai"
	"github.com/leaanthony/spinner"
	"google.golang.org/api/option"
)

//go:embed transcribe.py
var pythonScript embed.FS

var (
	appDir      string
	debugMode   bool
	verboseMode bool
)

func runCommand(loadingMessage string, args ...string) {
	cmd := exec.Command(args[0], args[1:]...)
	cmd.Dir = appDir

	if verboseMode {
		fmt.Println(loadingMessage)
		fmt.Println("")
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr

		if err := cmd.Run(); err != nil {
			fmt.Println("Command: ", args)
			fmt.Println("Error:", err)
			os.Exit(1)
		}
		fmt.Println("")
	} else {
		myspinner := spinner.New()
		myspinner.Start(loadingMessage)

		stdout, err := cmd.CombinedOutput()
		if err != nil {
			myspinner.Error()
			fmt.Appendln(stdout)
			fmt.Println("Command: ", args)
			fmt.Println("Error:", err)
			os.Exit(1)
		}

		myspinner.Success()
	}
}

func printResponse(resp *genai.GenerateContentResponse) string {
	var text string
	for _, cand := range resp.Candidates {
		if cand.FinishReason != genai.FinishReasonStop {
			fmt.Println("\033[31mFinish reason other than [STOP]\033[0m")
			fmt.Println("Finish reason:", cand.FinishReason)
		}

		if cand.Content != nil {
			for _, part := range cand.Content.Parts {
				// No idea what im doing and why I just can't convert it to string but whatever. It works for now.
				text += fmt.Sprintf("%v", part)
			}
		}
	}
	return text
}

func parseSRT(text string) []string {
	// Split text into lines
	lines := strings.Split(text, "\n")
	var sections []string
	var currentSection []string

	for _, line := range lines {
		trimmedLine := strings.TrimSpace(line)

		// If line is emtpy = end of str section
		if trimmedLine == "" {
			if len(currentSection) > 0 {
				sections = append(sections, strings.Join(currentSection, "\n"))
				currentSection = []string{}
			}
		} else {
			currentSection = append(currentSection, trimmedLine)
		}
	}

	// Add last line if exists
	if len(currentSection) > 0 {
		sections = append(sections, strings.Join(currentSection, "\n"))
	}

	return sections
}

func folderExists(path string) bool {
	info, err := os.Stat(path)
	if err != nil {
		if os.IsNotExist(err) {
			return false
		}
		panic(err)
	}
	return info.IsDir()
}

// https://stackoverflow.com/a/50741908
func moveFile(sourcePath, destPath string) error {
	debugLog("Moving file:", sourcePath, "to destination:", destPath)
	inputFile, err := os.Open(sourcePath)
	if err != nil {
		return fmt.Errorf("Couldn't open source file: %v", err)
	}
	defer inputFile.Close()

	outputFile, err := os.Create(destPath)
	if err != nil {
		return fmt.Errorf("Couldn't open dest file: %v", err)
	}
	defer outputFile.Close()

	_, err = io.Copy(outputFile, inputFile)
	if err != nil {
		return fmt.Errorf("Couldn't copy to dest from source: %v", err)
	}

	inputFile.Close() // for Windows, close before trying to remove: https://stackoverflow.com/a/64943554/246801

	err = os.Remove(sourcePath)
	if err != nil {
		return fmt.Errorf("Couldn't remove source file: %v", err)
	}
	return nil
}

func debugLog(a ...any) {
	if debugMode {
		yellow := "\033[33m"
		reset := "\033[0m"

		fmt.Print(yellow + " [debug] ")
		fmt.Println(a...)
		fmt.Print(reset)
	}
}

func generateConfig() {
	configText := `# Google Gemini API key:
key = "insert-key-here"

# Your CPU threads
threads = "8"

# Chose fast-whisper model
model = "large-v3"
`
	if err := os.WriteFile(path.Join(appDir, "config.toml"), []byte(configText), 0644); err != nil {
		fmt.Println("Error:", err)
		os.Exit(1)
	}
	fmt.Println("Created config file:", path.Join(appDir, "config.toml"))
}

func main() {
	fmt.Println("")
	fmt.Println("  \x1b[30m\x1b[47m\u001b[1mビデオトランスレーター\x1b[0m")
	fmt.Println("  \x1b[2mbideo toransurētā         v0.1.4\x1b[0m")
	fmt.Println("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")
	fmt.Println("")

	// Parse args
	installFlag := flag.Bool("install", false, "Use to install program and needed dependencies in user home folder")
	configFlag := flag.Bool("config", false, "Use to create or reset config file in app directory")
	uninstallFlag := flag.Bool("uninstall", false, "Use to remove program files and its dependencies from user home folder")
	ytdlpFlag := flag.Bool("ytdlp", false, "Download remote video using yt-dlp")
	verboseFlag := flag.Bool("verbose", false, "Print commands output in stdout")
	debugFlag := flag.Bool("debug", false, "Print debug info in stdout")
	flag.Parse()

	if *debugFlag {
		debugMode = true
	}
	if *verboseFlag {
		verboseMode = true
	}

	// path variables
	currentDir, err := os.Getwd()
	if err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
	homeDir, err := os.UserHomeDir()
	if err != nil {
		fmt.Println("Error:", err)
		os.Exit(1)
	}
	appDir = path.Join(homeDir, ".bideo-toransureta")
	debugLog("current dir:", currentDir)
	debugLog("app dir:", appDir)

	// --uninstall
	if *uninstallFlag {
		fmt.Println("Directory will be deleted in 10 seconds:", appDir)
		fmt.Println("Hit ctrl + C to stop.")
		for i := 10; i > 0; i-- {
			fmt.Print(i, " ")
			time.Sleep(time.Second)
		}
		fmt.Println("")

		err := os.RemoveAll(appDir)
		if err != nil {
			fmt.Println("Error:", err)
			os.Exit(1)
		}
		fmt.Println("\nSuccessfully uninstalled.")
		os.Exit(0)
	}

	// --install
	if *installFlag {
		if folderExists(appDir) {
			fmt.Println("Program files already installed, to reinstall first uninstall using --uninstall.")
			os.Exit(1)
		}

		fmt.Println("Starting instalation.")
		// Create application directory
		if err := os.MkdirAll(path.Join(appDir, "tmp"), os.ModePerm); err != nil {
			fmt.Println("Error:", err)
			os.Exit(1)
		}
		debugLog("Created application dir:", appDir)

		runCommand("Creating python venv.", "python", "-m", "venv", "whisper-env")
		runCommand("Installing dependencies.", "whisper-env/bin/pip", "install", "faster-whisper")

		// Extract python script from binary
		myspinner := spinner.New()
		myspinner.Start("Extracting python script from binary.")
		data, err := pythonScript.ReadFile("transcribe.py")
		if err != nil {
			fmt.Println("Error:", err)
			os.Exit(1)
		}
		if err := os.WriteFile(path.Join(appDir, "transcribe.py"), data, 0644); err != nil {
			myspinner.Error()
			fmt.Println("Error:", err)
			os.Exit(1)
		}
		myspinner.Success()

		generateConfig()

		fmt.Println("\nInstallation completed.")
		fmt.Println("TIP: Insert your Google Gemini API key in config file.")
		fmt.Println("Config file location:", path.Join(appDir, "config.toml"))
		os.Exit(0)
	}

	// --config
	if *configFlag {
		generateConfig()
		os.Exit(0)
	}

	if !folderExists(appDir) {
		fmt.Println("Program and dependencies not installed, install them with --install argument.")
		os.Exit(0)
	}

	// Load config file
	type Config struct {
		Key     string
		Threads string
		Model   string
	}
	var config Config
	if _, err := toml.DecodeFile(path.Join(appDir, "config.toml"), &config); err != nil {
		fmt.Println("Config file error:", err)
		os.Exit(1)
	}
	debugLog("-------- Config --------")
	debugLog("cpu threads:", config.Threads)
	debugLog("whisper model:", config.Model)
	debugLog("------------------------")

	if len(flag.Args()) < 1 {
		fmt.Println("Usage: bideo-toransureta [args] <url>")
		fmt.Println("Help:  bideo-toransureta -h")
		os.Exit(0)
	}

	if config.Key == "insert-key-here" {
		fmt.Println("Missing Google Gemini API key in config file.")
		fmt.Println("Config file location:", path.Join(appDir, "config.toml"))
	}

	// Clear tmp dir
	if err := os.RemoveAll(path.Join(appDir, "tmp")); err != nil {
		fmt.Println("Error:", err)
		os.Exit(1)
	}
	if err := os.MkdirAll(path.Join(appDir, "tmp"), os.ModePerm); err != nil {
		fmt.Println("Error:", err)
		os.Exit(1)
	}
	debugLog("Cleared dir:", path.Join(appDir, "tmp"))

	url := flag.Args()[0]

	// Auto detect if url is a link
	if strings.HasPrefix(url, "http://") || strings.HasPrefix(url, "https://") {
		*ytdlpFlag = true
	}

	// Download video
	if *ytdlpFlag {
		ytdlpNameTemplate := "%(title).150B%(title.151B&…|)s [%(display_id)s].%(ext)s"
		cmd := exec.Command("yt-dlp", "-o", ytdlpNameTemplate, "--print", "filename", url)
		output, err := cmd.CombinedOutput()
		if err != nil {
			fmt.Println("yt-dlp error:", err)
			os.Exit(1)
		}
		ytdlpName := path.Base(strings.TrimSpace(string(output)))

		runCommand("Downloading video.", "yt-dlp", "-o", path.Join(currentDir, ytdlpName), url)
		url = ytdlpName
	}

	// Define file names and paths
	name := strings.TrimSuffix(path.Base(url), path.Ext(url))
	isSrtInput := false
	if path.Ext(url) == ".srt" {
		name = strings.TrimSuffix(name, " (transcription)")
		isSrtInput = true
	}
	transcriptionFile := path.Join(appDir, "tmp", name+" (transcription).srt") // this file does not exists if provided argument is .srt file
	translationFile := path.Join(appDir, "tmp", name+".srt")

	// Start transcription
	if path.Ext(url) != ".srt" {
		runCommand("Extracting audio from video file.", "ffmpeg", "-y", "-i", path.Join(currentDir, url), "-q:a", "0", "-map", "a", "tmp/audio.mp3")

		runCommand("Transcription using Whisper AI.", "whisper-env/bin/python", "transcribe.py", name, config.Model, config.Threads, path.Join(appDir, "models"))
		debugLog("Created file:", transcriptionFile)
		url = transcriptionFile

		audioFile := path.Join(appDir, "tmp", "audio.mp3")
		debugLog("Deleting file:", audioFile)
		os.Remove(audioFile)
	}

	// Load .srt file
	transcriptionBuff, err := os.ReadFile(url)
	if err != nil {
		fmt.Println("Error:", err)
		os.Exit(1)
	}
	transcription := string(transcriptionBuff)

	// Init Gemini model
	myspinner := spinner.New()
	if verboseMode {
		fmt.Println("Translation using Google Gemini AI.")
	} else {
		myspinner.Start("Translation using Google Gemini AI.")
	}

	ctx := context.Background()
	client, err := genai.NewClient(ctx, option.WithAPIKey(config.Key))
	if err != nil {
		fmt.Println("Gemini error:", err)
		os.Exit(1)
	}
	defer client.Close()

	model := client.GenerativeModel("gemini-1.5-pro")

	model.SafetySettings = []*genai.SafetySetting{
		{
			Category:  genai.HarmCategoryHarassment,
			Threshold: genai.HarmBlockNone,
		},
		{
			Category:  genai.HarmCategoryHateSpeech,
			Threshold: genai.HarmBlockNone,
		},
		{
			Category:  genai.HarmCategorySexuallyExplicit,
			Threshold: genai.HarmBlockNone,
		},
		{
			Category:  genai.HarmCategoryDangerousContent,
			Threshold: genai.HarmBlockNone,
		},
	}

	cs := model.StartChat()
	cs.History = []*genai.Content{}

	// Split srt into parts
	debugLog("Characters count:", len(transcription))
	subtitles := parseSRT(transcription)
	debugLog("Subtitles sections count:", len(subtitles))

	var parts []string
	var part string
	for _, section := range subtitles {
		part += section + "\n"

		// tokResp, err := model.CountTokens(ctx, genai.Text(part))
		// if err != nil {
		// 	fmt.Println("Gemini API model token count error:", err)
		// 	os.Exit(1)
		// }
		// fmt.Println("total_tokens:", tokResp.TotalTokens)

		if len(part) > 8500 {
			parts = append(parts, part)
			part = ""
		}
	}
	if len(part) > 0 {
		parts = append(parts, part)
	}
	debugLog("Required API requests:", len(parts))

	// Finally make API calls
	var translatedSubtitles string
	for index, section := range parts {
		debugLog("Request #", index+1)
		var prompt string
		if index == 0 {
			prompt = "Translate these SRT subtitles into english. Return them as valid SRT subtitles. Subtitles to translate:\n" + section
		} else {
			prompt = section
		}

		res, err := cs.SendMessage(ctx, genai.Text(prompt))
		if err != nil {
			debugLog("Gemini API error:", err)
			debugLog("Retrying...")
			debugLog("Request #", index+1)
			time.Sleep(90 * time.Second)

			res, err = cs.SendMessage(ctx, genai.Text(prompt))
			if err != nil {
				if !verboseMode {
					myspinner.Error()
				}
				fmt.Println("Gemini API error:", err)
				os.Exit(1)
			}
		}
		translatedSubtitles += printResponse(res)

		if index != 0 {
			time.Sleep(5 * time.Second)
		}
	}
	if verboseMode {
		fmt.Println("Translation done.")
	} else {
		myspinner.Success()
	}

	// Save translation to file
	if err := os.WriteFile(translationFile, []byte(translatedSubtitles), 0644); err != nil {
		fmt.Println("Error:", err)
		os.Exit(1)
	}
	debugLog("Created file: ", translationFile)

	// Move files from temp folder
	if isSrtInput != true {
		if moveFile(transcriptionFile, path.Join(currentDir, name+" (transcription).srt")); err != nil {
			fmt.Println("Error:", err)
		}
	}
	if moveFile(translationFile, path.Join(currentDir, name+".srt")); err != nil {
		fmt.Println("Error:", err)
	}

	fmt.Println("\nSubtitles ready!")
}