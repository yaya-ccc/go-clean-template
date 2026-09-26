// Package config 负责从工作目录 config.yaml 加载服务运行配置（缺省值兜底）。
// 配置决策：全部配置入文件、密钥也在文件，环境变量不再是配置来源。
package config

import (
	"log"
	"os"
	"time"

	"github.com/goccy/go-yaml"
)

// LogConfig 是日志输出配置，映射 config.yaml 的 log 段。
type LogConfig struct {
	Format string `yaml:"format"` // text | json，缺省 json
	Level  string `yaml:"level"`  // debug | info | warn | error，缺省 info
}

// Config 汇总服务启动所需的全部外部配置。
type Config struct {
	Addr     string `yaml:"addr"`      // HTTP 监听地址
	MySQLDSN string `yaml:"mysql_dsn"` // MySQL 连接串

	JWTSecret  string        `yaml:"jwt_secret"`  // JWT HMAC 签名密钥；必填，无缺省值
	AccessTTL  time.Duration `yaml:"access_ttl"`  // access token 有效期
	RefreshTTL time.Duration `yaml:"refresh_ttl"` // refresh token 有效期

	Log LogConfig `yaml:"log"`
}

// configPath 是固定配置文件路径；容器经挂载注入，本地多环境用不同工作目录区分。
const configPath = "config.yaml"

// Load 读取工作目录 config.yaml；缺失时使用缺省值。
// jwt_secret 缺失直接退出：给签名密钥设缺省值等于公开后门。
func Load() Config {
	var cfg Config
	if b, err := os.ReadFile(configPath); err == nil {
		if err := yaml.Unmarshal(b, &cfg); err != nil {
			log.Fatalf("解析 %s 失败: %v", configPath, err)
		}
	}

	if cfg.Addr == "" {
		cfg.Addr = "127.0.0.1:8080"
	}
	if cfg.MySQLDSN == "" {
		cfg.MySQLDSN = "root:@tcp(127.0.0.1:3306)/go_clean_template?charset=utf8mb4&parseTime=true&loc=Local"
	}
	if cfg.AccessTTL == 0 {
		cfg.AccessTTL = 30 * time.Minute
	}
	if cfg.RefreshTTL == 0 {
		cfg.RefreshTTL = 30 * 24 * time.Hour
	}
	if cfg.Log.Format == "" {
		cfg.Log.Format = "json"
	}
	if cfg.Log.Level == "" {
		cfg.Log.Level = "info"
	}

	if cfg.JWTSecret == "" {
		log.Fatalf("%s 缺少 jwt_secret", configPath)
	}
	return cfg
}
