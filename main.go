package main

import (
	"encoding/json"
	"fmt"
	"gitclone/github"
	"runtime"
	"sync"

	"github.com/shirou/gopsutil/mem"
)

var (
	used   uint64
	active uint64
)

func printMemUsage() {
	bToMb := func(b uint64) uint64 {
		return b / 1024 / 1024
	}
	memInfo, _ := mem.VirtualMemory()
	if used == 0 {
		used = memInfo.Used
		active = memInfo.Active
	}

	b, _ := json.MarshalIndent(memInfo, "", "  ")
	fmt.Println(string(b))

	fmt.Printf("Total: %d Mi, Available: %d Mi, Used: %d Mi, UsedPercent: %.2f %%, Free: %d Mi, Active: %d Mi\n",
		bToMb(memInfo.Total),
		bToMb(memInfo.Available),
		bToMb(memInfo.Used),
		memInfo.UsedPercent,
		bToMb(uint64(memInfo.Free)),
		bToMb(uint64(memInfo.Active-active)),
	)

	var m runtime.MemStats
	runtime.ReadMemStats(&m)

	fmt.Printf("Alloc: %d Mi, TotalAlloc: %d Mi, Sys: %d Mi, HeapAlloc: %d Mi, Idle: %d Mi\n",
		bToMb(m.Alloc), bToMb(m.TotalAlloc), bToMb(m.Sys), bToMb(m.HeapAlloc), bToMb(m.HeapIdle))
}

func main() {
	fmt.Println("Start to git test")

	/* repoUrls := []string{
		"https://github.com/microsoft/vscode.git",
		"https://github.com/microsoft/vscode.git",
		"https://github.com/googleapis/googleapis.git",
		"https://github.com/microsoft/vscode.git",
		"https://github.com/googleapis/googleapis.git",
		"https://github.com/flutter/flutter.git",
		"https://github.com/torvalds/linux.git",
		"https://github.com/flutter/flutter.git",
		"https://github.com/googleapis/googleapis.git",
		"https://github.com/flutter/flutter.git",
		"https://github.com/gcc-mirror/gcc.git",
		"https://github.com/googleapis/googleapis.git",
		"https://github.com/aosp-mirror/platform_development.git",
		"https://github.com/torvalds/linux.git",
	} */

	repoUrls := []string{
		"https://github.com/microsoft/vscode.git",
		"https://github.com/microsoft/vscode.git",
		"https://github.com/torvalds/linux.git",
		"https://github.com/microsoft/vscode.git",
		"https://github.com/microsoft/vscode.git",
		"https://github.com/torvalds/linux.git",
	}

	g := github.NewGit()
	_ = g

	// This app will manage to clone repositories concurency
	// as much as possible without OOM exception.
	for _, r := range repoUrls {
		g.BloblessClone(r)
	}

	var wg sync.WaitGroup
	wg.Add(1)
	wg.Wait()
}
