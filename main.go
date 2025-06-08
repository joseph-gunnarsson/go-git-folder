package main

import (
	"archive/zip"
	"bufio"
	"flag"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
)

type Config struct {
	GitRepoURL     string
	IgnoreFile     string
	MaxDepth       int
	OutputDir      string
	ignorePatterns []string
}

func main() {
	var config Config

	flag.StringVar(&config.GitRepoURL, "g", "", "Git repository URL")
	flag.StringVar(&config.IgnoreFile, "i", "", "Ignore file with patterns (one pattern per line)")
	flag.IntVar(&config.MaxDepth, "d", -1, "Maximum depth of folders to copy (-1 for unlimited)")
	flag.StringVar(&config.OutputDir, "o", ".", "Output directory (default: current directory)")

	flag.Usage = func() {
		fmt.Fprintf(os.Stderr, "Usage: %s [options]\n", os.Args[0])
		fmt.Fprintf(os.Stderr, "\nOptions:\n")
		flag.PrintDefaults()
		fmt.Fprintf(os.Stderr, "\nExample:\n")
		fmt.Fprintf(os.Stderr, "  %s -g https://github.com/user/repo -i ignore.txt -d 3 -o ./output\n", os.Args[0])
		fmt.Fprintf(os.Stderr, "\nNote: This tool only copies directory structures, no files are copied.\n")
	}

	flag.Parse()

	if config.GitRepoURL == "" {
		fmt.Fprintf(os.Stderr, "Error: Git repository URL is required (-g flag)\n")
		flag.Usage()
		os.Exit(1)
	}

	if err := run(config); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}

func run(config Config) error {

	if config.IgnoreFile != "" {
		patterns, err := loadIgnorePatterns(config.IgnoreFile)
		if err != nil {
			return fmt.Errorf("failed to load ignore patterns: %w", err)
		}
		config.ignorePatterns = patterns
	}

	tempDir, err := os.MkdirTemp("", "git-folder-copier-*")
	if err != nil {
		return fmt.Errorf("failed to create temp directory: %w", err)
	}
	defer os.RemoveAll(tempDir)

	fmt.Printf("Downloading repository: %s\n", config.GitRepoURL)
	repoDir := filepath.Join(tempDir, "repo")
	if err := downloadRepo(config.GitRepoURL, repoDir); err != nil {
		return fmt.Errorf("failed to download repository: %w", err)
	}

	repoName := getRepoName(config.GitRepoURL)
	outputPath := config.OutputDir

	if err := os.MkdirAll(outputPath, 0755); err != nil {
		return fmt.Errorf("failed to create output directory: %w", err)
	}

	fmt.Printf("Copying folder structure to: %s\n", outputPath)
	if err := copyFolderStructure(repoDir, outputPath, config, 0); err != nil {
		return fmt.Errorf("failed to copy folder structure: %w", err)
	}

	fmt.Printf("Successfully copied folder structure from %s to: %s\n", repoName, outputPath)
	return nil
}

func loadIgnorePatterns(filename string) ([]string, error) {
	file, err := os.Open(filename)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	var patterns []string
	scanner := bufio.NewScanner(file)

	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line != "" && !strings.HasPrefix(line, "#") {
			patterns = append(patterns, line)
		}
	}

	return patterns, scanner.Err()
}

func downloadRepo(repoURL, destDir string) error {

	if isGitInstalled() {
		fmt.Println("Using git clone...")
		return cloneRepo(repoURL, destDir)
	}

	fmt.Println("Git not found, using HTTP download...")
	return downloadRepoHTTP(repoURL, destDir)
}

func isGitInstalled() bool {
	_, err := exec.LookPath("git")
	return err == nil
}

func cloneRepo(repoURL, destDir string) error {
	cmd := exec.Command("git", "clone", "--depth", "1", repoURL, destDir)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

func downloadRepoHTTP(repoURL, destDir string) error {
	zipURL, err := convertToZipURL(repoURL)
	if err != nil {
		return err
	}

	fmt.Printf("Downloading ZIP from: %s\n", zipURL)

	resp, err := http.Get(zipURL)
	if err != nil {
		return fmt.Errorf("failed to download repository: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("failed to download repository: HTTP %d", resp.StatusCode)
	}

	zipFile, err := os.CreateTemp("", "repo-*.zip")
	if err != nil {
		return fmt.Errorf("failed to create temp ZIP file: %w", err)
	}
	defer os.Remove(zipFile.Name())
	defer zipFile.Close()

	_, err = io.Copy(zipFile, resp.Body)
	if err != nil {
		return fmt.Errorf("failed to save ZIP file: %w", err)
	}

	return extractZipDirectoriesOnly(zipFile.Name(), destDir)
}

func convertToZipURL(repoURL string) (string, error) {

	cleanURL := strings.TrimSuffix(repoURL, ".git")

	if strings.Contains(cleanURL, "github.com") {
		return cleanURL + "/archive/refs/heads/main.zip", nil
	} else if strings.Contains(cleanURL, "gitlab.com") {
		return cleanURL + "/-/archive/main/archive.zip", nil
	} else if strings.Contains(cleanURL, "bitbucket.org") {
		return cleanURL + "/get/main.zip", nil
	}

	return cleanURL + "/archive/refs/heads/main.zip", nil
}

func extractZipDirectoriesOnly(zipPath, destDir string) error {
	reader, err := zip.OpenReader(zipPath)
	if err != nil {
		return fmt.Errorf("failed to open ZIP file: %w", err)
	}
	defer reader.Close()

	var rootDir string
	for _, file := range reader.File {
		if file.FileInfo().IsDir() && strings.Count(file.Name, "/") == 1 {
			rootDir = file.Name
			break
		}
	}

	for _, file := range reader.File {
		if !strings.HasPrefix(file.Name, rootDir) {
			continue
		}

		relativePath := strings.TrimPrefix(file.Name, rootDir)
		if relativePath == "" {
			continue
		}

		if !file.FileInfo().IsDir() {
			continue
		}

		destPath := filepath.Join(destDir, relativePath)

		err := os.MkdirAll(destPath, 0755)
		if err != nil {
			return fmt.Errorf("failed to create directory: %w", err)
		}
	}

	return nil
}

func getRepoName(repoURL string) string {

	parts := strings.Split(repoURL, "/")
	if len(parts) > 0 {
		name := parts[len(parts)-1]

		if strings.HasSuffix(name, ".git") {
			name = name[:len(name)-4]
		}
		return name
	}
	return "repo"
}

func copyFolderStructure(srcDir, destDir string, config Config, currentDepth int) error {

	if config.MaxDepth >= 0 && currentDepth > config.MaxDepth {
		return nil
	}

	entries, err := os.ReadDir(srcDir)
	if err != nil {
		return err
	}

	for _, entry := range entries {

		if !entry.IsDir() {
			continue
		}

		if entry.Name() == ".git" {
			continue
		}

		if shouldIgnoreDirectory(entry.Name(), config.ignorePatterns) {
			fmt.Printf("Ignoring directory: %s\n", entry.Name())
			continue
		}

		srcPath := filepath.Join(srcDir, entry.Name())
		destPath := filepath.Join(destDir, entry.Name())

		if err := os.MkdirAll(destPath, 0755); err != nil {
			return fmt.Errorf("failed to create directory %s: %w", destPath, err)
		}

		fmt.Printf("Created directory: %s\n", destPath)

		if err := copyFolderStructure(srcPath, destPath, config, currentDepth+1); err != nil {
			return err
		}
	}

	return nil
}

func shouldIgnoreDirectory(dirName string, patterns []string) bool {
	for _, pattern := range patterns {

		regexPattern := globToRegex(pattern)
		matched, err := regexp.MatchString(regexPattern, dirName)
		if err != nil {
			fmt.Printf("Warning: Invalid regex pattern '%s': %v\n", pattern, err)
			continue
		}
		if matched {
			return true
		}
	}
	return false
}

func globToRegex(glob string) string {
	regex := strings.ReplaceAll(glob, "*", ".*")
	regex = strings.ReplaceAll(regex, "?", ".")
	return "^" + regex + "$"
}
