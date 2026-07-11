package main

import (
	"archive/tar"
	"compress/gzip"
	"context"
	"fmt"
	"io"
	"io/fs"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

func (a *App) backupRows(ctx context.Context, limit int) []Backup {
	entries, err := os.ReadDir(a.cfg.BackupsDir)
	if err != nil {
		return []Backup{}
	}
	rows := make([]Backup, 0)
	for _, entry := range entries {
		fullPath := filepath.Join(a.cfg.BackupsDir, entry.Name())
		info, err := entry.Info()
		if err != nil {
			continue
		}
		size := info.Size()
		format := backupFormatFor(entry.Name(), entry.IsDir())
		restorable := format == "directory" || format == "tar.gz"
		backupType := "automatic"
		if strings.Contains(strings.ToLower(entry.Name()), "manual") {
			backupType = "manual"
		}
		if entry.IsDir() {
			size = pathSizeBytes(ctx, fullPath)
		}
		note := "压缩备份，可由面板停服后恢复。"
		if entry.IsDir() {
			note = "目录备份，可由面板停服后恢复。"
		} else if !restorable {
			note = "不支持的备份格式，仅展示。"
		}
		status := "ready"
		if !restorable {
			status = "failed"
		}
		rows = append(rows, Backup{
			ID: entry.Name(), CreatedAt: formatTime(info.ModTime()), Size: formatBytes(size),
			Type: backupType, Status: status, Format: format, Restorable: restorable, Note: note,
		})
	}
	sort.SliceStable(rows, func(i, j int) bool { return rows[i].CreatedAt > rows[j].CreatedAt })
	if len(rows) > limit {
		rows = rows[:limit]
	}
	return rows
}

func (a *App) createBackup(actor string) (map[string]any, error) {
	op := a.startOperation("backup:create", actor, nil)
	id := "manual-" + time.Now().Format("20060102-150405")
	target := filepath.Join(a.cfg.BackupsDir, id)
	tempTarget := target + ".tmp"
	if err := os.MkdirAll(a.cfg.BackupsDir, 0o755); err != nil {
		a.finishOperation(op, "failed", err.Error())
		return nil, err
	}
	_ = os.RemoveAll(tempTarget)
	saveWarning := ""
	if _, err := a.executeRcon("Save", a.cfg.RconTimeout); err != nil {
		saveWarning = "（RCON Save 失败，已直接复制当前存档: " + err.Error() + "）"
		a.audit("warn", "backup", "创建手动备份前 Save 失败，已继续复制当前存档: "+err.Error(), actor, nil)
	}
	if err := copyDir(a.cfg.SavesDir, tempTarget); err != nil {
		_ = os.RemoveAll(tempTarget)
		a.finishOperation(op, "failed", err.Error())
		return nil, err
	}
	if err := os.Rename(tempTarget, target); err != nil {
		_ = os.RemoveAll(tempTarget)
		a.finishOperation(op, "failed", err.Error())
		return nil, err
	}
	message := "已创建手动备份 " + id + saveWarning
	a.finishOperation(op, "success", message)
	a.audit("info", "backup", "已创建手动备份 "+id, actor, map[string]any{"backupId": id})
	return map[string]any{"ok": true, "message": message}, nil
}

func (a *App) restoreBackup(id, actor string) (map[string]any, error) {
	if !safeBackupID(id) {
		return nil, APIError{Status: http.StatusBadRequest, Message: "非法备份 ID"}
	}
	op := a.startOperation("backup:restore", actor, map[string]any{"backupId": id})
	source := filepath.Join(a.cfg.BackupsDir, id)
	info, err := os.Stat(source)
	if err != nil {
		a.finishOperation(op, "failed", err.Error())
		return nil, err
	}
	format := backupFormatFor(id, info.IsDir())
	if format != "directory" && format != "tar.gz" {
		err := fmt.Errorf("不支持恢复该备份格式: %s", format)
		a.finishOperation(op, "failed", err.Error())
		return nil, err
	}
	_, _ = a.executeRcon("Save", a.cfg.RconTimeout)
	stopCtx, stopCancel := context.WithTimeout(context.Background(), 45*time.Second)
	defer stopCancel()
	if _, err := a.runCompose(stopCtx, "stop", "palworld"); err != nil {
		a.finishOperation(op, "failed", err.Error())
		return nil, fmt.Errorf("停止游戏容器失败: %w", err)
	}
	failAfterStop := func(err error) (map[string]any, error) {
		restartCtx, restartCancel := context.WithTimeout(context.Background(), 70*time.Second)
		defer restartCancel()
		if _, restartErr := a.runCompose(restartCtx, "up", "-d", "--force-recreate", "palworld"); restartErr != nil {
			err = fmt.Errorf("%w；并且恢复失败后重新启动游戏容器也失败: %v", err, restartErr)
		}
		a.finishOperation(op, "failed", err.Error())
		return nil, err
	}
	clearTarget, restoreTarget := a.restorePathsForBackup(format)
	preRestore := filepath.Join(a.cfg.BackupsDir, "pre-restore-"+time.Now().Format("20060102-150405"))
	_ = copyDir(clearTarget, preRestore)
	if err := os.RemoveAll(clearTarget); err != nil {
		return failAfterStop(err)
	}
	if err := os.MkdirAll(clearTarget, 0o755); err != nil {
		return failAfterStop(err)
	}
	if err := restoreBackupSource(source, restoreTarget, format); err != nil {
		_ = os.RemoveAll(clearTarget)
		_ = copyDir(preRestore, clearTarget)
		return failAfterStop(err)
	}
	startCtx, startCancel := context.WithTimeout(context.Background(), 70*time.Second)
	defer startCancel()
	if _, err := a.runCompose(startCtx, "up", "-d", "--force-recreate", "palworld"); err != nil {
		a.finishOperation(op, "failed", err.Error())
		return nil, fmt.Errorf("备份已恢复，但重启游戏容器失败: %w", err)
	}
	a.finishOperation(op, "success", "已恢复 "+id)
	a.audit("warn", "backup", "已恢复备份 "+id+" 并重建游戏容器。", actor, map[string]any{"backupId": id, "format": format, "preRestore": filepath.Base(preRestore)})
	return map[string]any{"ok": true, "message": "已恢复 " + id + "，游戏容器正在按恢复后的存档启动"}, nil
}

func (a *App) maintenance(action, actor string) (map[string]any, error) {
	if action == "" {
		return nil, APIError{Status: http.StatusBadRequest, Message: "维护动作不能为空"}
	}
	op := a.startOperation(action, actor, nil)
	fail := func(err error) (map[string]any, error) {
		a.finishOperation(op, "failed", err.Error())
		return nil, err
	}
	success := func(message string) (map[string]any, error) {
		a.finishOperation(op, "success", message)
		return map[string]any{"ok": true, "message": message}, nil
	}
	switch {
	case action == "rcon:save":
		result, err := a.executeRcon("Save", a.cfg.RconTimeout)
		if err != nil {
			return fail(err)
		}
		a.audit("info", "rcon", "已执行 Save", actor, nil)
		return success(result.Output)
	case action == "server:shutdown":
		result, err := a.executeRcon("Shutdown 300 Server_shutdown_in_5_minutes", a.cfg.RconTimeout)
		if err != nil {
			return fail(err)
		}
		a.audit("warn", "rcon", "已提交延迟关服命令", actor, nil)
		return success(result.Output)
	case action == "server:restart":
		ctx, cancel := context.WithTimeout(context.Background(), 70*time.Second)
		defer cancel()
		if _, err := a.runCompose(ctx, "up", "-d", "--force-recreate", "palworld"); err != nil {
			return fail(err)
		}
		a.audit("warn", "server", "已按当前 .env 重建容器 "+a.cfg.Container, actor, nil)
		return success("游戏容器已按最新配置重建，Palworld 正在启动")
	case action == "server:update":
		ctx, cancel := context.WithTimeout(context.Background(), 4*time.Minute)
		defer cancel()
		if _, err := a.runCompose(ctx, "pull", "palworld"); err != nil {
			return fail(err)
		}
		if _, err := a.runCompose(ctx, "up", "-d", "--force-recreate", "palworld"); err != nil {
			return fail(err)
		}
		a.audit("warn", "update", "已执行服务端更新流程，Compose 项目: "+a.cfg.ComposeProject, actor, nil)
		return success("更新容器已重建，Palworld 正在完成启动与健康检查")
	case action == "backup:create":
		a.finishOperation(op, "delegated", "转入备份创建流程")
		return a.createBackup(actor)
	case strings.HasPrefix(action, "backup:restore:"):
		a.finishOperation(op, "delegated", "转入备份恢复流程")
		return a.restoreBackup(strings.TrimPrefix(action, "backup:restore:"), actor)
	default:
		return fail(APIError{Status: http.StatusBadRequest, Message: "未知维护动作: " + action})
	}
}

func (a *App) runCompose(ctx context.Context, args ...string) ([]byte, error) {
	base := []string{"compose", "-p", a.cfg.ComposeProject}
	return runCmd(ctx, a.cfg.ComposeDir, "docker", append(base, args...)...)
}

func copyDir(src, dst string) error {
	info, err := os.Stat(src)
	if err != nil {
		return err
	}
	if !info.IsDir() {
		return fmt.Errorf("%s 不是目录", src)
	}
	return filepath.WalkDir(src, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			if os.IsNotExist(err) {
				return nil
			}
			return err
		}
		rel, err := filepath.Rel(src, path)
		if err != nil {
			return err
		}
		target := filepath.Join(dst, rel)
		if d.IsDir() {
			return os.MkdirAll(target, 0o755)
		}
		if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
			return err
		}
		in, err := os.Open(path)
		if err != nil {
			if os.IsNotExist(err) {
				return nil
			}
			return err
		}
		defer in.Close()
		out, err := os.OpenFile(target, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o644)
		if err != nil {
			return err
		}
		defer out.Close()
		_, err = io.Copy(out, in)
		return err
	})
}

func backupFormatFor(name string, isDir bool) string {
	if isDir {
		return "directory"
	}
	lower := strings.ToLower(name)
	if strings.HasSuffix(lower, ".tar.gz") || strings.HasSuffix(lower, ".tgz") {
		return "tar.gz"
	}
	return "file"
}

func (a *App) restorePathsForBackup(format string) (clearTarget string, restoreTarget string) {
	if format == "tar.gz" {
		savedDir := filepath.Dir(a.cfg.SavesDir)
		return savedDir, filepath.Dir(savedDir)
	}
	return a.cfg.SavesDir, a.cfg.SavesDir
}

func restoreBackupSource(source, destination, format string) error {
	switch format {
	case "directory":
		return copyDir(source, destination)
	case "tar.gz":
		return extractTarGz(source, destination)
	default:
		return fmt.Errorf("不支持恢复该备份格式: %s", format)
	}
}

func extractTarGz(source, destination string) error {
	file, err := os.Open(source)
	if err != nil {
		return err
	}
	defer file.Close()
	gzipReader, err := gzip.NewReader(file)
	if err != nil {
		return err
	}
	defer gzipReader.Close()
	tarReader := tar.NewReader(gzipReader)
	for {
		header, err := tarReader.Next()
		if err == io.EOF {
			return nil
		}
		if err != nil {
			return err
		}
		if header == nil {
			continue
		}
		if err := extractTarEntry(tarReader, header, destination); err != nil {
			return err
		}
	}
}

func extractTarEntry(reader io.Reader, header *tar.Header, destination string) error {
	name := strings.TrimPrefix(filepath.Clean(header.Name), string(filepath.Separator))
	if name == "." || name == "" {
		return nil
	}
	target := filepath.Join(destination, name)
	destAbs, _ := filepath.Abs(destination)
	targetAbs, _ := filepath.Abs(target)
	if targetAbs != destAbs && !strings.HasPrefix(targetAbs, destAbs+string(filepath.Separator)) {
		return fmt.Errorf("备份包含非法路径: %s", header.Name)
	}
	switch header.Typeflag {
	case tar.TypeDir:
		return os.MkdirAll(target, 0o755)
	case tar.TypeReg, tar.TypeRegA:
		if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
			return err
		}
		mode := fs.FileMode(header.Mode) & 0o777
		if mode == 0 {
			mode = 0o644
		}
		out, err := os.OpenFile(target, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, mode)
		if err != nil {
			return err
		}
		defer out.Close()
		_, err = io.Copy(out, reader)
		return err
	default:
		return nil
	}
}
