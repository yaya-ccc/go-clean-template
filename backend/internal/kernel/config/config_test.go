package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// writeCfg 在临时目录写 config.yaml 并把工作目录切过去，返回清理函数。
func writeCfg(t *testing.T, content string) {
	t.Helper()
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "config.yaml"), []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
	wd, _ := os.Getwd()
	if err := os.Chdir(dir); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chdir(wd) })
}

func TestLoadFullConfig(t *testing.T) {
	writeCfg(t, `
addr: 0.0.0.0:9090
mysql_dsn: user:pass@tcp(db:3306)/x
jwt_secret: s3cret
access_ttl: 15m
refresh_ttl: 360h
log:
  format: text
  level: debug
`)
	cfg := Load()
	if cfg.Addr != "0.0.0.0:9090" {
		t.Errorf("Addr = %q", cfg.Addr)
	}
	if cfg.MySQLDSN != "user:pass@tcp(db:3306)/x" {
		t.Errorf("MySQLDSN = %q", cfg.MySQLDSN)
	}
	if cfg.JWTSecret != "s3cret" {
		t.Errorf("JWTSecret = %q", cfg.JWTSecret)
	}
	if cfg.AccessTTL != 15*time.Minute {
		t.Errorf("AccessTTL = %v", cfg.AccessTTL)
	}
	if cfg.RefreshTTL != 360*time.Hour {
		t.Errorf("RefreshTTL = %v", cfg.RefreshTTL)
	}
	if cfg.Log.Format != "text" || cfg.Log.Level != "debug" {
		t.Errorf("Log = %+v", cfg.Log)
	}
}

func TestLoadDefaultsAndRequiredSecret(t *testing.T) {
	writeCfg(t, "jwt_secret: must-exist\n")
	cfg := Load()
	if cfg.Addr != "127.0.0.1:8080" {
		t.Errorf("Addr 缺省应为 127.0.0.1:8080，得到 %q", cfg.Addr)
	}
	if cfg.AccessTTL != 30*time.Minute || cfg.RefreshTTL != 30*24*time.Hour {
		t.Errorf("缺省值不符：%+v", cfg)
	}
	if cfg.Log.Format != "json" || cfg.Log.Level != "info" {
		t.Errorf("Log 缺省应为 json/info，得到 %+v", cfg.Log)
	}
}
func TestLoadMissingFileOrSecretExits(t *testing.T) {
	if os.Getenv("TEST_LOAD_FAIL") == "1" {
		writeCfg(t, "addr: x\n") // 有文件但缺 jwt_secret
		Load()
		return
	}
	out, err := reexecSelf(t).CombinedOutput()
	if err == nil {
		t.Fatalf("缺 jwt_secret 应退出非零，输出 %s", out)
	}
	if want := "jwt_secret"; !strings.Contains(string(out), want) {
		t.Errorf("报错应包含 %q，输出 %s", want, out)
	}
}
