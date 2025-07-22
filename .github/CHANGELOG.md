## v0.1.11

- From now on, the gemini-2.0-flash model is used for translation instead of gemini-1.5-pro.
- Fixed displaying the output of a command that returned an error when verbose mode was disabled.
- Dependencies update.
- Code cleaning: utils functions moved to a separate file.

## v0.1.10

-   Use whisper.cpp by default on Windows.
-   New argument: `--model`

## v0.1.9

-   Builds for windows-x86_64 and macos-arm64.
-   Improved error logging.
-   Switched to native Go for whisper.cpp model downloads and curl is no longer required.
-   Option in the config file to enforce the use of whisper.cpp by default, without needing to specify the argument

## v0.1.8

-   First release on GitHub.
