package main

import (
	"context"
	"embed"
	"errors"
	"flag"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path"
	"runtime"
	"strings"
	"time"

	"github.com/BurntSushi/toml"
	"github.com/google/generative-ai-go/genai"
	"github.com/leaanthony/spinner"
	"google.golang.org/api/option"
)

//go:embed embed
var embedFS embed.FS

var (
	appDir            string
	debugMode         bool
	verboseMode       bool
	commandCurrentDir bool
)

var (
	redANSI    = "\033[31m"
	yellowANSI = "\033[33m"
	invertANSI = "\033[7m"
	dimANSI    = "\033[2m"
	resetANSI  = "\033[0m"
)

func debugLog(a ...any) {
	if debugMode {
		fmt.Print(yellowANSI + " [debug] ")
		fmt.Println(a...)
		fmt.Print(resetANSI)
	}
}

func printError(err error) {
	fmt.Println(redANSI+"Error:", err, resetANSI)
}

func runCommand(loadingMessage string, args ...string) {
	cmd := exec.Command(args[0], args[1:]...)
	if commandCurrentDir {
		cmd.Dir = appDir
	}

	if verboseMode {
		fmt.Println(invertANSI, "━━━", loadingMessage, "━━━", resetANSI)
		fmt.Println("")
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr

		if err := cmd.Run(); err != nil {
			fmt.Println("Command failed:")
			fmt.Println(strings.Join(args, " "))
			printError(err)
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
			fmt.Println("Command failed:")
			fmt.Println(strings.Join(args, " "))
			printError(err)
			os.Exit(1)
		}

		myspinner.Success()
	}
}

func printResponse(resp *genai.GenerateContentResponse) string {
	var text string
	for _, cand := range resp.Candidates {
		if cand.FinishReason != genai.FinishReasonStop {
			fmt.Println(redANSI + "Finish reason other than [STOP]" + resetANSI)
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

func fileExists(filePath string) bool {
	_, error := os.Stat(filePath)
	return !errors.Is(error, os.ErrNotExist)
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

// https://gophercoding.com/download-a-file/
func downloadFile(url string, filepath string) error {
	// Get the data
	resp, err := http.Get(url)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	// Check response code
	if resp.StatusCode != http.StatusOK {
		message := fmt.Sprintf("HTTP Error: %d", resp.StatusCode)
		return errors.New(message)
	}

	// Create the file
	out, err := os.Create(filepath)
	if err != nil {
		return err
	}
	defer out.Close()

	// Write the body to file
	_, err = io.Copy(out, resp.Body)
	return err
}

func generateConfig() {
	configText := `# Google Gemini API key:
key = "insert-key-here"

# Your CPU threads
threads = "8"

# Chose whisper model
# example: large-v3, medium, small, tiny
model = "medium"

# Force usage of whisper.cpp version without --cpp argument
# Enabled by default on Windows regardless of this setting
cpp = false
`
	if err := os.WriteFile(path.Join(appDir, "config.toml"), []byte(configText), 0644); err != nil {
		printError(err)
		os.Exit(1)
	}
	fmt.Println("Created config file:", path.Join(appDir, "config.toml"))
}

func main() {
	fmt.Println("")
	fmt.Println(" ", invertANSI, "ささやき", resetANSI)
	fmt.Println(" ", dimANSI, "sasayaki           v0.1.10", resetANSI)
	fmt.Println("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")
	fmt.Println("")

	// Parse args
	installFlag := flag.Bool("install", false, "Use to install program and needed dependencies in user home folder")
	configFlag := flag.Bool("config", false, "Use to create or reset config file")
	uninstallFlag := flag.Bool("uninstall", false, "Use to remove program files and its dependencies from user home folder")
	ytdlpFlag := flag.Bool("ytdlp", false, "Download remote video using yt-dlp")
	verboseFlag := flag.Bool("verbose", false, "Print commands output in stdout")
	debugFlag := flag.Bool("debug", false, "Print debug info in stdout")
	geminiFlag := flag.Bool("gemini", false, "Translate using Google Gemini instead of Whisper")
	langFlag := flag.String("lang", "english", "Specifies a target translation language when using Google Gemini")
	cppFlag := flag.Bool("cpp", false, "Transcribe using whisper.cpp instead of faster-whisper (enabled by default on Windows)")
	modelFlag := flag.String("model", "", "Chose whisper model")
	flag.Parse()

	if *debugFlag {
		debugMode = true
	}
	if *verboseFlag {
		verboseMode = true
	}
	action := "translate"
	if *geminiFlag {
		action = "transcribe"
	}

	// Detect OS and set OS specific variables
	var whisperCppFile string
	if runtime.GOOS == "windows" {
		whisperCppFile = "whisper-cli.exe"
		// Force usage of whisper.cpp on Windows
		*cppFlag = true
	} else {
		whisperCppFile = "whisper-cli"
	}

	// path variables
	currentDir, err := os.Getwd()
	if err != nil {
		printError(err)
		os.Exit(1)
	}
	homeDir, err := os.UserHomeDir()
	if err != nil {
		printError(err)
		os.Exit(1)
	}
	appDir = path.Join(homeDir, ".sasayaki")
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
			printError(err)
			os.Exit(1)
		}
		fmt.Println("\nSuccessfully uninstalled.")
		os.Exit(0)
	}

	// --install
	if *installFlag {
		// if folderExists(appDir) {
		// 	fmt.Println("Program files already installed, to reinstall first uninstall using --uninstall.")
		// 	os.Exit(1)
		// }
		commandCurrentDir = true

		fmt.Println("Starting instalation.")
		// Create application directory
		if err := os.MkdirAll(path.Join(appDir, "tmp"), os.ModePerm); err != nil {
			printError(err)
			os.Exit(1)
		}
		debugLog("Created application dir:", appDir)
		// Create models directory
		if err := os.MkdirAll(path.Join(appDir, "models"), os.ModePerm); err != nil {
			printError(err)
			os.Exit(1)
		}

		// Install python version or c++ version
		if *cppFlag {
			myspinner := spinner.New()
			myspinner.Start("Extracting whisper.cpp from binary.")
			whisperCli, err := embedFS.ReadFile("embed/" + whisperCppFile)
			if err != nil {
				myspinner.Error()
				printError(err)
				os.Exit(1)
			}
			if err := os.WriteFile(path.Join(appDir, whisperCppFile), whisperCli, 0644); err != nil {
				myspinner.Error()
				printError(err)
				os.Exit(1)
			}
			myspinner.Success()

			if runtime.GOOS != "windows" {
				runCommand("Granting execution permissions for whisper.cpp executable. ", "chmod", "+x", "whisper-cli")
			}

		} else {
			runCommand("Installing correct Python version using pyenv.", "pyenv", "install", "3.12", "-s")
			runCommand("Setting local Python version.", "pyenv", "local", "3.12")
			runCommand("Creating Python venv.", "pyenv", "exec", "python", "-m", "venv", path.Join(appDir, "whisper-env"))
			runCommand("Installing dependencies.", path.Join(appDir, "whisper-env", "bin", "pip"), "install", "faster-whisper")

			// Extract python script from binary
			myspinner := spinner.New()
			myspinner.Start("Extracting python script from binary.")
			pythonScript, err := embedFS.ReadFile("embed/transcribe.py")
			if err != nil {
				printError(err)
				os.Exit(1)
			}
			if err := os.WriteFile(path.Join(appDir, "transcribe.py"), pythonScript, 0644); err != nil {
				myspinner.Error()
				printError(err)
				os.Exit(1)
			}
			myspinner.Success()
		}

		generateConfig()

		fmt.Println("\nInstallation completed.")
		fmt.Println("TIP: Insert your Google Gemini API key in config file.")
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
		Cpp     bool
	}
	var config Config
	if _, err := toml.DecodeFile(path.Join(appDir, "config.toml"), &config); err != nil {
		fmt.Println("Config file error.")
		printError(err)
		os.Exit(1)
	}
	debugLog("-------- Config --------")
	debugLog("cpu threads:", config.Threads)
	debugLog("whisper model:", config.Model)
	debugLog("------------------------")

	if len(flag.Args()) < 1 {
		fmt.Println("Usage: sasayaki [args] <url>")
		fmt.Println("Help:  sasayaki -h")
		os.Exit(0)
	}

	if (config.Key == "insert-key-here") && *geminiFlag {
		printError(errors.New("Missing Google Gemini API key in config file."))
		fmt.Println("Config file location:", path.Join(appDir, "config.toml"))
		os.Exit(1)
	}

	if *modelFlag != "" {
		config.Model = *modelFlag
	}

	if config.Cpp {
		*cppFlag = true
	}

	if *cppFlag {
		if !fileExists(path.Join(appDir, whisperCppFile)) {
			printError(errors.New("whisper.cpp binary not found."))
			fmt.Println("TIP: You can install it using: --cpp --install arguments. Warning: This will overwrite your config file with default one.")
			os.Exit(1)
		}
	}

	// Download whisper.cpp model if --cpp enabled
	if *cppFlag {
		modelName := "ggml-" + config.Model + ".bin"
		modelPath := path.Join(appDir, "models", modelName)

		if !fileExists(modelPath) {
			myspinner := spinner.New()
			myspinner.Start("Downloading whisper.cpp model (" + modelName + ").")

			err := downloadFile("https://huggingface.co/ggerganov/whisper.cpp/resolve/main/"+modelName, modelPath)
			if err != nil {
				myspinner.Error()
				printError(err)
				os.Exit(1)
			}
			myspinner.Success()
		}
	}

	// Clear tmp dir
	if err := os.RemoveAll(path.Join(appDir, "tmp")); err != nil {
		printError(err)
		os.Exit(1)
	}
	if err := os.MkdirAll(path.Join(appDir, "tmp"), os.ModePerm); err != nil {
		printError(err)
		os.Exit(1)
	}
	debugLog("Cleared dir:", path.Join(appDir, "tmp"))

	url := flag.Args()[0]

	var (
		downloadUrl         string // --ytdlp
		videoInput          string
		videoOutput         string // only if downloading video with yt-dlp
		videoTmp            string // --ytdlp, tmp video file awaiting for translated subs
		srtInput            string // only if translating .srt transcription file again
		srtTmp              string // tmp file from python script, might be transcription or translation
		srtTranslatedTmp    string // tmp file from Google Gemini, might be only translation
		srtOutput           string // output file with transcription
		srtTranslatedOutput string // output file with translated subtitles
		outputDir           string // generated files final destination
		fileName            string // name of input file without exctension
	)

	// Auto detect if url is a link
	if strings.HasPrefix(url, "http://") || strings.HasPrefix(url, "https://") {
		*ytdlpFlag = true
		downloadUrl = url
	}

	// Download video
	if *ytdlpFlag {
		ytdlpNameTemplate := "%(title).150B%(title.151B&…|)s [%(display_id)s].%(ext)s"
		cmd := exec.Command("yt-dlp", "--windows-filenames", "--remux-video", "mkv", "-o", ytdlpNameTemplate, "--print", "filename", url)

		// Tmp fix for Windows cmd output not in utf-8
		if runtime.GOOS == "windows" {
			cmd.Args = append(cmd.Args, "--restrict-filenames")
		}

		output, err := cmd.CombinedOutput()
		if err != nil {
			fmt.Println("yt-dlp error.")
			printError(err)
			os.Exit(1)
		}
		ytdlpName := path.Base(strings.TrimSpace(string(output)))
		ytdlpName = strings.TrimSuffix(path.Base(ytdlpName), path.Ext(ytdlpName))
		ytdlpName = ytdlpName + ".mkv"
		videoTmp = path.Join(appDir, "tmp", ytdlpName)

		runCommand("Downloading video.", "yt-dlp", "--remux-video", "mkv", "-o", videoTmp, downloadUrl)
		videoInput = videoTmp
	}

	// Define file names and paths
	isSrtInput := false
	if path.Ext(url) == ".srt" {
		isSrtInput = true
		*geminiFlag = true
		srtInput = url
		fileName = strings.TrimSuffix(path.Base(srtInput), path.Ext(srtInput))
		fileName = strings.TrimSuffix(fileName, " (transcription)")
	} else {
		if !*ytdlpFlag {
			videoInput = url
		}
		fileName = strings.TrimSuffix(path.Base(videoInput), path.Ext(videoInput))
	}
	srtTmp = path.Join(appDir, "tmp", fileName+" (transcription).srt")
	nameForCppExecutable := path.Join(appDir, "tmp", fileName+" (transcription)") // without extension
	srtTranslatedTmp = path.Join(appDir, "tmp", fileName+".srt")

	// Start transcription
	if path.Ext(url) != ".srt" {
		audioFile := path.Join(appDir, "tmp", "audio.wav")
		// ffmpeg -i <video> -ar 16000 -ac 1 -c:a pcm_s16le output.wav
		runCommand("Extracting audio from video file.", "ffmpeg", "-y", "-i", videoInput, "-ar", "16000", "-ac", "1", "-c:a", "pcm_s16le", audioFile)

		if *cppFlag {
			translate := "false"
			if action == "translate" {
				translate = "true"
			}

			// TODO: --prompt
			runCommand("Transcription using whisper.cpp.", path.Join(appDir, whisperCppFile), "--threads", config.Threads, "--translate", translate, "--output-srt", "--output-file", nameForCppExecutable, "--language", "auto", "--model", path.Join(appDir, "models", "ggml-"+config.Model+".bin"), "--file", audioFile)
		} else {
			runCommand("Transcription using faster-whisper.", path.Join(appDir, "whisper-env", "bin", "python"), path.Join(appDir, "transcribe.py"), "--output", srtTmp, "--model", config.Model, "--threads", config.Threads, "--appdir", path.Join(appDir, "models"), "--action", action, "--input", audioFile)
		}
		debugLog("Created file:", srtTmp)

		debugLog("Deleting file:", audioFile)
		os.Remove(audioFile)
	}

	// Load .srt file
	var fileToRead string
	if isSrtInput {
		fileToRead = srtInput
	} else {
		fileToRead = srtTmp
	}

	transcriptionBuff, err := os.ReadFile(fileToRead)
	if err != nil {
		printError(err)
		os.Exit(1)
	}
	transcription := string(transcriptionBuff)

	if *geminiFlag {
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
			fmt.Println("Gemini error.")
			printError(err)
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
		debugLog("Translation language:", *langFlag)
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
				prompt = "Translate these SRT subtitles into " + *langFlag + ". Return them as valid SRT subtitles. Subtitles to translate:\n" + section
			} else {
				prompt = section
			}

			res, err := cs.SendMessage(ctx, genai.Text(prompt))
			if err != nil {
				debugLog("Gemini API error.")
				printError(err)
				debugLog("Retrying...")
				debugLog("Request #", index+1)
				time.Sleep(90 * time.Second)

				res, err = cs.SendMessage(ctx, genai.Text(prompt))
				if err != nil {
					if !verboseMode {
						myspinner.Error()
					}
					fmt.Println("Gemini API error.")
					printError(err)
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
		if err := os.WriteFile(srtTranslatedTmp, []byte(translatedSubtitles), 0644); err != nil {
			printError(err)
			os.Exit(1)
		}
		debugLog("Created file: ", srtTranslatedTmp)
	}

	// Move files from temp folder
	if *ytdlpFlag {
		outputDir = currentDir
	} else if isSrtInput {
		outputDir = path.Dir(srtInput)
	} else {
		outputDir = path.Dir(videoInput)
	}

	srtOutput = path.Join(outputDir, fileName+" (transcription).srt")
	srtTranslatedOutput = path.Join(outputDir, fileName+".srt")
	videoOutput = path.Join(outputDir, fileName+".mkv")

	if isSrtInput == true {
		if moveFile(srtTranslatedTmp, srtTranslatedOutput); err != nil {
			printError(err)
		}

		fmt.Println("\nSubtitles ready!")
		fmt.Println(srtTranslatedOutput)
		os.Exit(0)
	}

	if *ytdlpFlag {
		var srtSource, lang string
		if *geminiFlag {
			srtSource = srtTranslatedTmp
			lang = *langFlag
		} else {
			srtSource = srtTmp
			lang = "eng"
		}

		runCommand("Embedding Subtitles.", "ffmpeg", "-y", "-i", videoTmp, "-i", srtSource, "-c", "copy", "-c:s", "srt", "-metadata:s:s:0", "language="+lang, videoOutput)

		debugLog("Deleting file:", videoTmp)
		os.Remove(videoTmp)
		debugLog("Deleting file:", srtTmp)
		os.Remove(srtTmp)
		if *geminiFlag {
			debugLog("Deleting file:", srtTranslatedTmp)
			os.Remove(srtTranslatedTmp)
		}

		fmt.Println("\nSubtitles ready!")
		fmt.Println(videoOutput)
		os.Exit(0)
	}

	if *geminiFlag {
		if moveFile(srtTmp, srtOutput); err != nil {
			printError(err)
		}

		if moveFile(srtTranslatedTmp, srtTranslatedOutput); err != nil {
			printError(err)
		}

	} else {
		if moveFile(srtTmp, srtTranslatedOutput); err != nil {
			printError(err)
		}
	}

	fmt.Println("\nSubtitles ready!")
}
