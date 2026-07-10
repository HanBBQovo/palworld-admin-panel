package main

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math"
	"net"
	"net/http"
	"os"
	"os/exec"
	pathpkg "path"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"time"
)

func (a *App) serveSPA(w http.ResponseWriter, r *http.Request) {
	if strings.HasPrefix(r.URL.Path, "/api/") {
		writeError(w, APIError{Status: http.StatusNotFound, Message: "接口不存在"})
		return
	}
	clean := pathpkg.Clean("/" + r.URL.Path)
	if hasDotPath(clean) {
		http.NotFound(w, r)
		return
	}
	if a.cfg.WebRoot == "" {
		http.NotFound(w, r)
		return
	}
	rel := strings.TrimPrefix(clean, "/")
	target := filepath.Join(a.cfg.WebRoot, filepath.FromSlash(rel))
	rootAbs, _ := filepath.Abs(a.cfg.WebRoot)
	targetAbs, _ := filepath.Abs(target)
	if !strings.HasPrefix(targetAbs, rootAbs) {
		http.NotFound(w, r)
		return
	}
	if info, err := os.Stat(targetAbs); err == nil && !info.IsDir() {
		http.ServeFile(w, r, targetAbs)
		return
	}
	if strings.HasPrefix(clean, "/assets/") {
		http.NotFound(w, r)
		return
	}
	index := filepath.Join(a.cfg.WebRoot, "index.html")
	if _, err := os.Stat(index); err != nil {
		http.NotFound(w, r)
		return
	}
	http.ServeFile(w, r, index)
}

func hasDotPath(path string) bool {
	for _, part := range strings.Split(path, "/") {
		if strings.HasPrefix(part, ".") && part != "." {
			return true
		}
	}
	return false
}

func readJSON(r *http.Request, out any) error {
	defer r.Body.Close()
	decoder := json.NewDecoder(io.LimitReader(r.Body, 1<<20))
	decoder.DisallowUnknownFields()
	return decoder.Decode(out)
}

func writeJSON(w http.ResponseWriter, status int, payload any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(payload)
}

func writeError(w http.ResponseWriter, err error) {
	status := http.StatusInternalServerError
	message := "Server error"
	var apiErr APIError
	if errors.As(err, &apiErr) {
		status = apiErr.Status
		message = apiErr.Message
	} else if err != nil {
		message = err.Error()
	}
	writeJSON(w, status, map[string]any{"error": message})
}

func runCmd(ctx context.Context, dir, name string, args ...string) ([]byte, error) {
	cmd := exec.CommandContext(ctx, name, args...)
	if dir != "" {
		cmd.Dir = dir
	}
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	output, err := cmd.Output()
	if err != nil {
		msg := strings.TrimSpace(stderr.String())
		if msg != "" {
			return output, fmt.Errorf("%w: %s", err, msg)
		}
		return output, err
	}
	return output, nil
}

type diskInfo struct{ UsedGB, TotalGB float64 }

func diskUsage(ctx context.Context, target string) diskInfo {
	cctx, cancel := context.WithTimeout(ctx, 1800*time.Millisecond)
	defer cancel()
	output, err := runCmd(cctx, "", "df", "-Pk", target)
	if err != nil {
		return diskInfo{}
	}
	return parseDiskUsage(string(output))
}

func parseDiskUsage(output string) diskInfo {
	lines := strings.Split(strings.TrimSpace(output), "\n")
	if len(lines) < 2 {
		return diskInfo{}
	}
	fields := strings.Fields(lines[len(lines)-1])
	if len(fields) < 5 {
		return diskInfo{}
	}
	totalIndex := len(fields) - 5
	usedIndex := len(fields) - 4
	total, _ := strconv.ParseFloat(fields[totalIndex], 64)
	used, _ := strconv.ParseFloat(fields[usedIndex], 64)
	return diskInfo{UsedGB: round1(used / 1024 / 1024), TotalGB: round1(total / 1024 / 1024)}
}

func pathSizeGB(ctx context.Context, target string) float64 {
	return round2(float64(pathSizeBytes(ctx, target)) / 1024 / 1024 / 1024)
}

func pathSizeBytes(ctx context.Context, target string) int64 {
	cctx, cancel := context.WithTimeout(ctx, 2500*time.Millisecond)
	defer cancel()
	output, err := runCmd(cctx, "", "du", "-sk", target)
	if err != nil {
		return 0
	}
	fields := strings.Fields(string(output))
	if len(fields) == 0 {
		return 0
	}
	kb, _ := strconv.ParseInt(fields[0], 10, 64)
	return kb * 1024
}

func parseHostCPU(value string) float64 {
	clean := strings.TrimSpace(strings.TrimSuffix(value, "%"))
	raw, _ := strconv.ParseFloat(clean, 64)
	if raw <= 0 {
		return 0
	}
	cores := runtime.NumCPU()
	if cores <= 0 {
		cores = 1
	}
	return round1(raw / float64(cores))
}

func parseMemoryUsage(value string) (float64, float64) {
	parts := strings.Split(value, "/")
	if len(parts) != 2 {
		return 0, 0
	}
	return round1(memoryToGB(parts[0])), round1(memoryToGB(parts[1]))
}

func memoryToGB(value string) float64 {
	amount, unit := splitNumberUnit(value)
	if amount == 0 {
		return 0
	}
	switch {
	case strings.HasPrefix(unit, "ki"), unit == "kb":
		return amount / 1024 / 1024
	case strings.HasPrefix(unit, "mi"), unit == "mb":
		return amount / 1024
	case strings.HasPrefix(unit, "gi"), unit == "gb":
		return amount
	case strings.HasPrefix(unit, "ti"), unit == "tb":
		return amount * 1024
	default:
		return amount / 1024 / 1024 / 1024
	}
}

func splitNumberUnit(value string) (float64, string) {
	text := strings.TrimSpace(value)
	if text == "" {
		return 0, ""
	}
	end := 0
	for end < len(text) {
		c := text[end]
		if (c >= '0' && c <= '9') || c == '.' {
			end++
			continue
		}
		break
	}
	if end == 0 {
		return 0, strings.ToLower(strings.TrimSpace(text))
	}
	amount, err := strconv.ParseFloat(text[:end], 64)
	if err != nil {
		return 0, strings.ToLower(strings.TrimSpace(text[end:]))
	}
	return amount, strings.ToLower(strings.TrimSpace(text[end:]))
}

func steamBuildID(manifestPath string) string {
	raw, err := os.ReadFile(manifestPath)
	if err != nil {
		return ""
	}
	for _, key := range []string{"buildid", "TargetBuildID"} {
		for _, line := range strings.Split(string(raw), "\n") {
			fields := strings.Fields(strings.ReplaceAll(line, "\"", ""))
			if len(fields) >= 2 && fields[0] == key {
				return fields[1]
			}
		}
	}
	return ""
}

func totalMemoryBytes() uint64 {
	ctx, cancel := context.WithTimeout(context.Background(), 800*time.Millisecond)
	defer cancel()
	output, err := runCmd(ctx, "", "sh", "-lc", "awk '/MemTotal/ {print $2*1024}' /proc/meminfo 2>/dev/null")
	if err == nil {
		value, _ := strconv.ParseUint(strings.TrimSpace(string(output)), 10, 64)
		if value > 0 {
			return value
		}
	}
	return 0
}

func lastModified(target string) string {
	info, err := os.Stat(target)
	if err != nil {
		return "-"
	}
	return formatTime(info.ModTime())
}

func formatTimeString(value string) string {
	if strings.TrimSpace(value) == "" || strings.HasPrefix(value, "0001-") {
		return "-"
	}
	t, err := time.Parse(time.RFC3339Nano, value)
	if err != nil {
		return value
	}
	return formatTime(t)
}

var displayLocation = time.Local

func setDisplayTimezone(name string) {
	name = strings.TrimSpace(name)
	if name == "" {
		return
	}
	if location, err := time.LoadLocation(name); err == nil {
		displayLocation = location
	}
}

func serverTimezone() string {
	return displayLocation.String()
}

func formatTime(t time.Time) string {
	return t.In(displayLocation).Format("2006-01-02 15:04:05")
}

func parseDockerLogLine(line string) (string, string) {
	timestamp := formatTime(time.Now())
	parts := strings.SplitN(strings.TrimSpace(line), " ", 2)
	if len(parts) != 2 {
		return timestamp, strings.TrimSpace(line)
	}
	value, err := time.Parse(time.RFC3339Nano, parts[0])
	if err != nil {
		return timestamp, strings.TrimSpace(line)
	}
	return formatTime(value), strings.TrimSpace(parts[1])
}

func formatUptime(startedAt string) string {
	if startedAt == "" || strings.HasPrefix(startedAt, "0001-") {
		return "-"
	}
	start, err := time.Parse(time.RFC3339Nano, startedAt)
	if err != nil {
		return "-"
	}
	seconds := int64(time.Since(start).Seconds())
	if seconds < 0 {
		seconds = 0
	}
	days := seconds / 86400
	hours := (seconds % 86400) / 3600
	minutes := (seconds % 3600) / 60
	if days > 0 {
		return fmt.Sprintf("%d 天 %d 小时 %d 分", days, hours, minutes)
	}
	return fmt.Sprintf("%d 小时 %d 分", hours, minutes)
}

func formatBytes(size int64) string {
	if size <= 0 {
		return "-"
	}
	mb := float64(size) / 1024 / 1024
	if mb < 1024 {
		return fmt.Sprintf("%.0f MB", math.Max(1, mb))
	}
	return fmt.Sprintf("%.1f GB", mb/1024)
}

func formatEnvValue(value string) string {
	if value == "" || strings.ContainsAny(value, " \t#\"'") {
		raw, _ := json.Marshal(value)
		return string(raw)
	}
	return value
}

func formatBool(value bool) string {
	if value {
		return "true"
	}
	return "false"
}

func trimFloat(value float64) string { return strconv.FormatFloat(value, 'f', -1, 64) }

func splitList(value string) []string {
	var result []string
	for _, item := range strings.Split(value, ",") {
		item = strings.TrimSpace(item)
		if item != "" {
			result = append(result, item)
		}
	}
	if len(result) == 0 {
		return []string{"Steam", "Xbox", "PS5", "Mac"}
	}
	return result
}

func readEnvFile(filePath string) map[string]string {
	result := map[string]string{}
	raw, err := os.ReadFile(filePath)
	if err != nil {
		return result
	}
	for _, line := range strings.Split(string(raw), "\n") {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" || strings.HasPrefix(trimmed, "#") || !strings.Contains(trimmed, "=") {
			continue
		}
		parts := strings.SplitN(trimmed, "=", 2)
		key := strings.TrimSpace(parts[0])
		if key == "" {
			continue
		}
		result[key] = parseEnvValue(strings.TrimSpace(parts[1]))
	}
	return result
}

func loadDotEnv(filePath string) {
	raw, err := os.ReadFile(filePath)
	if err != nil {
		return
	}
	for _, line := range strings.Split(string(raw), "\n") {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" || strings.HasPrefix(trimmed, "#") || !strings.Contains(trimmed, "=") {
			continue
		}
		parts := strings.SplitN(trimmed, "=", 2)
		key := strings.TrimSpace(parts[0])
		value := parseEnvValue(strings.TrimSpace(parts[1]))
		if key != "" && os.Getenv(key) == "" {
			_ = os.Setenv(key, value)
		}
	}
}

func parseEnvValue(value string) string {
	if len(value) >= 2 && ((value[0] == '"' && value[len(value)-1] == '"') || (value[0] == '\'' && value[len(value)-1] == '\'')) {
		unquoted, err := strconv.Unquote(value)
		if err == nil {
			return unquoted
		}
		return value[1 : len(value)-1]
	}
	return value
}

func envValueAny(fileEnv map[string]string, fallback string, keys ...string) string {
	for _, key := range keys {
		if value := strings.TrimSpace(fileEnv[key]); value != "" {
			return value
		}
	}
	return getenvAny(fallback, keys...)
}

func getenv(key, fallback string) string {
	value := strings.TrimSpace(os.Getenv(key))
	if value == "" {
		return fallback
	}
	return value
}

func getenvAny(fallback string, keys ...string) string {
	for _, key := range keys {
		if value := strings.TrimSpace(os.Getenv(key)); value != "" {
			return value
		}
	}
	return fallback
}

func envIntAny(fileEnv map[string]string, fallback int, keys ...string) int {
	for _, key := range keys {
		if value, err := strconv.Atoi(strings.TrimSpace(fileEnv[key])); err == nil {
			return value
		}
	}
	return getenvIntAny(fallback, keys...)
}

func getenvInt(key string, fallback int) int {
	value, err := strconv.Atoi(strings.TrimSpace(os.Getenv(key)))
	if err != nil {
		return fallback
	}
	return value
}

func getenvIntAny(fallback int, keys ...string) int {
	for _, key := range keys {
		if value, err := strconv.Atoi(strings.TrimSpace(os.Getenv(key))); err == nil {
			return value
		}
	}
	return fallback
}

func getenvFloatAny(fallback float64, keys ...string) float64 {
	for _, key := range keys {
		if value, err := strconv.ParseFloat(strings.TrimSpace(os.Getenv(key)), 64); err == nil {
			return value
		}
	}
	return fallback
}

func parseBool(value string, fallback bool) bool {
	if value == "" {
		return fallback
	}
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "1", "true", "yes", "on":
		return true
	case "0", "false", "no", "off":
		return false
	default:
		return fallback
	}
}

func randomID() string {
	var buf [16]byte
	if _, err := rand.Read(buf[:]); err != nil {
		return strconv.FormatInt(time.Now().UnixNano(), 36)
	}
	buf[6] = (buf[6] & 0x0f) | 0x40
	buf[8] = (buf[8] & 0x3f) | 0x80
	return fmt.Sprintf("%x-%x-%x-%x-%x", buf[0:4], buf[4:6], buf[6:8], buf[8:10], buf[10:])
}

func safeBackupID(id string) bool {
	if id == "" {
		return false
	}
	for _, r := range id {
		if (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') || r == '.' || r == '_' || r == '-' {
			continue
		}
		return false
	}
	return true
}

func round1(value float64) float64 { return math.Round(value*10) / 10 }
func round2(value float64) float64 { return math.Round(value*100) / 100 }

func minDuration(a, b time.Duration) time.Duration {
	if a < b {
		return a
	}
	return b
}

func isTimeout(err error) bool {
	var netErr net.Error
	return errors.As(err, &netErr) && netErr.Timeout()
}

func orDefault(value, fallback string) string {
	if strings.TrimSpace(value) == "" {
		return fallback
	}
	return value
}
