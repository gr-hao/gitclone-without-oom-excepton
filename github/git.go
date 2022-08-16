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
	"time"

	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing"
	"github.com/shirou/gopsutil/mem"
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

func shellRun(command string) (error, string, string) {
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	cmd := exec.Command("sh", "-c", command)
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	err := cmd.Run()
	return err, stdout.String(), stderr.String()
}

type Repo struct {
	RepoUrl             string
	RepoDirPath         string
	RemoteBranches      []string
	freeMemmoryAtStart  uint64
	freeMemmoryAtFailed uint64
	minMemmoryRequired  uint64
	memmoryRequired     uint64
	failureCount        uint64
	cloneType           string
	repo                *git.Repository
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
	repoQueue             []*Repo
	queue                 chan *Repo
	ramAtStart            uint64
	ramCofigured          uint64
	minMemoryForEachClone uint64
	totalMemoryConsuming  uint64
	memGuard              uint64
	beingClones           int
	lock                  sync.RWMutex
}

func NewGit() *Git {
	repoFolder := os.Getenv("REPO_FOLDER")
	if repoFolder == "" {
		repoFolder = "./repos"
	}
	g := Git{
		repoFolder: repoFolder,
		repoQueue:  []*Repo{},
		queue:      make(chan *Repo),
	}

	g.ramAtStart = 0
	g.minMemoryForEachClone = 30
	ramCofigured := os.Getenv("MEMORY_LIMIT")

	x, _ := strconv.ParseInt(ramCofigured, 10, 64)
	if x == 0 {
		g.ramCofigured = 160
	} else {
		g.ramCofigured = uint64(x)
	}
	fmt.Printf("MEMORY_LIMIT: %d\n", g.ramCofigured)
	go g.GitDispatcher()
	return &g
}

func (g *Git) GitDispatcher() {
	cloneSuccess := 0
	success := make(chan int)
	for {
		select {
		case x := <-success:
			cloneSuccess += x
		case repo := <-g.queue:
			g.repoQueue = append(g.repoQueue, repo)
		case <-time.After(time.Second):
			free, used := g.GetFreeUsage()
			fmt.Printf("cloned success: %d, queue: %d, clonings: %d,  MemUsed=%d Mi\n", cloneSuccess, len(g.repoQueue), g.beingClones, used)

			if len(g.repoQueue) == 0 {
				continue
			}

			g.lock.RLock()
			if len(g.repoQueue) <= 1 && g.beingClones == 0 {
				g.memGuard = 0
				g.totalMemoryConsuming = 0
			}
			g.lock.RUnlock()

			i := 0
			var repo *Repo

			// Repo with small required-mem is higher priority to run fist
			// Should be optimized with order queue.
			sort.Slice(g.repoQueue, func(i, j int) bool {
				if g.repoQueue[i].memmoryRequired < g.repoQueue[j].memmoryRequired {
					return true
				}
				return false
			})

			for _, repo = range g.repoQueue {
				totalNextMemUsing := uint64(0)

				g.lock.RLock()
				if g.beingClones == 0 {
					g.memGuard = 0
					g.totalMemoryConsuming = 0
					repo.failureCount = 0
					repo.memmoryRequired = repo.minMemmoryRequired
				} else {
					totalNextMemUsing = g.totalMemoryConsuming + repo.memmoryRequired
				}

				available := uint64(0)
				if g.ramCofigured > g.memGuard {
					available = g.ramCofigured - g.memGuard
				}
				g.lock.RUnlock()

				if repo.failureCount >= 5 && len(g.repoQueue) > 1 {
					continue
				}

				if totalNextMemUsing <= available && free >= repo.memmoryRequired {
					fmt.Printf("totalNextMemUsing: %d, available: %d, free: %d, memmoryRequired: %d\n", totalNextMemUsing, available, free, repo.memmoryRequired)

					g.lock.Lock()
					g.totalMemoryConsuming += repo.memmoryRequired
					g.beingClones += 1
					g.lock.Unlock()

					repo.freeMemmoryAtStart, _ = g.GetFreeUsage()

					go func(repo *Repo) {
						var err error
						fmt.Printf("Start clone '%s' ...\n", repo.RepoUrl)

						if repo.cloneType == "blobless" {
							err = g.doBloblessClone(repo, true)
						} else {
							err = g.doFullClone(repo)
						}

						g.lock.Lock()
						g.totalMemoryConsuming -= repo.memmoryRequired
						g.beingClones -= 1
						if g.beingClones < 0 {
							g.beingClones = 0
						}
						g.lock.Unlock()

						if err != nil {
							fmt.Printf("Failed to clone '%s', requeue!\n", repo.RepoUrl)
							repo.failureCount++
							repo.freeMemmoryAtFailed, _ = g.GetFreeUsage()

							if repo.memmoryRequired < g.ramCofigured {
								repo.memmoryRequired *= (2 << repo.failureCount)
								fmt.Printf("----------------- memmoryRequired = %d\n", repo.memmoryRequired)

								g.lock.Lock()
								g.memGuard += repo.memmoryRequired
								g.lock.Unlock()
							}

							g.queue <- repo
						} else {
							g.lock.Lock()
							if g.memGuard >= repo.memmoryRequired {
								g.memGuard -= repo.memmoryRequired
							}
							g.lock.Unlock()

							repo.failureCount = 0
							repo.memmoryRequired = g.minMemoryForEachClone
							success <- 1
							fmt.Printf("=== Clone '%s' successfully ===\n", repo.RepoUrl)
						}
					}(repo)
					i++
				}
			}
			g.repoQueue = g.repoQueue[i:]
		}
	}
}

func (g *Git) GetFreeUsage() (uint64, uint64) {
	bToMb := func(b uint64) uint64 {
		return b / 1024 / 1024
	}
	memInfo, _ := mem.VirtualMemory()
	if g.ramAtStart == 0 {
		g.ramAtStart = memInfo.Used
	}
	used := uint64(0)
	if memInfo.Used > g.ramAtStart {
		used = bToMb(memInfo.Used - g.ramAtStart)
	}

	// Used number only reflect the memory consumed by git clones,
	// not for the whole memory that contiainer app is being used.
	return g.ramCofigured - used, used
}

func (g *Git) FullClone(RepoUrl string) {
	repo := Repo{
		RepoUrl:            RepoUrl,
		cloneType:          "full",
		memmoryRequired:    g.minMemoryForEachClone,
		minMemmoryRequired: g.minMemoryForEachClone,
	}
	g.queue <- &repo
}

func (g *Git) BloblessClone(RepoUrl string) {
	sizeMemMap := map[uint64][]uint64{
		20:  {0, 100},
		40:  {100, 300},
		80:  {300, 500},
		120: {500, 1024},
		150: {1024, 2048},
		250: {2048, 1024 * 4},
	}
	minMemoryForEachClone := uint64(200)

	repoSize := g.getRepoSize(RepoUrl)
	for s, r := range sizeMemMap {
		if r[0] <= repoSize && repoSize < r[1] {
			minMemoryForEachClone = s
			break
		}
	}

	repo := Repo{
		RepoUrl:            RepoUrl,
		cloneType:          "blobless",
		memmoryRequired:    minMemoryForEachClone,
		minMemmoryRequired: minMemoryForEachClone,
	}

	fmt.Printf("Queue to clone: %s, required-mem: %d\n", RepoUrl, minMemoryForEachClone)
	g.queue <- &repo
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
	repo.RepoDirPath = fmt.Sprintf("%s/%s", g.repoFolder, randStr(8))

	cmd := fmt.Sprintf("git clone --filter=blob:none %s %s", repo.RepoUrl, repo.RepoDirPath)
	//cmd := fmt.Sprintf("git clone --filter=tree:0 %s %s", repo.RepoUrl, repo.RepoDirPath)
	fmt.Printf("Runing: %s\n", cmd)

	err, out, errout := shellRun(cmd)
	if err != nil {
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
	_, err = repo.getRemoteBranches()
	return err
}

func (g *Git) getRepoSize(repoUrl string) uint64 {
	s := strings.Split(repoUrl, "github.com/")
	if len(s) <= 1 {
		return 0
	}

	r := strings.ReplaceAll(s[1], ".git", "")
	url := "https://api.github.com/repos/" + r

	resp, err := http.Get(url)
	if err != nil {
		return 0
	}

	defer resp.Body.Close()
	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return 0
	}

	var j map[string]interface{}
	err = json.Unmarshal(body, &j)
	if err != nil {
		return 0
	}

	size := uint64(j["size"].(float64) / 1024)
	return size
}
