// SPDX-FileCopyrightText: 2026 James L. Burns and The GoPMgr Contributors
// SPDX-License-Identifier: GPL-3.0-or-later

// Standalone SDK experiment. Never linked into Go or the Rust worker.
#include <arpa/inet.h>
#include <errno.h>
#include <fcntl.h>
#include <inttypes.h>
#include <libproc.h>
#include <limits.h>
#include <mach/mach_time.h>
#include <stdio.h>
#include <stdlib.h>
#include <string.h>
#include <sys/mman.h>
#include <sys/resource.h>
#include <sys/socket.h>
#include <sys/un.h>
#include <sys/wait.h>
#include <time.h>
#include <unistd.h>

#define PAYLOAD_BYTES (32U * 1024U * 1024U)

static uint64_t now_ns(clockid_t clock) {
    struct timespec t;
    if (clock_gettime(clock, &t) != 0) exit(3);
    return (uint64_t)t.tv_sec * 1000000000ULL + (uint64_t)t.tv_nsec;
}

static int sample(int argc, char **argv) {
    if (argc < 3 || argc > 10) return 2;
    mach_timebase_info_data_t scale;
    if (mach_timebase_info(&scale) != KERN_SUCCESS || scale.denom == 0) return 1;
    for (int i = 2; i < argc; i++) {
        char *end = NULL;
        errno = 0;
        long pid = strtol(argv[i], &end, 10);
        if (errno || end == argv[i] || *end || pid <= 0 || pid > INT_MAX) return 2;
        struct rusage_info_v0 r = {0};
        uint64_t before = now_ns(CLOCK_MONOTONIC);
        if (proc_pid_rusage((int)pid, RUSAGE_INFO_V0, (rusage_info_t *)&r) != 0) {
            perror("proc_pid_rusage");
            return 1;
        }
        // libproc CPU times are Mach absolute ticks, unlike getrusage's
        // timeval. Widen before multiplication to avoid uint64 overflow.
        __uint128_t user_ns = (__uint128_t)r.ri_user_time * scale.numer / scale.denom;
        __uint128_t system_ns = (__uint128_t)r.ri_system_time * scale.numer / scale.denom;
        if (user_ns > UINT64_MAX || system_ns > UINT64_MAX) return 1;
        printf("{\"pid\":%ld,\"start_ticks\":%" PRIu64 ","
               "\"rss_bytes\":%" PRIu64 ",\"footprint_bytes\":%" PRIu64 ","
               "\"user_ns\":%" PRIu64 ",\"system_ns\":%" PRIu64 ","
               "\"before_ns\":%" PRIu64 ",\"after_ns\":%" PRIu64 "}\n",
               pid, r.ri_proc_start_abstime, r.ri_resident_size, r.ri_phys_footprint,
               (uint64_t)user_ns, (uint64_t)system_ns, before, now_ns(CLOCK_MONOTONIC));
    }
    return fflush(stdout) == 0 ? 0 : 1;
}

static int payload(void) {
    unsigned char *held = MAP_FAILED;
    puts("ready");
    fflush(stdout);
    for (int command; (command = getchar()) != EOF;) {
        if (command == 'q') break;
        if (command == 'a' || command == 'b') {
            if (held != MAP_FAILED) return 2;
            held = mmap(NULL, PAYLOAD_BYTES, PROT_READ | PROT_WRITE,
                        MAP_PRIVATE | MAP_ANON, -1, 0);
            if (held == MAP_FAILED) return 1;
            volatile unsigned char *p = held;
            for (size_t i = 0; i < PAYLOAD_BYTES; i += 4096) p[i] = 0x5a;
            if (command == 'b') {
                if (munmap(held, PAYLOAD_BYTES) != 0) return 1;
                held = MAP_FAILED;
            }
            puts(command == 'a' ? "held" : "burst");
        } else if (command == 'c') {
            uint64_t start = now_ns(CLOCK_PROCESS_CPUTIME_ID);
            volatile uint64_t value = 1;
            while (now_ns(CLOCK_PROCESS_CPUTIME_ID) - start < 50000000ULL) value++;
            puts("cpu");
        } else return 2;
        if (fflush(stdout) != 0) return 1;
    }
    return held == MAP_FAILED || munmap(held, PAYLOAD_BYTES) == 0 ? 0 : 1;
}

static int connect_error(int domain, const struct sockaddr *address, socklen_t length) {
    int fd = socket(domain, SOCK_STREAM, 0);
    if (fd < 0) return errno;
    int error = connect(fd, address, length) == 0 ? 0 : errno;
    close(fd);
    return error;
}

static int access_probe(char **argv) {
    int fd = open(argv[2], O_RDONLY);
    int read_error = fd < 0 ? errno : 0;
    unsigned char byte = 0;
    if (fd >= 0) {
        if (read(fd, &byte, 1) != 1) return 1;
        close(fd);
    }
    fd = open(argv[3], O_WRONLY | O_CREAT | O_TRUNC, 0600);
    int write_error = fd < 0 ? errno : 0;
    if (fd >= 0) {
        if (write(fd, "Z", 1) != 1) return 1;
        close(fd);
    }
    struct sockaddr_un local = {.sun_family = AF_UNIX};
    if (strlen(argv[4]) >= sizeof(local.sun_path)) return 2;
    strcpy(local.sun_path, argv[4]);
    struct sockaddr_in tcp = {.sin_family = AF_INET, .sin_addr.s_addr = htonl(INADDR_LOOPBACK)};
    char *end = NULL;
    long port = strtol(argv[5], &end, 10);
    if (end == argv[5] || *end || port <= 0 || port > 65535) return 2;
    tcp.sin_port = htons((uint16_t)port);
    int local_error = connect_error(AF_UNIX, (struct sockaddr *)&local, sizeof(local));
    int tcp_error = connect_error(AF_INET, (struct sockaddr *)&tcp, sizeof(tcp));
    if (strcmp(argv[6], "deny-children") == 0) {
        const struct rlimit nproc = {0, 0};
        if (setrlimit(RLIMIT_NPROC, &nproc) != 0) return 1;
    } else if (strcmp(argv[6], "allow-children") != 0) return 2;
    pid_t child = fork();
    int fork_error = child < 0 ? errno : 0;
    if (child == 0) _exit(0);
    if (child > 0) {
        int status;
        pid_t waited;
        do { waited = waitpid(child, &status, 0); } while (waited < 0 && errno == EINTR);
        if (waited != child || !WIFEXITED(status) || WEXITSTATUS(status) != 0) return 1;
    }
    printf("{\"started\":true,\"read_errno\":%d,\"read_byte\":%u,"
           "\"write_errno\":%d,\"unix_errno\":%d,\"tcp_errno\":%d,"
           "\"fork_errno\":%d}\n",
           read_error, byte, write_error, local_error, tcp_error, fork_error);
    return fflush(stdout) == 0 ? 0 : 1;
}

int main(int argc, char **argv) {
    const struct rlimit core = {0, 0};
    if (setrlimit(RLIMIT_CORE, &core) != 0) return 1;
    if (argc >= 3 && strcmp(argv[1], "sample") == 0) return sample(argc, argv);
    if (argc == 2 && strcmp(argv[1], "payload") == 0) return payload();
    if (argc == 7 && strcmp(argv[1], "access") == 0) return access_probe(argv);
    return 2;
}
