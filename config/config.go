package config

import (
	"log/slog"
	"os"
	"strconv"

	"github.com/joho/godotenv"
)

type Config struct {
	ApiId           int
	ApiHash         string
	PhoneNumber     string
	BotToken        string
	RedisUri        string
	RedisPass       string
	TelethonSession string
	PyrogramSession string
	GramjsSession   string
}

func NewConfig() *Config {
	err := godotenv.Load()
	if err != nil {
		slog.Error("Env Not Found, Please Create .env file from .env.sample", "error", err)
		os.Exit(1)
	}
	apiId, err := strconv.Atoi(os.Getenv("API_ID"))
	if err != nil {
		slog.Error("Not Acceptable API ID", "error", err)
		os.Exit(1)
	}
	apiHash := os.Getenv("API_HASH")
	redisUri := os.Getenv("REDIS_URI")
	redisPass := os.Getenv("REDIS_PASS")
	phoneNumber := os.Getenv("PHONE_NUMBER")
	botToken := os.Getenv("BOT_TOKEN")
	telethonSession := os.Getenv("TELETHON_SESSION")
	pyrogramSession := os.Getenv("PYROGRAM_SESSION")
	gramjsSession := os.Getenv("GRAMJS_SESSION")
	return &Config{
		ApiId:           apiId,
		ApiHash:         apiHash,
		PhoneNumber:     phoneNumber,
		BotToken:        botToken,
		RedisUri:        redisUri,
		RedisPass:       redisPass,
		TelethonSession: telethonSession,
		PyrogramSession: pyrogramSession,
		GramjsSession:   gramjsSession,
	}
}
