// SPDX-FileCopyrightText: 2026 James L. Burns and The GoPMgr Contributors
// SPDX-License-Identifier: GPL-3.0-or-later

// Standalone SDK experiment. Never linked into Go or the Rust worker.
//
// Watches one directory tree through FSEvents and prints one JSON line per
// file-level event. fseventsd records events in the kernel's order, so this
// observer sees files that were created and removed before anyone listed the
// directory. It does not see file contents, does not say which process acted,
// merges flags per path, and can drop events; the drop flags are printed so
// the caller can fail the observation.
#include <CoreServices/CoreServices.h>
#include <stdio.h>
#include <string.h>
#include <unistd.h>

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
        printf("{\"path_hex\":\"");
        print_hex(names[i]);
        printf("\",\"flags\":%u,\"id\":%llu}\n", (unsigned)flags[i], (unsigned long long)ids[i]);
    }
    fflush(stdout);
}

int main(int argc, char **argv) {
    if (argc != 2) {
        return 2;
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
    puts("{\"done\":true}");
    fflush(stdout);
    CFRelease(roots);
    CFRelease(root);
    return 0;
}
