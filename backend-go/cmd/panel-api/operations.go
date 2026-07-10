package main

import (
	"context"
	"errors"
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
		if entry.IsDir() {
			size = pathSizeBytes(ctx, fullPath)
		}
		backupType := "automatic"
		if strings.Contains(strings.ToLower(entry.Name()), "manual") {
			backupType = "manual"
		}
		note := "文件备份，恢复前需要手动解包。"
		if entry.IsDir() {
			note = "目录备份，可直接恢复。"
		}
		rows = append(rows, Backup{ID: entry.Name(), CreatedAt: formatTime(info.ModTime()), Size: formatBytes(size), Type: backupType, Status: "ready", Note: note})
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
	if err := os.MkdirAll(a.cfg.BackupsDir, 0o755); err != nil {
		a.finishOperation(op, "failed", err.Error())
		return nil, err
	}
	_, _ = a.executeRcon("Save", a.cfg.RconTimeout)
	if err := copyDir(a.cfg.SavesDir, target); err != nil {
		a.finishOperation(op, "failed", err.Error())
		return nil, err
	}
	a.finishOperation(op, "success", "已创建手动备份 "+id)
	a.audit("info", "backup", "已创建手动备份 "+id, actor, map[string]any{"backupId": id})
	return map[string]any{"ok": true, "message": "已创建手动备份 " + id}, nil
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
	if !info.IsDir() {
		err := errors.New("当前只支持恢复目录备份")
		a.finishOperation(op, "failed", err.Error())
		return nil, err
	}
	preRestore := filepath.Join(a.cfg.BackupsDir, "pre-restore-"+time.Now().Format("20060102-150405"))
	_ = copyDir(a.cfg.SavesDir, preRestore)
	if err := os.RemoveAll(a.cfg.SavesDir); err != nil {
		a.finishOperation(op, "failed", err.Error())
		return nil, err
	}
	if err := copyDir(source, a.cfg.SavesDir); err != nil {
		a.finishOperation(op, "failed", err.Error())
		return nil, err
	}
	a.finishOperation(op, "success", "已恢复 "+id)
	a.audit("warn", "backup", "已恢复备份 "+id+"；重启游戏容器后生效。", actor, map[string]any{"backupId": id})
	return map[string]any{"ok": true, "message": "已恢复 " + id + "，建议立即重启游戏容器"}, nil
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
		result, err := a.executeRcon("Shutdown 300 服务器将在5分钟后关闭", a.cfg.RconTimeout)
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
		if _, err := a.runCompose(ctx, "up", "-d", "palworld"); err != nil {
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
