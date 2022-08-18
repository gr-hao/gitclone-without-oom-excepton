package github

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"math/rand"
	"net/http"
	"os"
	"os/exec"
	"sort"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing"
	"github.com/shirou/gopsutil/mem"
)

var (
	RepoSize   map[string]int = map[string]int{}
	sizeMemMap                = map[int][]int{
		20:  {0, 100},
		40:  {100, 300},
		60:  {300, 500},
		70:  {500, 1024},
		180: {1024, 2048},
		250: {2048, 1024 * 4},
		//90: {2048, 1024 * 4},
	}
)

func init() {
	rand.Seed(time.Now().UnixNano())
}

func randStr(n int) string {
	letterRunes := []rune("abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ")
	b := make([]rune, n)
	for i := range b {
		b[i] = letterRunes[rand.Intn(len(letterRunes))]
	}
	return string(b)
}

type Repo struct {
	RepoUrl            string
	cloneType          string
	RepoName           string
	RepoDirPath        string
	RemoteBranches     []string
	minMemmoryRequired int
	memmoryRequired    int
	failureCount       int
	repo               *git.Repository
	worktree           *git.Worktree
	cmd                *exec.Cmd
	cloneStatus        string
}

func shellRun(repo *Repo, command string) (error, string, string) {
	cmd := exec.Command("sh", "-c", command)

	cmd.SysProcAttr = &syscall.SysProcAttr{
		Setpgid: true,
	}
	repo.cmd = cmd
	err := cmd.Run()
	return err, "", ""
}

func shellRun2(command string) (error, string, string) {
	var stdout bytes.Buffer
	var stderr bytes.Buffer

	cmd := exec.Command("sh", "-c", command)
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	err := cmd.Run()
	return err, stdout.String(), stderr.String()
}

func (r *Repo) getRemoteBranches() ([]string, error) {
	var err error
	r.RemoteBranches = []string{}

	if r.repo == nil {
		r.repo, err = git.PlainOpen(r.RepoDirPath)
		if err != nil {
			return nil, err
		}
	}

	r.worktree, err = r.repo.Worktree()
	if err != nil {
		fmt.Println("Build WorkTree failed!", err)
		return nil, err
	}

	fmt.Println("Build WorkTree successfully!", r.RepoUrl)

	remote, err := r.repo.Remote("origin")
	if err != nil {
		return nil, err

	}
	refList, err := remote.List(&git.ListOptions{})
	if err != nil {
		return nil, err
	}

	refPrefix := "refs/heads/"
	for _, ref := range refList {
		refName := ref.Name().String()
		if !strings.HasPrefix(refName, refPrefix) {
			continue
		}
		branchName := refName[len(refPrefix):]
		r.RemoteBranches = append(r.RemoteBranches, branchName)
	}
	return r.RemoteBranches, nil
}

func (r *Repo) Checkout(url, branch, commit, dest string) error {
	var err error
	repo := r.repo

	if repo == nil {
		repo, err = git.PlainOpen(r.RepoDirPath)
		if err != nil {
			return err
		}
	}

	w, err := repo.Worktree()
	if err != nil {
		return err
	}
	err = w.Checkout(&git.CheckoutOptions{
		Hash: plumbing.NewHash(commit),
	})
	if err != nil {
		return err
	}
	return nil
}

type Git struct {
	repoFolder            string
	ramAtStart            uint64
	ramCofigured          int
	minMemoryForEachClone int
	totalMemoryConsuming  int
	memGuard              int
	queueChan             chan *Repo
	clonningQueueChan     chan *Repo
	repoQueue             []*Repo
	clonningQueue         []*Repo
	success               chan int
	lock                  sync.RWMutex
}

func NewGit() *Git {
	repoFolder := os.Getenv("REPO_FOLDER")
	if repoFolder == "" {
		fmt.Println("REPO_FOLDER has not set, use local ./repos!")
		repoFolder = "./repos"
	}

	g := Git{
		repoFolder: repoFolder,
		repoQueue:  []*Repo{},
		queueChan:  make(chan *Repo),
	}

	g.ramAtStart = 0
	g.minMemoryForEachClone = 30
	ramCofigured := os.Getenv("MEMORY_LIMIT")

	x, _ := strconv.ParseInt(ramCofigured, 10, 64)
	if x == 0 {
		g.ramCofigured = 80
	} else {
		g.ramCofigured = int(x)
	}
	fmt.Printf("MEMORY_LIMIT: %d\n", g.ramCofigured)

	cmd := fmt.Sprintf("rm -rf %s/gitclone*", repoFolder)
	shellRun2(cmd)

	go g.GitDispatcher()
	return &g
}

func (g *Git) GitRepoNums() int {
	g.lock.RLock()
	defer g.lock.RUnlock()
	return len(g.clonningQueue) + len(g.repoQueue)
}

func (g *Git) CloningNum() int {
	g.lock.RLock()
	defer g.lock.RUnlock()
	return len(g.clonningQueue)
}
func (g *Git) WaitingNum() int {
	g.lock.RLock()
	defer g.lock.RUnlock()
	return len(g.repoQueue)
}

func (g *Git) GitDispatcher() {
	guard := 70
	x := os.Getenv("MEMORY_GUARD")
	y, _ := strconv.ParseInt(x, 10, 64)
	if y > 0 {
		guard = int(y)
	}
	fmt.Printf("MEMORY_GUARD: %d\n", guard)

	cloneSuccess := 0
	pressureTime := false
	g.success = make(chan int)
	g.clonningQueueChan = make(chan *Repo)

	for {
		select {
		case x := <-g.success:
			cloneSuccess += x
		case repo := <-g.queueChan:
			g.repoQueue = append(g.repoQueue, repo)
		case repo := <-g.clonningQueueChan:
			add := true
			for i, r := range g.clonningQueue {
				if r.RepoDirPath == repo.RepoDirPath && repo.cmd == nil {
					fmt.Printf("*** Repo %s stoped!\n", r.RepoName)
					g.clonningQueue = append(g.clonningQueue[0:i], g.clonningQueue[i+1:]...)
					add = false
					break
				}
			}
			if add {
				g.clonningQueue = append(g.clonningQueue, repo)
			}
		case <-time.After(1 * time.Second):
			free, used := g.GetFreeUsage()

			sort.Slice(g.clonningQueue, func(i, j int) bool {
				if g.clonningQueue[i].memmoryRequired > g.clonningQueue[j].memmoryRequired {
					return true
				}
				return false
			})

			if used+guard > g.ramCofigured && len(g.clonningQueue) >= 1 {
				pressureTime = true
				for _, r := range g.clonningQueue {
					if r.cloneStatus != "stoped" && r.cmd != nil && r.cmd.Process != nil {
						fmt.Printf("\n\n\n\n****** Stop cloning %s/%s ******\n\n\n\n", r.RepoDirPath, r.RepoName)
						r.cmd.Process.Signal(syscall.SIGSTOP)
						r.cloneStatus = "stoped"

						/* id, err := syscall.Getpgid(r.cmd.Process.Pid)
						if err == nil {
							r.cloneStatus = "stoped"
							syscall.Kill(-id, syscall.SIGKILL)
						} */

						/*
							//r.cmd.Process.Signal(syscall.SIGKILL)
							fmt.Printf("r.cmd.SysProcAttr.Pgid = %d\n", r.cmd.SysProcAttr.Pgid)
							fmt.Printf("r.cmd.Process.Pid = %d\n", r.cmd.Process.Pid)

							syscall.Kill(-r.cmd.SysProcAttr.Pgid, syscall.SIGKILL)
							//syscall.Kill(r.cmd.Process.Pid, syscall.SIGKILL)
						*/
						break
					}
				}
			} else if used+int(float64(guard)*1.2) <= g.ramCofigured {
				pressureTime = false
			}

			if g.ramCofigured-used > int(float64(guard)*1.4) {
				// Restart stopping-clone when have enough mem
				for i := len(g.clonningQueue) - 1; i >= 0; i-- {
					r := g.clonningQueue[i]
					if r.cloneStatus == "stoped" {
						r.cloneStatus = "cloning"
						fmt.Printf("\n\n\n\n****** Restart cloning %s/%s ******\n\n\n\n", r.RepoDirPath, r.RepoName)
						r.cmd.Process.Signal(syscall.SIGCONT)
						break
					}
				}
			}

			fmt.Printf("cloned success: %d, queue: %d, clonings: %d,  MemUsed=%d Mi/%d Mi, PressureTime: %v\n",
				cloneSuccess, len(g.repoQueue), g.CloningNum(), used, g.ramCofigured, pressureTime)

			fmt.Printf("%d Clones: [\n", g.CloningNum())
			for _, r := range g.clonningQueue {
				if r.cloneStatus == "stoped" {
					fmt.Printf("\t%s/%s --> Require %v (MiB) (status: 'PENDING' - waiting for free mem... )\n", r.RepoDirPath, r.RepoName, r.memmoryRequired)
				} else {
					fmt.Printf("\t%s/%s --> Require %v (MiB) (status: %s)\n", r.RepoDirPath, r.RepoName, r.memmoryRequired, r.cloneStatus)
				}
			}
			fmt.Printf("]\n")

			// No more clonning if in pressure time
			if len(g.repoQueue) == 0 || pressureTime {
				continue
			}

			// Repo with small required-mem is higher priority to run fist
			// Should be optimized with order queue.
			sort.Slice(g.repoQueue, func(i, j int) bool {
				if g.repoQueue[i].memmoryRequired < g.repoQueue[j].memmoryRequired {
					return true
				}
				return false
			})

			for i, repo := range g.repoQueue {
				g.lock.Lock()
				totalNextMemUsing := 0
				if len(g.clonningQueue) == 0 {
					g.memGuard = 0
					g.totalMemoryConsuming = 0
					repo.failureCount = 0
					repo.memmoryRequired = repo.minMemmoryRequired
				} else {
					totalNextMemUsing = g.totalMemoryConsuming + repo.memmoryRequired
				}

				available := 0
				available = g.ramCofigured - g.memGuard

				g.lock.Unlock()

				if repo.failureCount >= 5 && len(g.repoQueue) > 1 {
					continue
				}

				clonnings := g.CloningNum()
				if (totalNextMemUsing <= available && free >= repo.memmoryRequired) || (clonnings == 0) {
					fmt.Printf("totalNextMemUsing: %d, available: %d, free: %d, memmoryRequired: %d\n",
						totalNextMemUsing, available, free, repo.memmoryRequired)

					g.lock.Lock()
					g.totalMemoryConsuming += repo.memmoryRequired
					g.lock.Unlock()

					g.repoQueue = append(g.repoQueue[0:i], g.repoQueue[i+1:]...)
					go g.clone(repo)
					clonnings++
					break // Do not remove this line
				}
			}
		}
	}
}

func (g *Git) clone(repo *Repo) {
	var err error
	fmt.Printf("Start clone '%s' ...\n", repo.RepoUrl)
	repo.cloneStatus = "cloning"
	g.clonningQueueChan <- repo

	repo.RepoDirPath = fmt.Sprintf("%s/gitclone-%s", g.repoFolder, randStr(8))

	if repo.cloneType == "blobless" {
		err = g.doBloblessClone(repo, true)
	} else {
		err = g.doFullClone(repo)
	}

	repo.cmd = nil
	g.clonningQueueChan <- repo

	g.lock.Lock()
	g.totalMemoryConsuming -= repo.memmoryRequired

	if err != nil {
		fmt.Printf("Failed to clone '%s', requeue!\n", repo.RepoUrl)
		repo.failureCount += 1

		if repo.memmoryRequired < g.ramCofigured {
			if g.memGuard > repo.memmoryRequired {
				g.memGuard -= repo.memmoryRequired
			}
			repo.memmoryRequired *= (2 << repo.failureCount)
			fmt.Printf("----------------- memmoryRequired = %d\n", repo.memmoryRequired)

			g.memGuard += repo.memmoryRequired
		}

		g.queueChan <- repo
	} else {
		if g.memGuard >= repo.memmoryRequired {
			g.memGuard -= repo.memmoryRequired
		}

		repo.failureCount = 0
		g.success <- 1
		fmt.Printf("=== Clone '%s' successfully ===\n", repo.RepoUrl)
	}
	g.lock.Unlock()
}

func (g *Git) GetFreeUsage() (int, int) {
	bToMb := func(b uint64) uint64 {
		return b / 1024 / 1024
	}
	memInfo, _ := mem.VirtualMemory()
	if g.ramAtStart == 0 {
		g.ramAtStart = memInfo.Used
	}
	used := 0
	if memInfo.Used > g.ramAtStart {
		used = int(bToMb(memInfo.Used - g.ramAtStart))
	}

	// Used number only reflect the memory consumed by git clones,
	// not for the whole memory that contiainer app is being used.
	return int(g.ramCofigured - used), int(used)
}

func (g *Git) FullClone(RepoUrl string) {
	repo := Repo{
		RepoUrl:            RepoUrl,
		cloneType:          "full",
		memmoryRequired:    g.minMemoryForEachClone,
		minMemmoryRequired: g.minMemoryForEachClone,
	}
	g.queueChan <- &repo
}

func (g *Git) BloblessClone(RepoUrl string) {
	minMemoryForEachClone := 300

	repoSize, rname := g.getRepoSize(RepoUrl)
	if repoSize == 0 {
		return
	}
	for s, r := range sizeMemMap {
		if r[0] <= repoSize && int(repoSize) < r[1] {
			minMemoryForEachClone = s
			break
		}
	}

	repo := Repo{
		RepoUrl:            RepoUrl,
		RepoName:           rname,
		cloneType:          "blobless",
		memmoryRequired:    minMemoryForEachClone,
		minMemmoryRequired: minMemoryForEachClone,
	}
	fmt.Printf("Queue to clone: %s, required-mem: %d\n", RepoUrl, minMemoryForEachClone)
	g.queueChan <- &repo
}

func (g *Git) doFullClone(repo *Repo) error {
	repo.RepoDirPath = fmt.Sprintf("%s/%s", g.repoFolder, randStr(8))
	r, err := git.PlainOpen(repo.RepoDirPath)
	if err != nil {
		r, err = git.PlainClone(
			repo.RepoDirPath,
			false,
			&git.CloneOptions{
				URL:      repo.RepoUrl,
				Progress: os.Stdout,
				Depth:    0,
			})
		if err != nil {
			return err
		}
		workTree, err := r.Worktree()
		_ = workTree
		if err != nil {
			return err
		}
		fmt.Printf("Cloned repo %s successfully!\n", repo.RepoUrl)
	} else {
		workTree, err := r.Worktree()
		if err != nil {
			return err
		}
		err = workTree.Pull(&git.PullOptions{RemoteName: "origin"})
		if err != nil {
			fmt.Printf("Repo %s: %s\n", repo.RepoUrl, err.Error())
		}
		fmt.Printf("Pull repo %s successfully!\n", repo.RepoUrl)
	}
	repo.repo = r
	repo.getRemoteBranches()
	return nil
}

func (g *Git) doBloblessClone(repo *Repo, debug bool) error {
	cmd := fmt.Sprintf("git clone -j1 --filter=blob:none %s %s", repo.RepoUrl, repo.RepoDirPath)
	//cmd := fmt.Sprintf("git clone --filter=tree:0 %s %s", repo.RepoUrl, repo.RepoDirPath)
	fmt.Printf("Runing: %s\n", cmd)

	err, out, errout := shellRun(repo, cmd)
	if err != nil {
		fmt.Println(err)
		return err
	}
	if debug {
		if out != "" {
			fmt.Println("--- stdout ---")
			fmt.Println(out)
		}
		if errout != "" {
			fmt.Println("--- stderr ---")
			fmt.Println(errout)
		}
	}

	//_, err = repo.getRemoteBranches()

	// Remove folder
	cmd = fmt.Sprintf("rm -rf %s", repo.RepoDirPath)
	shellRun2(cmd)
	return err
}

func (g *Git) getRepoSize(repoUrl string) (int, string) {
	s := strings.Split(repoUrl, "github.com/")
	if len(s) <= 1 {
		return 0, ""
	}

	rname := strings.ReplaceAll(s[1], ".git", "")
	//url := "https://api.github.com/repos/" + r
	url := fmt.Sprintf("https://gr-hao:xxx@api.github.com/repos/%s", rname)

	x, ok := RepoSize[repoUrl]
	if ok {
		return x, rname
	}

	resp, err := http.Get(url)
	if err != nil {
		return 0, ""
	}

	defer resp.Body.Close()
	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return 0, ""
	}

	var j map[string]interface{}
	err = json.Unmarshal(body, &j)
	if err != nil {
		return 0, ""
	}

	size := int(j["size"].(float64) / 1024)
	RepoSize[repoUrl] = size
	return size, rname
}
