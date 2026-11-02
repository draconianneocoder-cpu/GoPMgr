// SPDX-FileCopyrightText: 2026 James L. Burns and The GoPMgr Contributors
// SPDX-License-Identifier: GPL-3.0-or-later

// Standalone SDK experiment. Never linked into Go or the Rust worker.
//
// Watches one directory tree through FSEvents. Usage:
//
//   fsevents ROOT [PREFIX ...]
//
// Events under a PREFIX (or under ROOT when none is given) are printed in
// full, one JSON line each. Every other event is only counted, by its first
// path component, so a volume-wide watch never prints another application's
// paths. Events carrying a dropped-event flag are counted wherever they are,
// so the caller can fail an incomplete observation.
//
// fseventsd records events in the kernel's order, so this observer sees files
// that were created and removed before anyone listed the directory. It does
// not see file contents, does not say which process acted, merges flags per
// path, and can drop events.
#include <CoreServices/CoreServices.h>
#include <stdio.h>
#include <string.h>
#include <unistd.h>

#define MAX_PREFIXES 16
#define MAX_COMPONENTS 64
#define DROP_FLAGS (kFSEventStreamEventFlagMustScanSubDirs | kFSEventStreamEventFlagUserDropped | \
                    kFSEventStreamEventFlagKernelDropped)

static const char *prefixes[MAX_PREFIXES];
static int prefix_count;
static char components[MAX_COMPONENTS][64];
static unsigned long long component_counts[MAX_COMPONENTS];
static int component_count;
static unsigned long long other_overflow, dropped_events;

static int under_prefix(const char *path) {
    for (int i = 0; i < prefix_count; i++) {
        size_t n = strlen(prefixes[i]);
        if (strncmp(path, prefixes[i], n) == 0 && (path[n] == '/' || path[n] == '\0')) {
            return 1;
        }
    }
    return 0;
}

// count_other records an event outside every prefix by its first component
// only ("/Users", "/private", ...), never by a longer path.
static void count_other(const char *path) {
    char first[64] = "/";
    const char *start = path[0] == '/' ? path + 1 : path;
    size_t n = strcspn(start, "/");
    if (n >= sizeof first - 1) {
        n = sizeof first - 2;
    }
    memcpy(first + 1, start, n);
    first[n + 1] = '\0';
    for (int i = 0; i < component_count; i++) {
        if (strcmp(components[i], first) == 0) {
            component_counts[i]++;
            return;
        }
    }
    if (component_count == MAX_COMPONENTS) {
        other_overflow++;
        return;
    }
    strcpy(components[component_count], first);
    component_counts[component_count++] = 1;
}

static void print_hex(const char *text) {
    for (const unsigned char *c = (const unsigned char *)text; *c; c++) {
        printf("%02x", *c);
    }
}

static void callback(ConstFSEventStreamRef stream, void *info, size_t count, void *paths,
                     const FSEventStreamEventFlags flags[], const FSEventStreamEventId ids[]) {
    (void)stream;
    (void)info;
    char **names = paths;
    for (size_t i = 0; i < count; i++) {
        if (flags[i] & DROP_FLAGS) {
            dropped_events++;
        }
        if (!under_prefix(names[i])) {
            count_other(names[i]);
            continue;
        }
        printf("{\"path_hex\":\"");
        print_hex(names[i]);
        printf("\",\"flags\":%u,\"id\":%llu}\n", (unsigned)flags[i], (unsigned long long)ids[i]);
    }
    fflush(stdout);
}

int main(int argc, char **argv) {
    if (argc < 2 || argc - 2 > MAX_PREFIXES) {
        return 2;
    }
    for (int i = 2; i < argc; i++) {
        prefixes[prefix_count++] = argv[i];
    }
    if (prefix_count == 0) {
        prefixes[prefix_count++] = argv[1];
    }
    CFStringRef root = CFStringCreateWithCString(NULL, argv[1], kCFStringEncodingUTF8);
    if (root == NULL) {
        return 1;
    }
    CFArrayRef roots = CFArrayCreate(NULL, (const void **)&root, 1, &kCFTypeArrayCallBacks);
    FSEventStreamRef stream = FSEventStreamCreate(
        NULL, callback, NULL, roots, kFSEventStreamEventIdSinceNow, 0.02,
        kFSEventStreamCreateFlagFileEvents | kFSEventStreamCreateFlagNoDefer |
            kFSEventStreamCreateFlagWatchRoot);
    if (stream == NULL) {
        return 1;
    }
    dispatch_queue_t queue = dispatch_queue_create("org.gopmgr.df0.fsevents", NULL);
    FSEventStreamSetDispatchQueue(stream, queue);
    if (!FSEventStreamStart(stream)) {
        return 1;
    }
    // Events dispatched after this line refer to activity after the start.
    puts("{\"ready\":true}");
    fflush(stdout);
    char buffer[64];
    while (read(STDIN_FILENO, buffer, sizeof buffer) > 0) {
    }
    // Deliver everything fseventsd has recorded before reporting the end.
    FSEventStreamFlushSync(stream);
    FSEventStreamStop(stream);
    FSEventStreamInvalidate(stream);
    FSEventStreamRelease(stream);
    dispatch_sync(queue, ^{
                  });
    printf("{\"done\":true,\"dropped_events\":%llu,\"other_overflow\":%llu,\"other_counts\":{",
           dropped_events, other_overflow);
    for (int i = 0; i < component_count; i++) {
        printf("%s\"", i ? "," : "");
        print_hex(components[i]);
        printf("\":%llu", component_counts[i]);
    }
    puts("}}");
    fflush(stdout);
    CFRelease(roots);
    CFRelease(root);
    return 0;
}
