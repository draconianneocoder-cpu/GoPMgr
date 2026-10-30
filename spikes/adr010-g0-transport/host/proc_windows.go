// SPDX-FileCopyrightText: 2026 James L. Burns and The GoPMgr Contributors
// SPDX-License-Identifier: GPL-3.0-or-later

//go:build windows

package main

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"syscall"
	"unsafe"

	"golang.org/x/sys/windows"
)

// cleanEnv keeps only what the JVM needs to start on Windows. SystemRoot
// is required by Winsock; TEMP and TMP by the JVM's temporary files.
// JAVA_TOOL_OPTIONS, _JAVA_OPTIONS and CLASSPATH never pass (TPT-017).
func cleanEnv() []string {
	var env []string
	for _, k := range []string{"SystemRoot", "TEMP", "TMP"} {
		if v, ok := os.LookupEnv(k); ok {
			env = append(env, k+"="+v)
		}
	}
	return env
}

// platformProcAttr starts the JVM suspended, so attachTree can put it in a
// Job Object before it runs a single instruction (TPT-013).
func platformProcAttr(cmd *exec.Cmd) {
	cmd.SysProcAttr = &syscall.SysProcAttr{CreationFlags: windows.CREATE_SUSPENDED}
}

// processTree is the worker's Job Object. Every process the JVM starts
// joins it, and closing it kills them all: KILL_ON_JOB_CLOSE also covers a
// host that crashes.
type processTree struct {
	job windows.Handle
}

// attachTree creates the job, assigns the suspended worker to it, then
// resumes the worker. It fails closed: on any error the caller kills the
// process, so no worker ever runs outside its job.
func attachTree(pid int) (*processTree, error) {
	job, err := windows.CreateJobObject(nil, nil)
	if err != nil {
		return nil, fmt.Errorf("create job object: %w", err)
	}
	info := windows.JOBOBJECT_EXTENDED_LIMIT_INFORMATION{}
	info.BasicLimitInformation.LimitFlags = windows.JOB_OBJECT_LIMIT_KILL_ON_JOB_CLOSE
	if _, err := windows.SetInformationJobObject(job, windows.JobObjectExtendedLimitInformation,
		uintptr(unsafe.Pointer(&info)), uint32(unsafe.Sizeof(info))); err != nil {
		_ = windows.CloseHandle(job)
		return nil, fmt.Errorf("set job limits: %w", err)
	}
	proc, err := windows.OpenProcess(windows.PROCESS_SET_QUOTA|windows.PROCESS_TERMINATE, false, uint32(pid))
	if err != nil {
		_ = windows.CloseHandle(job)
		return nil, fmt.Errorf("open worker process: %w", err)
	}
	err = windows.AssignProcessToJobObject(job, proc)
	_ = windows.CloseHandle(proc)
	if err != nil {
		_ = windows.CloseHandle(job)
		return nil, fmt.Errorf("assign worker to job: %w", err)
	}
	if err := resumeProcess(uint32(pid)); err != nil {
		_ = windows.CloseHandle(job)
		return nil, err
	}
	return &processTree{job: job}, nil
}

// resumeProcess resumes the threads of a process started suspended (only
// its main thread exists at that point).
func resumeProcess(pid uint32) error {
	snap, err := windows.CreateToolhelp32Snapshot(windows.TH32CS_SNAPTHREAD, 0)
	if err != nil {
		return fmt.Errorf("thread snapshot: %w", err)
	}
	defer windows.CloseHandle(snap)
	entry := windows.ThreadEntry32{Size: uint32(unsafe.Sizeof(windows.ThreadEntry32{}))}
	resumed := 0
	for err = windows.Thread32First(snap, &entry); err == nil; err = windows.Thread32Next(snap, &entry) {
		if entry.OwnerProcessID != pid {
			continue
		}
		th, oerr := windows.OpenThread(windows.THREAD_SUSPEND_RESUME, false, entry.ThreadID)
		if oerr != nil {
			return fmt.Errorf("open worker thread: %w", oerr)
		}
		_, rerr := windows.ResumeThread(th)
		_ = windows.CloseHandle(th)
		if rerr != nil {
			return fmt.Errorf("resume worker thread: %w", rerr)
		}
		resumed++
	}
	if !errors.Is(err, windows.ERROR_NO_MORE_FILES) {
		return fmt.Errorf("walk threads: %w", err)
	}
	if resumed == 0 {
		return errors.New("resume worker: no thread found")
	}
	return nil
}

// kill ends every process in the job, including grandchildren.
func (t *processTree) kill() {
	if t != nil {
		_ = windows.TerminateJobObject(t.job, 1)
	}
}

// release kills anything left in the job and closes it.
func (t *processTree) release() {
	if t != nil {
		t.kill()
		_ = windows.CloseHandle(t.job)
	}
}

// jobAccounting is JOBOBJECT_BASIC_ACCOUNTING_INFORMATION.
type jobAccounting struct {
	TotalUserTime             int64
	TotalKernelTime           int64
	ThisPeriodTotalUserTime   int64
	ThisPeriodTotalKernelTime int64
	TotalPageFaultCount       uint32
	TotalProcesses            uint32
	ActiveProcesses           uint32
	TotalTerminatedProcesses  uint32
}

// members reports how many processes in the tree are alive, and how
// it found out.
func (t *processTree) members() (int, string) {
	if t == nil {
		return -1, "no job"
	}
	var acc jobAccounting
	if err := windows.QueryInformationJobObject(t.job, windows.JobObjectBasicAccountingInformation,
		uintptr(unsafe.Pointer(&acc)), uint32(unsafe.Sizeof(acc)), nil); err != nil {
		return -1, "job accounting failed: " + err.Error()
	}
	return int(acc.ActiveProcesses), "job object active processes"
}

// processAlive reports whether pid names a running process.
func processAlive(pid int) bool {
	h, err := windows.OpenProcess(windows.PROCESS_QUERY_LIMITED_INFORMATION, false, uint32(pid))
	if err != nil {
		return false
	}
	defer windows.CloseHandle(h)
	var code uint32
	if err := windows.GetExitCodeProcess(h, &code); err != nil {
		return false
	}
	return code == 259 // STILL_ACTIVE
}
