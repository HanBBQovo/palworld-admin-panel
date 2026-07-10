package main

import (
	"bufio"
	"context"
	"encoding/binary"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

func (a *App) executeRcon(command string, timeout time.Duration) (RconCommandResult, error) {
	command = strings.TrimSpace(command)
	if command == "" {
		return RconCommandResult{}, APIError{Status: http.StatusBadRequest, Message: "RCON 命令不能为空"}
	}
	if !a.cfg.AllowRawRcon && !isAllowedRcon(command) {
		return RconCommandResult{}, APIError{Status: http.StatusBadRequest, Message: "该 RCON 命令不在白名单内；如需开放任意命令，请设置 PANEL_ALLOW_RAW_RCON=true"}
	}
	password := a.cfg.RconPassword
	if savedPassword := strings.TrimSpace(a.readSettings().AdminPassword); savedPassword != "" {
		password = savedPassword
	}
	if password == "" {
		return RconCommandResult{}, errors.New("PALWORLD_ADMIN_PASSWORD 未配置，无法连接 RCON")
	}
	output, err := runRcon(a.cfg.RconHost, a.cfg.RconPort, password, command, timeout)
	if err != nil {
		return RconCommandResult{}, err
	}
	return RconCommandResult{Command: command, Output: output, ExecutedAt: formatTime(time.Now())}, nil
}

func isAllowedRcon(command string) bool {
	lower := strings.ToLower(command)
	for _, prefix := range allowedRconPrefixes {
		p := strings.ToLower(prefix)
		if lower == p || strings.HasPrefix(lower, p+" ") {
			return true
		}
	}
	return false
}

func runRcon(host string, port int, password, command string, timeout time.Duration) (string, error) {
	if timeout <= 0 {
		timeout = 1800 * time.Millisecond
	}
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()
	conn, err := (&net.Dialer{Timeout: minDuration(800*time.Millisecond, timeout)}).DialContext(ctx, "tcp", net.JoinHostPort(host, strconv.Itoa(port)))
	if err != nil {
		return "", fmt.Errorf("RCON 连接失败: %w", err)
	}
	defer conn.Close()
	deadline := time.Now().Add(timeout)
	_ = conn.SetDeadline(deadline)
	if err := writeRconPacket(conn, 1, 3, password); err != nil {
		return "", err
	}
	authed := false
	for time.Now().Before(deadline) {
		id, _, _, err := readRconPacket(conn)
		if err != nil {
			return "", fmt.Errorf("RCON 鉴权超时")
		}
		if id == -1 {
			return "", fmt.Errorf("RCON 鉴权失败，请检查管理员密码")
		}
		if id == 1 {
			authed = true
			break
		}
	}
	if !authed {
		return "", fmt.Errorf("RCON 鉴权超时")
	}
	if err := writeRconPacket(conn, 2, 2, command); err != nil {
		return "", err
	}
	_ = writeRconPacket(conn, 3, 2, "")
	var output strings.Builder
	gotBody := false
	for time.Now().Before(deadline) {
		if gotBody {
			_ = conn.SetReadDeadline(time.Now().Add(180 * time.Millisecond))
		} else {
			_ = conn.SetReadDeadline(deadline)
		}
		id, _, body, err := readRconPacket(conn)
		if err != nil {
			if gotBody && isTimeout(err) {
				break
			}
			return "", fmt.Errorf("RCON 连接超时")
		}
		if body != "" {
			output.WriteString(body)
			gotBody = true
		}
		if id == 3 && gotBody {
			break
		}
	}
	text := strings.TrimSpace(output.String())
	if text == "" {
		return "OK", nil
	}
	return text, nil
}

func writeRconPacket(w io.Writer, id, typ int32, body string) error {
	bodyBytes := []byte(body)
	size := int32(4 + 4 + len(bodyBytes) + 2)
	buf := make([]byte, 4+size)
	binary.LittleEndian.PutUint32(buf[0:4], uint32(size))
	binary.LittleEndian.PutUint32(buf[4:8], uint32(id))
	binary.LittleEndian.PutUint32(buf[8:12], uint32(typ))
	copy(buf[12:], bodyBytes)
	_, err := w.Write(buf)
	return err
}

func readRconPacket(r io.Reader) (int32, int32, string, error) {
	var sizeBuf [4]byte
	if _, err := io.ReadFull(r, sizeBuf[:]); err != nil {
		return 0, 0, "", err
	}
	size := int(binary.LittleEndian.Uint32(sizeBuf[:]))
	if size < 10 || size > 1024*1024 {
		return 0, 0, "", fmt.Errorf("非法 RCON 包长度: %d", size)
	}
	buf := make([]byte, size)
	if _, err := io.ReadFull(r, buf); err != nil {
		return 0, 0, "", err
	}
	id := int32(binary.LittleEndian.Uint32(buf[0:4]))
	typ := int32(binary.LittleEndian.Uint32(buf[4:8]))
	bodyLen := size - 10
	body := ""
	if bodyLen > 0 {
		body = strings.TrimRight(string(buf[8:8+bodyLen]), "\x00")
	}
	return id, typ, body, nil
}

func parsePlayers(output string) []Player {
	players := make([]Player, 0)
	for _, line := range strings.Split(output, "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(strings.ToLower(line), "name,") {
			continue
		}
		parts := strings.Split(line, ",")
		if len(parts) < 3 {
			continue
		}
		name := strings.TrimSpace(parts[0])
		playerID := strings.TrimSpace(parts[1])
		steamID := strings.TrimSpace(parts[2])
		if name == "" || strings.EqualFold(name, "no online players") {
			continue
		}
		id := playerID
		if id == "" {
			id = steamID
		}
		if id == "" {
			id = fmt.Sprintf("player-%d", len(players)+1)
		}
		platform := "Unknown"
		if steamID != "" && steamID != "-" {
			platform = "Steam"
		}
		players = append(players, Player{
			ID:        id,
			Name:      name,
			PlayerUID: orDefault(playerID, "-"),
			Platform:  platform,
			SteamID:   orDefault(steamID, "-"),
			Status:    "online",
		})
	}
	return players
}

func (a *App) audit(level, source, message, actor string, metadata map[string]any) {
	if metadata == nil {
		metadata = map[string]any{}
	}
	row := LogEntry{
		ID: randomID(), Timestamp: time.Now().Format(time.RFC3339), Level: level,
		Source: source, Message: message, Actor: actor, Metadata: metadata,
	}
	a.auditMu.Lock()
	defer a.auditMu.Unlock()
	_ = os.MkdirAll(filepath.Dir(a.cfg.AuditFile), 0o755)
	file, err := os.OpenFile(a.cfg.AuditFile, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o600)
	if err != nil {
		return
	}
	defer file.Close()
	_ = os.Chmod(a.cfg.AuditFile, 0o600)
	raw, _ := json.Marshal(row)
	_, _ = file.Write(append(raw, '\n'))
}

func (a *App) auditRows(limit int) []LogEntry {
	file, err := os.Open(a.cfg.AuditFile)
	if err != nil {
		return []LogEntry{}
	}
	defer file.Close()
	rows := make([]LogEntry, 0)
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		var row LogEntry
		if err := json.Unmarshal(scanner.Bytes(), &row); err == nil {
			row.Timestamp = formatTimeString(row.Timestamp)
			rows = append(rows, row)
		}
	}
	if len(rows) > limit {
		rows = rows[len(rows)-limit:]
	}
	return rows
}

func (a *App) startOperation(action, actor string, metadata map[string]any) string {
	id := randomID()
	a.writeOperation(OperationRecord{ID: id, Action: action, Status: "running", Actor: actor, Metadata: metadata, CreatedAt: time.Now().Format(time.RFC3339)})
	return id
}

func (a *App) finishOperation(id, status, message string) {
	a.writeOperation(OperationRecord{ID: id, Status: status, Message: message, CompletedAt: time.Now().Format(time.RFC3339)})
}

func (a *App) writeOperation(row OperationRecord) {
	a.opsMu.Lock()
	defer a.opsMu.Unlock()
	_ = os.MkdirAll(filepath.Dir(a.cfg.OpsFile), 0o755)
	file, err := os.OpenFile(a.cfg.OpsFile, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o600)
	if err != nil {
		return
	}
	defer file.Close()
	_ = os.Chmod(a.cfg.OpsFile, 0o600)
	raw, _ := json.Marshal(row)
	_, _ = file.Write(append(raw, '\n'))
}
