package github

import (
	"math/rand"
	"os/exec"
	"time"
)

func init() {
	rand.Seed(time.Now().UnixNano())
}

func ShellRun(command string, cb func(*exec.Cmd)) error {
	cmd := exec.Command("sh", "-c", command)

	/* cmd.SysProcAttr = &syscall.SysProcAttr{
		Setpgid: true,
	} */

	if cb != nil {
		cb(cmd)
	}
	err := cmd.Run()
	return err
}

func RandStr(n int) string {
	letterRunes := []rune("abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ")
	b := make([]rune, n)
	for i := range b {
		b[i] = letterRunes[rand.Intn(len(letterRunes))]
	}
	return string(b)
}

func BToMb(b uint64) uint64 {
	return b / 1024 / 1024
}
