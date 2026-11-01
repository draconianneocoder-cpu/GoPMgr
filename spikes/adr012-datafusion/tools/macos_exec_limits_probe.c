// SPDX-FileCopyrightText: 2026 James L. Burns and The GoPMgr Contributors
// SPDX-License-Identifier: GPL-3.0-or-later

// Standalone, child-only DF0 experiment. Not linked into Go or Rust.
#include <errno.h>
#include <mach/mach.h>
#include <spawn.h>
#include <stdio.h>
#include <string.h>
#include <sys/mman.h>
#include <sys/resource.h>
#include <sys/wait.h>
#include <unistd.h>

#ifndef DF0_AS_MIB
#define DF0_AS_MIB 0
#endif
#ifndef DF0_DENY_CHILDREN
#define DF0_DENY_CHILDREN 0
#endif

static FILE *output;

static int observe(const char *mode, const char *stage, rlim_t requested,
                   int install_error, int mapping_error, unsigned long long checksum,
                   int fork_error, int spawn_error, int wait_ok) {
    struct rlimit as = {0}, nproc = {0};
    struct rusage usage = {0};
    task_basic_info_64_data_t info = {0};
    mach_msg_type_number_t count = TASK_BASIC_INFO_64_COUNT;
    if (getrlimit(RLIMIT_AS, &as) != 0 || getrlimit(RLIMIT_NPROC, &nproc) != 0 ||
        getrusage(RUSAGE_SELF, &usage) != 0 ||
        task_info(mach_task_self(), TASK_BASIC_INFO_64, (task_info_t)&info, &count) != KERN_SUCCESS) {
        return 1;
    }
    if (fprintf(output,
        "{\"mode\":\"%s\",\"stage\":\"%s\",\"requested_as_bytes\":%llu,"
        "\"install_errno\":%d,\"read_errno\":0,\"as_soft_bytes\":%llu,\"as_hard_bytes\":%llu,"
        "\"nproc_soft\":%llu,\"nproc_hard\":%llu,\"virtual_bytes\":%llu,"
        "\"peak_rss_bytes\":%lld,\"mapping_errno\":%d,\"checksum\":%llu,"
        "\"fork_errno\":%d,\"spawn_errno\":%d,\"wait_ok\":%s}\n",
        mode, stage, (unsigned long long)requested, install_error,
        (unsigned long long)as.rlim_cur, (unsigned long long)as.rlim_max,
        (unsigned long long)nproc.rlim_cur, (unsigned long long)nproc.rlim_max,
        (unsigned long long)info.virtual_size, (long long)usage.ru_maxrss,
        mapping_error, checksum, fork_error, spawn_error, wait_ok ? "true" : "false") < 0 ||
        fflush(output) != 0) {
        return 1;
    }
    return 0;
}

static int install(int resource, rlim_t value) {
    const struct rlimit limit = {value, value};
    return setrlimit(resource, &limit) == 0 ? 0 : errno;
}

static int touch(const char *mode) {
    const size_t size = 128U * 1024U * 1024U;
    unsigned long long checksum = 0;
    int allocation_error = 0;
    void *mapping = mmap(NULL, size, PROT_READ | PROT_WRITE, MAP_ANON | MAP_PRIVATE, -1, 0);
    if (mapping == MAP_FAILED) {
        allocation_error = errno;
    } else {
        // Fixed byte accesses remain within the successful mapping. Volatile
        // stores/loads make resident touching observable, not optimized away.
        volatile unsigned char *bytes = mapping;
        for (size_t i = 0; i < size; i += 4096) {
            bytes[i] = 0x5a;
            checksum += bytes[i];
        }
    }
    int result = observe(mode, "payload", 0, 0, allocation_error, checksum, 0, 0, 1);
    if (mapping != MAP_FAILED && munmap(mapping, size) != 0) {
        return 1;
    }
    return result;
}

static int wait_owned(pid_t pid) {
    int status = 0;
    pid_t result;
    do {
        result = waitpid(pid, &status, 0);
    } while (result == -1 && errno == EINTR);
    return result == pid && WIFEXITED(status) && WEXITSTATUS(status) == 0;
}

static int spawn_probe(const char *self, const char *mode) {
    if (getuid() == 0 || geteuid() == 0) {
        return 2; // Root bypasses this per-real-UID resource check.
    }
    int fork_error = 0, spawn_error = 0, wait_ok = 1;
    pid_t child = fork();
    if (child == 0) {
        _exit(0);
    } else if (child < 0) {
        fork_error = errno;
    } else {
        wait_ok = wait_owned(child);
    }
    char *const args[] = {(char *)self, "empty", NULL};
    char *const env[] = {NULL};
    spawn_error = posix_spawn(&child, self, NULL, NULL, args, env);
    if (spawn_error == 0) {
        wait_ok = wait_owned(child) && wait_ok;
    }
    return observe(mode, "attempts", 0,
                   0, 0, 0, fork_error, spawn_error, wait_ok);
}

int main(int argc, char **argv) {
    output = stdout;
    if (install(RLIMIT_CORE, 0) != 0) {
        return 1;
    }
#ifdef DF0_WORKER
    if (argc == 1) {
        output = stderr;
        const rlim_t requested = (rlim_t)DF0_AS_MIB * 1024 * 1024;
        int error = requested == 0 ? 0 : install(RLIMIT_AS, requested);
        if (error == 0 && DF0_DENY_CHILDREN) {
            if (getuid() == 0 || geteuid() == 0) {
                return 2;
            }
            error = install(RLIMIT_NPROC, 0);
        }
        if (observe("worker", error ? "installation" : "preexec", requested,
                    error, 0, 0, 0, 0, 1) != 0) {
            return 1;
        }
        if (error != 0) {
            return 78; // Local setup refusal, not a typed gRPC RESOURCE_LIMIT.
        }
        char *const args[] = {DF0_WORKER, NULL};
        execv(DF0_WORKER, args); // Same PID and inherited stdin/empty environment.
        return 79;
    }
#endif
    if (argc != 2) {
        return 2;
    }
    if (strcmp(argv[1], "empty") == 0) {
        return 0;
    }
    if (strcmp(argv[1], "control") == 0 || strcmp(argv[1], "payload") == 0) {
        return touch(argv[1]);
    }
    if (strcmp(argv[1], "spawn-control") == 0 || strcmp(argv[1], "spawn-attempts") == 0) {
        return spawn_probe(argv[0], argv[1]);
    }
    if (strcmp(argv[1], "spawn-deny") == 0) {
        if (getuid() == 0 || geteuid() == 0) {
            return 2;
        }
        int error = install(RLIMIT_NPROC, 0);
        if (observe("spawn-deny", error ? "installation" : "preexec", 0,
                    error, 0, 0, 0, 0, 1) != 0) {
            return 1;
        }
        if (error != 0) {
            return 0;
        }
        char *const args[] = {argv[0], "spawn-attempts", NULL};
        execv(argv[0], args);
        return 79;
    }
    rlim_t requested;
    if (strcmp(argv[1], "as64") == 0) {
        requested = 64;
    } else if (strcmp(argv[1], "as512") == 0) {
        requested = 512;
    } else if (strcmp(argv[1], "as8192") == 0) {
        requested = 8192;
    } else if (strcmp(argv[1], "as-headroom") == 0) {
        task_basic_info_64_data_t info = {0};
        mach_msg_type_number_t count = TASK_BASIC_INFO_64_COUNT;
        if (task_info(mach_task_self(), TASK_BASIC_INFO_64,
                      (task_info_t)&info, &count) != KERN_SUCCESS ||
            info.virtual_size > RLIM_INFINITY - 64U * 1024U * 1024U) {
            return 1;
        }
        requested = info.virtual_size + 64U * 1024U * 1024U;
    } else {
        return 2;
    }
    if (strcmp(argv[1], "as-headroom") != 0) {
        requested *= 1024 * 1024;
    }
    int error = install(RLIMIT_AS, requested);
    if (observe(argv[1], error ? "installation" : "preexec", requested,
                error, 0, 0, 0, 0, 1) != 0) {
        return 1;
    }
    if (error != 0) {
        return 0;
    }
    char *const args[] = {argv[0], "payload", NULL};
    execv(argv[0], args);
    return 79;
}
