package main

import (
	"archive/tar"
	"compress/gzip"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestFindWorldLevelSaveSkipsRuntimeBackups(t *testing.T) {
	root := t.TempDir()
	active := filepath.Join(root, "Saved", "SaveGames", "0", "world", "Level.sav")
	backup := filepath.Join(root, "Saved", "SaveGames", "0", "world", "backup", "world", "old", "Level.sav")
	for _, path := range []string{active, backup} {
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte("save"), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	got, err := findWorldLevelSave(root)
	if err != nil || got != active {
		t.Fatalf("expected active save %q, got %q, err=%v", active, got, err)
	}
}

func TestLatestCompressedBackupUsesModTime(t *testing.T) {
	dir := t.TempDir()
	oldPath := filepath.Join(dir, "old.tar.gz")
	newPath := filepath.Join(dir, "new.tar.gz")
	for _, path := range []string{oldPath, newPath} {
		file, err := os.Create(path)
		if err != nil {
			t.Fatal(err)
		}
		gz := gzip.NewWriter(file)
		tw := tar.NewWriter(gz)
		_ = tw.Close()
		_ = gz.Close()
		_ = file.Close()
	}
	old := time.Now().Add(-time.Hour)
	_ = os.Chtimes(oldPath, old, old)
	got, _, err := latestCompressedBackup(dir)
	if err != nil || got != newPath {
		t.Fatalf("expected newest backup %q, got %q, err=%v", newPath, got, err)
	}
}

func TestLatestStableCompressedBackupSkipsRecentFile(t *testing.T) {
	dir := t.TempDir()
	now := time.Now()
	oldPath := filepath.Join(dir, "complete.tar.gz")
	recentPath := filepath.Join(dir, "still-writing.tar.gz")
	for _, path := range []string{oldPath, recentPath} {
		writeTarGz(t, path, map[string]string{"Level.sav": "save"})
	}
	oldTime := now.Add(-time.Minute)
	recentTime := now.Add(-5 * time.Second)
	_ = os.Chtimes(oldPath, oldTime, oldTime)
	_ = os.Chtimes(recentPath, recentTime, recentTime)
	got, _, err := latestStableCompressedBackup(dir, now, 30*time.Second)
	if err != nil || got != oldPath {
		t.Fatalf("expected stable backup %q, got %q, err=%v", oldPath, got, err)
	}
}

func TestRefreshWorldSnapshotCopiesCompletedBackup(t *testing.T) {
	root := t.TempDir()
	backupsDir := filepath.Join(root, "backups")
	stateDir := filepath.Join(root, "panel-state")
	snapshotDir := filepath.Join(root, "advanced-state", "world-snapshot")
	for _, dir := range []string{backupsDir, stateDir} {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			t.Fatal(err)
		}
	}
	backupPath := filepath.Join(backupsDir, "palworld-save-test.tar.gz")
	writeTarGz(t, backupPath, map[string]string{
		"Saved/SaveGames/0/world/Level.sav":          "level-data",
		"Saved/SaveGames/0/world/LevelMeta.sav":      "meta-data",
		"Saved/SaveGames/0/world/Players/player.sav": "player-data",
	})
	stableTime := time.Now().Add(-time.Minute)
	if err := os.Chtimes(backupPath, stableTime, stableTime); err != nil {
		t.Fatal(err)
	}

	app := &App{cfg: Config{BackupsDir: backupsDir, StateDir: stateDir, WorldSnapshot: snapshotDir}}
	snapshot, err := app.refreshWorldSnapshot()
	if err != nil {
		t.Fatal(err)
	}
	if snapshot.BackupID != filepath.Base(backupPath) || snapshot.ID == "" {
		t.Fatalf("unexpected snapshot metadata: %#v", snapshot)
	}
	if snapshot.Fingerprint == "" {
		t.Fatal("snapshot fingerprint was not recorded")
	}
	assertFileContent(t, filepath.Join(snapshotDir, "Level.sav"), "level-data")
	assertFileContent(t, filepath.Join(snapshotDir, "Players", "player.sav"), "player-data")
	if _, err := os.Stat(filepath.Join(stateDir, "world-index-snapshot.json")); err != nil {
		t.Fatalf("snapshot metadata was not written: %v", err)
	}
}

func TestWorldSnapshotAvailableUsesLevelSave(t *testing.T) {
	dir := t.TempDir()
	app := &App{cfg: Config{WorldSnapshot: dir}}
	if app.worldSnapshotAvailable() {
		t.Fatal("empty snapshot directory must not be available")
	}
	if err := os.WriteFile(filepath.Join(dir, "Level.sav"), []byte("save"), 0o600); err != nil {
		t.Fatal(err)
	}
	if !app.worldSnapshotAvailable() {
		t.Fatal("snapshot with Level.sav must be available")
	}
}

func TestWorldSnapshotFingerprintTracksPlayerSaves(t *testing.T) {
	dir := t.TempDir()
	playersDir := filepath.Join(dir, "Players")
	if err := os.MkdirAll(playersDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "Level.sav"), []byte("same-level"), 0o600); err != nil {
		t.Fatal(err)
	}
	playerPath := filepath.Join(playersDir, "player.sav")
	if err := os.WriteFile(playerPath, []byte("old-player"), 0o600); err != nil {
		t.Fatal(err)
	}

	before, err := worldSnapshotFingerprint(dir)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(playerPath, []byte("new-player"), 0o600); err != nil {
		t.Fatal(err)
	}
	after, err := worldSnapshotFingerprint(dir)
	if err != nil {
		t.Fatal(err)
	}
	if before == after {
		t.Fatal("changing only a player save must change the snapshot fingerprint")
	}
}

func TestWorldSnapshotNeedsRefreshTracksLatestBackup(t *testing.T) {
	root := t.TempDir()
	backupsDir := filepath.Join(root, "backups")
	stateDir := filepath.Join(root, "panel-state")
	snapshotDir := filepath.Join(root, "world-snapshot")
	for _, dir := range []string{backupsDir, stateDir, snapshotDir} {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			t.Fatal(err)
		}
	}
	oldPath := filepath.Join(backupsDir, "palworld-save-old.tar.gz")
	writeTarGz(t, oldPath, map[string]string{"Level.sav": "old"})
	oldTime := time.Now().Add(-2 * time.Minute)
	if err := os.Chtimes(oldPath, oldTime, oldTime); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(snapshotDir, "Level.sav"), []byte("old"), 0o600); err != nil {
		t.Fatal(err)
	}

	app := &App{cfg: Config{BackupsDir: backupsDir, StateDir: stateDir, WorldSnapshot: snapshotDir}}
	fingerprint, err := worldSnapshotFingerprint(snapshotDir)
	if err != nil {
		t.Fatal(err)
	}
	metadata, err := json.Marshal(WorldSnapshot{ID: "snapshot", BackupID: filepath.Base(oldPath), Fingerprint: fingerprint})
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(app.worldSnapshotFile(), metadata, 0o600); err != nil {
		t.Fatal(err)
	}
	needsRefresh, latestBackupID, err := app.worldSnapshotNeedsRefresh()
	if err != nil || needsRefresh || latestBackupID != filepath.Base(oldPath) {
		t.Fatalf("expected current snapshot, refresh=%v latest=%q err=%v", needsRefresh, latestBackupID, err)
	}

	newPath := filepath.Join(backupsDir, "palworld-save-new.tar.gz")
	writeTarGz(t, newPath, map[string]string{"Level.sav": "new"})
	newTime := time.Now().Add(-time.Minute)
	if err := os.Chtimes(newPath, newTime, newTime); err != nil {
		t.Fatal(err)
	}
	needsRefresh, latestBackupID, err = app.worldSnapshotNeedsRefresh()
	if err != nil || !needsRefresh || latestBackupID != filepath.Base(newPath) {
		t.Fatalf("expected newer backup, refresh=%v latest=%q err=%v", needsRefresh, latestBackupID, err)
	}
}

func TestWaitForWorldIndexRequiresMatchingFingerprint(t *testing.T) {
	server := newWorldIndexStatusServer(t, map[string]any{
		"ready": true, "syncing": false, "stale": false,
		"fingerprint": "current", "players": 7, "guilds": 2,
	})
	defer server.Close()

	app := &App{cfg: Config{WorldIndexURL: server.URL, WorldIndexPass: "secret"}}
	status, err := app.waitForWorldIndex("current", time.Second)
	if err != nil {
		t.Fatal(err)
	}
	if status.Players != 7 || status.Guilds != 2 {
		t.Fatalf("unexpected index counts: %#v", status)
	}
	if _, err := app.waitForWorldIndex("different", 20*time.Millisecond); err == nil {
		t.Fatal("mismatched fingerprint must not be accepted as current")
	}
}

func TestWaitForWorldIndexReturnsParserFailure(t *testing.T) {
	server := newWorldIndexStatusServer(t, map[string]any{
		"ready": true, "syncing": false, "stale": true,
		"last_error": "new save format is unsupported", "fingerprint": "old",
	})
	defer server.Close()

	app := &App{cfg: Config{WorldIndexURL: server.URL, WorldIndexPass: "secret"}}
	_, err := app.waitForWorldIndex("current", time.Second)
	if err == nil || !strings.Contains(err.Error(), "new save format is unsupported") {
		t.Fatalf("expected parser error, got %v", err)
	}
}

func newWorldIndexStatusServer(t *testing.T, status map[string]any) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/login":
			writeJSON(w, http.StatusOK, map[string]string{"token": "test-token"})
		case "/api/status":
			if r.Header.Get("Authorization") != "Bearer test-token" {
				t.Fatalf("missing world-index bearer token")
			}
			writeJSON(w, http.StatusOK, status)
		default:
			http.NotFound(w, r)
		}
	}))
}

func TestNormalizeAnnouncementSupportsChinese(t *testing.T) {
	got, err := normalizeAnnouncement("  服务器将在十分钟后维护  ")
	if err != nil || got != "服务器将在十分钟后维护" {
		t.Fatalf("unexpected announcement: %q err=%v", got, err)
	}
	if _, err := normalizeAnnouncement("   "); err == nil {
		t.Fatal("expected empty announcement to fail")
	}
	if _, err := normalizeAnnouncement(strings.Repeat("测", maxAnnouncementRunes+1)); err == nil {
		t.Fatal("expected oversized announcement to fail")
	}
	if isAllowedRcon("Broadcast 服务器维护") {
		t.Fatal("broadcast must use the UTF-8 REST announcement endpoint")
	}
}

func writeTarGz(t *testing.T, path string, files map[string]string) {
	t.Helper()
	file, err := os.Create(path)
	if err != nil {
		t.Fatal(err)
	}
	gz := gzip.NewWriter(file)
	tw := tar.NewWriter(gz)
	for name, content := range files {
		header := &tar.Header{Name: name, Mode: 0o600, Size: int64(len(content)), Typeflag: tar.TypeReg}
		if err := tw.WriteHeader(header); err != nil {
			t.Fatal(err)
		}
		if _, err := io.Copy(tw, strings.NewReader(content)); err != nil {
			t.Fatal(err)
		}
	}
	if err := tw.Close(); err != nil {
		t.Fatal(err)
	}
	if err := gz.Close(); err != nil {
		t.Fatal(err)
	}
	if err := file.Close(); err != nil {
		t.Fatal(err)
	}
}

func assertFileContent(t *testing.T, path, want string) {
	t.Helper()
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if string(raw) != want {
		t.Fatalf("unexpected contents for %s: %q", path, raw)
	}
}
