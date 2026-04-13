package main

import (
	"flag"
	"fmt"
	"os"
	"strconv"

	"github.com/joho/godotenv"
	"github.com/pressly/goose/v3"
	"github.com/ranit-biswas/taskflow/internal/config"
	"github.com/ranit-biswas/taskflow/internal/database"
	"github.com/ranit-biswas/taskflow/migrations"
	"go.uber.org/zap"
)

func main() {
	_ = godotenv.Load()

	logger, _ := zap.NewProduction()
	defer logger.Sync()

	flag.Usage = func() {
		fmt.Fprintf(os.Stderr, `Usage: taskflow-migrate <command> [args]

Commands:
  up                 Apply all pending migrations
  up-by-one          Apply the next pending migration only
  up-to <version>    Migrate up to (and including) a specific version
  down               Roll back one migration
  down-to <version>  Roll back down to (but not including) a specific version
  redo               Roll back the last migration and re-apply it
  status             Print migration status
  version            Print the current migration version
  reset              Roll back all migrations
`)
	}
	flag.Parse()

	command := flag.Arg(0)
	if command == "" {
		flag.Usage()
		os.Exit(1)
	}

	cfg := config.Load()

	db, err := database.Connect(cfg.DB)
	if err != nil {
		logger.Fatal("failed to connect to database", zap.Error(err))
	}
	defer db.Close()

	goose.SetBaseFS(migrations.FS)
	if err := goose.SetDialect("postgres"); err != nil {
		logger.Fatal("failed to set dialect", zap.Error(err))
	}

	switch command {
	case "up":
		logger.Info("running migrations up")
		if err := goose.Up(db, "."); err != nil {
			logger.Fatal("migration up failed", zap.Error(err))
		}
		logger.Info("migrations completed successfully")

	case "up-by-one":
		logger.Info("applying next migration")
		if err := goose.UpByOne(db, "."); err != nil {
			logger.Fatal("migration up-by-one failed", zap.Error(err))
		}
		logger.Info("migration applied successfully")

	case "up-to":
		version := parseVersion(flag.Arg(1), "up-to")
		logger.Info("migrating up to version", zap.Int64("version", version))
		if err := goose.UpTo(db, ".", version); err != nil {
			logger.Fatal("migration up-to failed", zap.Error(err))
		}
		logger.Info("migrations completed successfully")

	case "down":
		logger.Info("rolling back one migration")
		if err := goose.Down(db, "."); err != nil {
			logger.Fatal("migration down failed", zap.Error(err))
		}
		logger.Info("rollback completed successfully")

	case "down-to":
		version := parseVersion(flag.Arg(1), "down-to")
		logger.Info("rolling back to version", zap.Int64("version", version))
		if err := goose.DownTo(db, ".", version); err != nil {
			logger.Fatal("migration down-to failed", zap.Error(err))
		}
		logger.Info("rollback completed successfully")

	case "redo":
		logger.Info("redoing last migration")
		if err := goose.Redo(db, "."); err != nil {
			logger.Fatal("migration redo failed", zap.Error(err))
		}
		logger.Info("redo completed successfully")

	case "status":
		if err := goose.Status(db, "."); err != nil {
			logger.Fatal("migration status failed", zap.Error(err))
		}

	case "version":
		if err := goose.Version(db, "."); err != nil {
			logger.Fatal("migration version failed", zap.Error(err))
		}

	case "reset":
		logger.Info("resetting all migrations")
		if err := goose.Reset(db, "."); err != nil {
			logger.Fatal("migration reset failed", zap.Error(err))
		}
		logger.Info("reset completed successfully")

	default:
		fmt.Fprintf(os.Stderr, "unknown command: %s\n", command)
		flag.Usage()
		os.Exit(1)
	}
}

func parseVersion(arg, command string) int64 {
	if arg == "" {
		fmt.Fprintf(os.Stderr, "usage: taskflow-migrate %s <version>\n", command)
		os.Exit(1)
	}
	v, err := strconv.ParseInt(arg, 10, 64)
	if err != nil {
		fmt.Fprintf(os.Stderr, "invalid version %q: %v\n", arg, err)
		os.Exit(1)
	}
	return v
}
