package main

import (
	"fmt"
	"gitclone/github"
	"sync"
	"time"
)

var RepoUrls = []string{
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

	/* repoUrls := []string{
		"https://github.com/microsoft/vscode.git",
	} */

	_ = repoUrls

	g := github.NewGit()
	_ = g

	defer func() {
		r := recover()
		fmt.Println(r)
	}()

	go func() {
		for {
			if g.GitRepoNums() == 0 {
				for _, r := range repoUrls {
					g.BloblessClone(r)
				}
			}
			time.Sleep(5 * time.Second)
		}
	}()

	var wg sync.WaitGroup
	wg.Add(1)
	wg.Wait()
}
