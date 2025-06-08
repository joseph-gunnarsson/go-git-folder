# Go Git Folder

A Go-based command-line tool that copies the directory structure of a Git repository without downloading the actual files. This tool is useful when you need to replicate a repository's folder structure.

## Features

- Clone directory structure from any Git repository
- Configurable directory depth limit
- Custom ignore patterns support
- Works with or without Git installed (falls back to HTTP download)

## Installation

```bash
# Clone the repository
git clone https://github.com/joseph-gunnarsson/go-git-folder

# Build the binary
cd go-git-folder
go build -o git-folder-copier
```

## Usage

```bash
./git-folder-copier [options]
```

### Options

- `-g` : Git repository URL (required)
- `-i` : Path to ignore file with patterns (optional)
- `-d` : Maximum depth of folders to copy (-1 for unlimited) (optional)
- `-o` : Output directory (default: current directory)

### Example

```bash
./git-folder-copier -g https://github.com/user/repo -i ignore.txt -d 3 -o ./output
```

### Ignore File Format

The ignore file should contain one pattern per line. Empty lines and lines starting with # are ignored.

Example `ignore.txt`:
```
node_modules
.vscode
build
dist
```

## How It Works

1. The tool first attempts to use `git clone` if Git is installed on the system
2. If Git is not available, it falls back to downloading the repository as a ZIP file
3. Only directory structures are extracted/copied
4. Specified ignore patterns are applied during the copying process
5. The tool respects the maximum depth setting if specified
