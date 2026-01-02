# Obfuscation with Garble

This project uses `garble` to obfuscate the compiled Go binaries. Obfuscation makes it more difficult for security software to detect the agent and for analysts to reverse-engineer it.

## Installation

To install `garble`, run the following command:

```bash
go install mvdan.cc/garble@latest
```

## Usage

The `build.go` script is configured to automatically use `garble` if it's available in your system's `PATH`. When you run `go run build.go`, the script checks for the presence of the `garble` command.

- **If `garble` is found:** The script will use it to build the agent and server binaries, applying obfuscation to the code.
- **If `garble` is not found:** The script will fall back to the standard `go build` command. The resulting binaries will be fully functional but not obfuscated.

For the best results and to minimize detection, it is highly recommended to install `garble` before building the project.
