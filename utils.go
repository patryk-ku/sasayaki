package main

import (
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path"
	"strings"

	"github.com/google/generative-ai-go/genai"
	"github.com/leaanthony/spinner"
)

func DebugLog(a ...any) {
	if debugMode {
		fmt.Print(yellowANSI + " [debug] ")
		fmt.Println(a...)
		fmt.Print(resetANSI)
	}
}

func PrintError(err error) {
	fmt.Println(redANSI+"Error:", err, resetANSI)
}

func RunCommand(loadingMessage string, args ...string) {
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
			PrintError(err)
			os.Exit(1)
		}
		fmt.Println("")
	} else {
		myspinner := spinner.New()
		myspinner.Start(loadingMessage)

		stdout, err := cmd.CombinedOutput()
		if err != nil {
			myspinner.Error()
			fmt.Println(string(stdout))
			fmt.Println("Command failed:")
			fmt.Println(strings.Join(args, " "))
			PrintError(err)
			os.Exit(1)
		}

		myspinner.Success()
	}
}

func PrintResponse(resp *genai.GenerateContentResponse) string {
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

func ParseSRT(text string) []string {
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

func FileExists(filePath string) bool {
	_, error := os.Stat(filePath)
	return !errors.Is(error, os.ErrNotExist)
}

func FolderExists(path string) bool {
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
func MoveFile(sourcePath, destPath string) error {
	DebugLog("Moving file:", sourcePath, "to destination:", destPath)
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
func DownloadFile(url string, filepath string) error {
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

func GenerateConfig() {
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
		PrintError(err)
		os.Exit(1)
	}
	fmt.Println("Created config file:", path.Join(appDir, "config.toml"))
}
