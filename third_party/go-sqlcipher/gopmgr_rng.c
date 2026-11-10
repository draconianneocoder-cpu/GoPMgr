/* SPDX-FileCopyrightText: 2026 James L. Burns and The GoPMgr Contributors
 * SPDX-License-Identifier: MIT
 *
 * rng_get_bytes for SQLCipher's LibTomCrypt provider, which seeds its
 * Fortuna generator from it (SQLCipher 4.5 and later). This replaces
 * LibTomCrypt's own rng_get_bytes.c, which falls back to timing the C
 * clock when the system source cannot be read. Here the operating
 * system's generator is the only source: on any failure the buffer is
 * cleared and 0 is returned, so SQLCipher refuses to start instead of
 * running on weak randomness.
 */
#include "tomcrypt.h"

#include <string.h>

#if defined(_WIN32)
#include <windows.h>
#include <bcrypt.h>
#else
#include <unistd.h>
#if defined(__APPLE__) || defined(__linux__)
#include <sys/random.h>
#endif
#endif

unsigned long rng_get_bytes(unsigned char *out, unsigned long outlen,
                            void (*callback)(void))
{
   unsigned long done = 0;
   (void)callback;
   if (out == NULL) {
      return 0;
   }
#if defined(_WIN32)
   if (BCryptGenRandom(NULL, out, (ULONG)outlen,
                       BCRYPT_USE_SYSTEM_PREFERRED_RNG) == 0) {
      done = outlen;
   }
#else
   while (done < outlen) {
      /* getentropy fills at most 256 bytes per call. */
      size_t n = outlen - done > 256 ? 256 : (size_t)(outlen - done);
      if (getentropy(out + done, n) != 0) {
         break;
      }
      done += (unsigned long)n;
   }
#endif
   if (done != outlen) {
      memset(out, 0, (size_t)outlen);
      return 0;
   }
   return outlen;
}
