// 遵循project_guide.md
//
// search-backfill — rebuilds the search_documents projection from the
// canonical business tables. Run this:
//
//   - Once after deploying Phase 1 to populate the projection for all
//     existing customers + vendors (handlers only cover rows touched
//     after deploy).
//   - Any time projection logic (producers/*) changes in a way that
//     rewrites existing rows — bumping searchprojection.CurrentProjectorVersion
//     makes this explicit.
//   - To recover from a projection drift incident (rare — projector
//     failures are logged by the handlers).
//
// The tool is idempotent and safe to re-run. It upserts one row per
// entity without interleaving reads/writes that could race with live
// traffic; running against a production DB is supported but expect
// table scans on customers + vendors.
//
// Usage:
//
//	go run ./cmd/search-backfill                 # all entity families
//	go run ./cmd/search-backfill -only customer  # only customers
//	go run ./cmd/search-backfill -only vendor    # only vendors
//	go run ./cmd/search-backfill -dry            # log progress, skip upserts
package main

import (
	"context"
	"flag"
	"log"
	"time"

	"gorm.io/gorm"

	"gobooks/internal/config"
	"gobooks/internal/db"
	"gobooks/internal/logging"
	"gobooks/internal/models"
	"gobooks/internal/searchprojection"
	"gobooks/internal/searchprojection/producers"
)

func main() {
	only := flag.String("only", "all", "restrict to one family: all | customer | vendor")
	dry := flag.Bool("dry", false, "scan + log counts but skip projection upserts")
	companyFilter := flag.Uint("company", 0, "limit to a single company_id (0 = all companies)")
	batchSize := flag.Int("batch", 500, "rows per batch; lower = gentler on the pool")
	flag.Parse()

	logging.Init()
	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("config load failed: %v", err)
	}
	logging.SetLevel(cfg.LogLevel)

	gormDB, err := db.Connect(cfg)
	if err != nil {
		log.Fatalf("db connect failed: %v", err)
	}

	var projector searchprojection.Projector
	if *dry {
		projector = searchprojection.NoopProjector{}
	} else {
		client, err := searchprojection.OpenEntFromGorm(gormDB)
		if err != nil {
			log.Fatalf("ent client init failed: %v", err)
		}
		p, err := searchprojection.NewEntProjector(client, searchprojection.AsciiNormalizer{})
		if err != nil {
			log.Fatalf("projector init failed: %v", err)
		}
		projector = p
	}

	ctx := context.Background()
	start := time.Now()

	if *only == "all" || *only == "customer" {
		if err := backfillCustomers(ctx, gormDB, projector, *companyFilter, *batchSize); err != nil {
			log.Fatalf("customer backfill failed: %v", err)
		}
	}
	if *only == "all" || *only == "vendor" {
		if err := backfillVendors(ctx, gormDB, projector, *companyFilter, *batchSize); err != nil {
			log.Fatalf("vendor backfill failed: %v", err)
		}
	}

	logging.L().Info("search-backfill complete", "elapsed_ms", time.Since(start).Milliseconds(), "dry", *dry)
}

// backfillCustomers scans the customers table in batches and upserts
// each row's projection. Uses a keyset-style ID cursor so the scan is
// order-stable even if rows are written concurrently by handler traffic.
func backfillCustomers(ctx context.Context, db *gorm.DB, p searchprojection.Projector, companyFilter uint, batch int) error {
	logging.L().Info("backfill customers start")
	var cursor uint
	total := 0
	for {
		q := db.Model(&models.Customer{}).Where("id > ?", cursor).Order("id ASC").Limit(batch)
		if companyFilter != 0 {
			q = q.Where("company_id = ?", companyFilter)
		}
		var rows []models.Customer
		if err := q.Find(&rows).Error; err != nil {
			return err
		}
		if len(rows) == 0 {
			break
		}
		for _, c := range rows {
			doc := producers.CustomerDocument(c)
			if err := p.Upsert(ctx, doc); err != nil {
				logging.L().Warn("customer upsert failed (continuing)", "id", c.ID, "company_id", c.CompanyID, "err", err)
				continue
			}
			cursor = c.ID
			total++
		}
		logging.L().Info("backfill customers progress", "scanned_total", total)
	}
	logging.L().Info("backfill customers done", "total", total)
	return nil
}

func backfillVendors(ctx context.Context, db *gorm.DB, p searchprojection.Projector, companyFilter uint, batch int) error {
	logging.L().Info("backfill vendors start")
	var cursor uint
	total := 0
	for {
		q := db.Model(&models.Vendor{}).Where("id > ?", cursor).Order("id ASC").Limit(batch)
		if companyFilter != 0 {
			q = q.Where("company_id = ?", companyFilter)
		}
		var rows []models.Vendor
		if err := q.Find(&rows).Error; err != nil {
			return err
		}
		if len(rows) == 0 {
			break
		}
		for _, v := range rows {
			doc := producers.VendorDocument(v)
			if err := p.Upsert(ctx, doc); err != nil {
				logging.L().Warn("vendor upsert failed (continuing)", "id", v.ID, "company_id", v.CompanyID, "err", err)
				continue
			}
			cursor = v.ID
			total++
		}
		logging.L().Info("backfill vendors progress", "scanned_total", total)
	}
	logging.L().Info("backfill vendors done", "total", total)
	return nil
}
