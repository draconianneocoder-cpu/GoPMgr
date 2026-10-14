/* SPDX-FileCopyrightText: 2026 James L. Burns and The GoPMgr Contributors
 * SPDX-License-Identifier: GPL-3.0-or-later
 * Adapted scenario: SQLite test/walrestart.test, upstream fe57e14b.
 * Only the temporary test amalgamation contains fault point 660.
 */
#include "sqlite3.c"
#include <stdio.h>
#include <stdlib.h>

static sqlite3 *reader, *writer;
static int armed, fired;
static u32 oldFrames;

static void require(int ok, const char *message){
  if( !ok ){ fprintf(stderr, "FAIL: %s\n", message); exit(2); }
}
static void harnessExec(sqlite3 *db, const char *sql){
  char *error = 0;
  int rc = sqlite3_exec(db, sql, 0, 0, &error);
  if( rc!=SQLITE_OK ){
    fprintf(stderr, "SQL failed (%d): %s\n", rc, error ? error : sqlite3_errmsg(db));
    sqlite3_free(error);
    exit(3);
  }
}
static Wal *wal(sqlite3 *db){ return db->aDb[0].pBt->pBt->pPager->pWal; }
static int fault(int n){
  if( n==660 && armed ){
    Wal *r = wal(reader);
    u32 salts[2];
    memcpy(salts, r->hdr.aSalt, sizeof(salts));
    armed = 0;
    fired++;
    require(walCkptInfo(r)->nBackfill==r->hdr.mxFrame, "prior WAL not fully backfilled");
    oldFrames = r->hdr.mxFrame;
    harnessExec(writer, "UPDATE t SET b=printf('%0600d',200+a) WHERE a<5");
    require(salts[0]!=walIndexHdr(r)->aSalt[0] || salts[1]!=walIndexHdr(r)->aSalt[1], "writer did not reset WAL salts");
    require(walCkptInfo(r)->nBackfill==0, "writer did not reset nBackfill");
    require(walIndexHdr(r)->mxFrame>0 && walIndexHdr(r)->mxFrame<oldFrames, "new WAL not smaller than cached WAL");
    printf("reset: old=%u new=%u\n", oldFrames, walIndexHdr(r)->mxFrame);
  }
  return 0;
}
static void checkpoint(sqlite3 *db, int full){
  int log = -1, backfill = -1;
  int rc = sqlite3_wal_checkpoint_v2(db, 0, SQLITE_CHECKPOINT_PASSIVE, &log, &backfill);
  require(rc==SQLITE_OK, "checkpoint returned an error");
  if( full ) require(log>0 && log==backfill, "ordinary checkpoint did not finish");
}
static void verify(sqlite3 *db, int generation){
  sqlite3_stmt *s = 0;
  int i = 1, rc;
  require(sqlite3_prepare_v2(db,"SELECT a,b FROM t ORDER BY a",-1,&s,0)==SQLITE_OK,"prepare contents");
  while( (rc=sqlite3_step(s))==SQLITE_ROW ){
    char expected[601];
    int base = generation==3 ? 300 : (generation==2 && i<5 ? 200 : 100);
    snprintf(expected,sizeof(expected),"%0600d",base+i);
    require(sqlite3_column_int(s,0)==i && strcmp((const char*)sqlite3_column_text(s,1),expected)==0,"committed contents changed");
    i++;
  }
  require(rc==SQLITE_DONE && i==21,"committed row count changed");
  require(sqlite3_finalize(s)==SQLITE_OK,"finalize contents");
  require(sqlite3_prepare_v2(db,"PRAGMA integrity_check",-1,&s,0)==SQLITE_OK,"prepare integrity");
  require(sqlite3_step(s)==SQLITE_ROW && strcmp((const char*)sqlite3_column_text(s,0),"ok")==0,"integrity failure");
  require(sqlite3_step(s)==SQLITE_DONE,"additional integrity failure");
  require(sqlite3_finalize(s)==SQLITE_OK,"finalize integrity");
}
static void openDb(const char *path, int keyed, sqlite3 **out){
  require(sqlite3_open(path,out)==SQLITE_OK,"open DB");
  if( keyed ) harnessExec(*out,"PRAGMA key=\"x'4242424242424242424242424242424242424242424242424242424242424242'\"");
  harnessExec(*out,"PRAGMA wal_autocheckpoint=0");
}
int main(int argc, char **argv){
  int keyed;
  require(argc==3,"usage: harness path keyed");
  keyed = atoi(argv[2]);
  openDb(argv[1],keyed,&reader);
  /* SQLCipher's default codec page size is 4096. Changing only SQLite's
   * page_size after keying makes the fixture inconsistent with its codec. */
  if( !keyed ) harnessExec(reader,"PRAGMA page_size=1024");
  harnessExec(reader,"PRAGMA auto_vacuum=0; PRAGMA journal_mode=WAL; CREATE TABLE t(a INTEGER PRIMARY KEY,b);"
    "WITH RECURSIVE s(i) AS (SELECT 1 UNION ALL SELECT i+1 FROM s WHERE i<20) INSERT INTO t SELECT i,printf('%0600d',i) FROM s; CREATE INDEX idx ON t(b)");
  checkpoint(reader,1);
  harnessExec(reader,"UPDATE t SET b=printf('%0600d',100+a)");
  checkpoint(reader,1);
  verify(reader,1);
  openDb(argv[1],keyed,&writer);
  harnessExec(writer,"SELECT count(*) FROM t");
  sqlite3_test_control(SQLITE_TESTCTRL_FAULT_INSTALL,fault);
  armed = 1;
  checkpoint(reader,0);
  sqlite3_test_control(SQLITE_TESTCTRL_FAULT_INSTALL,0);
  require(fired==1,"reset callback not fired exactly once");
  /* This distinguishes the original defect from a setup/SQL/compile failure. */
  if( walCkptInfo(wal(reader))->nBackfill!=0 ){
    fprintf(stderr,"WAL_RESET_REGRESSION: stale generation backfilled\n");
    return 7;
  }
  verify(writer,2);
  harnessExec(writer,"UPDATE t SET b=printf('%0600d',300+a)");
  require(walIndexHdr(wal(writer))->mxFrame>oldFrames,"new WAL did not grow beyond stale frame count");
  checkpoint(reader,1);
  verify(reader,3);
  require(sqlite3_close(writer)==SQLITE_OK,"close writer");
  require(sqlite3_close(reader)==SQLITE_OK,"close reader");
  openDb(argv[1],keyed,&reader);
  verify(reader,3);
  if( keyed ){
    sqlite3_stmt *s = 0;
    require(sqlite3_prepare_v2(reader,"PRAGMA cipher_integrity_check",-1,&s,0)==SQLITE_OK,"prepare cipher integrity");
    require(sqlite3_step(s)==SQLITE_DONE,"cipher integrity failed");
    require(sqlite3_finalize(s)==SQLITE_OK,"finalize cipher integrity");
  }
  require(sqlite3_close(reader)==SQLITE_OK,"close reopened DB");
  puts("WAL_RESET_PASS");
  return 0;
}
