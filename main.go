package main

import (
	"fmt"
	"gitclone/github"
	"sync"
)

func main() {
	fmt.Println("Start to git test")

	repoUrls := []string{
		"https://github.com/microsoft/vscode.git",
		"https://github.com/microsoft/vscode.git",
		"https://github.com/googleapis/googleapis.git",
		"https://github.com/torvalds/linux.git",
		"https://github.com/microsoft/vscode.git",
		"https://github.com/googleapis/googleapis.git",
		"https://github.com/flutter/flutter.git",
		"https://github.com/torvalds/linux.git",
		"https://github.com/flutter/flutter.git",
		"https://github.com/googleapis/googleapis.git",
		"https://github.com/torvalds/linux.git",
		"https://github.com/flutter/flutter.git",
		"https://github.com/gcc-mirror/gcc.git",
		"https://github.com/googleapis/googleapis.git",
		"https://github.com/aosp-mirror/platform_development.git",
		"https://github.com/torvalds/linux.git",
	}

	/* repoUrls := []string{
		"https://github.com/microsoft/vscode.git",
		"https://github.com/microsoft/vscode.git",
		"https://github.com/torvalds/linux.git",
		"https://github.com/microsoft/vscode.git",
		"https://github.com/microsoft/vscode.git",
		"https://github.com/torvalds/linux.git",
	} */

	g := github.NewGit()
	_ = g

	defer func() {
		r := recover()
		fmt.Println(r)
	}()

	// This app will manage to clone repositories concurency
	// as much as possible without OOM exception.
	for _, r := range repoUrls {
		g.BloblessClone(r)
	}

	var wg sync.WaitGroup
	wg.Add(1)
	wg.Wait()
}
