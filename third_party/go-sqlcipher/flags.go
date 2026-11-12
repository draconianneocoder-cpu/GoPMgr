package sqlite3

/*
// make go-sqlite3 use embedded library without code changes
#cgo CFLAGS: -DUSE_LIBSQLITE3

// enable encryption codec in sqlite
#cgo CFLAGS: -DSQLITE_HAS_CODEC

// register SQLCipher with SQLite at start-up (required since SQLCipher 4.7)
#cgo CFLAGS: -DSQLITE_EXTRA_INIT=sqlcipher_extra_init
#cgo CFLAGS: -DSQLITE_EXTRA_SHUTDOWN=sqlcipher_extra_shutdown

// GoPMgr: distrust the schema of every file, and refuse the SQL that can
// corrupt one. With trusted_schema off, a view, trigger, CHECK, DEFAULT,
// index or generated column in a file may call only functions and virtual
// tables marked safe for it. Defensive mode refuses writable_schema, writes
// to shadow tables and sqlite_dbpage. Set at compile time so that no
// connection, however it is opened, can miss them.
#cgo CFLAGS: -DSQLITE_TRUSTED_SCHEMA=0
#cgo CFLAGS: -DSQLITE_DEFAULT_DEFENSIVE

// use memory for temporay storage in sqlite
#cgo CFLAGS: -DSQLITE_TEMP_STORE=2

// use libtomcrypt implementation in sqlcipher
#cgo CFLAGS: -DSQLCIPHER_CRYPTO_LIBTOMCRYPT

// disable anything "not portable" in libtomcrypt
#cgo CFLAGS: -DLTC_NO_ASM

// disable assertions
#cgo CFLAGS: -DNDEBUG

// set operating specific sqlite flags
#cgo linux CFLAGS: -DSQLITE_OS_UNIX=1
#cgo windows CFLAGS: -DSQLITE_OS_WIN=1

// gopmgr_rng.c seeds the provider from the Windows system generator
#cgo windows LDFLAGS: -lbcrypt
*/
import "C"
