// SPDX-FileCopyrightText: 2026 James L. Burns and The GoPMgr Contributors
// SPDX-License-Identifier: GPL-3.0-or-later

// Standalone SDK capability probe. Not linked into Go or the Rust worker.
#include <mach/mach.h>
#include <stdio.h>
#include <sys/resource.h>
#include <unistd.h>

int main(void) {
    const struct rlimit core = {0, 0};
    if (setrlimit(RLIMIT_CORE, &core) != 0) {
        perror("disable core dump");
        return 1;
    }
    task_basic_info_64_data_t info = {0};
    mach_msg_type_number_t count = TASK_BASIC_INFO_64_COUNT;
    kern_return_t read_result = task_info(mach_task_self(), TASK_BASIC_INFO_64,
                                         (task_info_t)&info, &count);
    int old_limit = -1;
    kern_return_t limit_result = task_set_phys_footprint_limit(mach_task_self(),
                                                              64, &old_limit);
    // Record the API result rather than assuming this SDK declaration grants
    // permission or establishes fatal enforcement. No allocation attack follows.
    if (printf("{\"unprivileged\":%s,\"task_info_result\":%d,"
               "\"footprint_limit_result\":%d,\"kern_no_access\":%d,"
               "\"requested_mib\":64,\"old_limit_mib\":%d}\n",
               getuid() != 0 && geteuid() != 0 ? "true" : "false",
               read_result, limit_result, KERN_NO_ACCESS,
               limit_result == KERN_SUCCESS ? old_limit : -1) < 0) {
        return 1;
    }
    return read_result == KERN_SUCCESS ? 0 : 1;
}
