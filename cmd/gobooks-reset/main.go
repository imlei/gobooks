// 遵循产品需求 v1.0
//
// gobooks-reset wipes all GoBooks data in the configured PostgreSQL database
// and resets ID sequences. After running, start gobooks and complete /setup again.
package main

import (
	"flag"
	"log"
	"os"

	"gobooks/internal/config"
	"gobooks/internal/db"
)

func main() {
	yes := flag.Bool("yes", false, "required: confirm you want to delete ALL company and accounting data")
	flag.Parse()
	if !*yes {
		log.Println("Refusing to run without -yes (this deletes all GoBooks data in the configured database).")
		os.Exit(1)
	}

	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("config load failed: %v", err)
	}

	gormDB, err := db.Connect(cfg)
	if err != nil {
		log.Fatalf("db connect failed: %v", err)
	}

	if err := db.ResetAllApplicationData(gormDB); err != nil {
		log.Fatalf("reset failed: %v", err)
	}

	log.Println("All application data cleared. Restart gobooks and open /setup to configure the company again.")
}
