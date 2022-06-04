// +build linux,amd64

package util

import (
	"os"
	"syscall"
	"unsafe"

	"github.com/tiechui1994/tool/log"
)

func Deamon1(args []string) error {
	fd, _ := os.Create("/tmp/tool.log")
	pid, err := syscall.ForkExec(args[0], args, &syscall.ProcAttr{
		Dir: ".",
		Env: os.Environ(),
		Files: []uintptr{
			os.Stdin.Fd(),
			fd.Fd(),
			fd.Fd(),
		},
		Sys: &syscall.SysProcAttr{
			Setsid: true,
		},
	})
	if err != nil {
		return err
	}

	if pid > 0 {
		log.Infoln("backgroud run pid: [%v]", pid)
		os.Exit(0)
	}
	return nil
}

func Deamon2(callback func()) {
	pid, _, _ := syscall.Syscall(syscall.SYS_FORK, 0, 0, 0)
	if pid == 0 {
		// 创建新的进程组, 在新的进程组当中, 子进程成为进程组的首进程, 使得该进程脱离终端.
		syscall.Syscall(syscall.SYS_SETSID, 0, 0, 0)
		// 再次 fork 一个子进程, 退出父进程. 保证该进程不是进程组长, 同时让该进程无法再打开一个新的终端.
		pid, _, _ := syscall.Syscall(syscall.SYS_FORK, 0, 0, 0)
		if pid == 0 {
			// 修改工作目录
			dir, _ := syscall.BytePtrFromString("/")
			syscall.Syscall(syscall.SYS_CHDIR, uintptr(unsafe.Pointer(dir)), 0, 0)

			// 将文件当时创建屏蔽字设置为0
			syscall.Syscall(syscall.SYS_UMASK, uintptr(0), 0, 0)

			fd, _ := os.Create("/var/log/tool.log")
			os.Stdout = fd
			os.Stderr = fd

			callback()
		} else if pid > 0 {
			os.Exit(0)
		} else {
			os.Exit(-1)
		}
	} else {
		os.Exit(0)
	}
}
