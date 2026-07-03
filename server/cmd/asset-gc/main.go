// Command asset-gc removes orphan assets: COS objects (and their assets rows)
// no longer referenced by any non-deleted update (§9.2/§9.3). Update soft-deps
// only mark updates as deleted; this job performs the actual storage reclaim.
// It is idempotent and meant to run periodically from cron.
package main

import (
	"context"
	"log"
	"os"
	"time"

	"github.com/hoywu/expo-ota/server/db/models"
	"github.com/hoywu/expo-ota/server/internal/storage"

	"github.com/zeromicro/go-zero/core/stores/sqlx"
)

func main() {
	dbURL := os.Getenv("DB_URL")
	if dbURL == "" {
		log.Fatal("DB_URL is required")
	}

	storage.SetAssetKeyPrefix(os.Getenv("COS_KEY_PREFIX"))
	store, err := storage.NewCosStore(storage.CosConfig{
		SecretID:  os.Getenv("COS_SECRET_ID"),
		SecretKey: os.Getenv("COS_SECRET_KEY"),
		Region:    os.Getenv("COS_REGION"),
		Bucket:    os.Getenv("COS_BUCKET"),
		Domain:    os.Getenv("COS_DOMAIN"),
	})
	if err != nil {
		log.Fatalf("init COS store: %v", err)
	}

	conn := sqlx.NewSqlConn("postgres", dbURL)
	assets := models.NewAssetsModel(conn)

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
	defer cancel()

	deletedUpdates, blockedUpdates, err := purgeSoftDeletedUpdates(ctx, conn)
	if err != nil {
		log.Fatalf("purge soft-deleted updates: %v", err)
	}
	if blockedUpdates > 0 {
		log.Printf("asset-gc: hard-deleted %d soft-deleted update(s), %d still blocked by rollback references",
			deletedUpdates, blockedUpdates)
	} else if deletedUpdates > 0 {
		log.Printf("asset-gc: hard-deleted %d soft-deleted update(s)", deletedUpdates)
	}

	orphans, err := assets.FindOrphans(ctx)
	if err != nil {
		log.Fatalf("find orphan assets: %v", err)
	}

	var deleted, failed int
	for _, asset := range orphans {
		if err := store.Delete(ctx, asset.StorageKey); err != nil {
			log.Printf("delete COS object %s failed: %v", asset.StorageKey, err)
			failed++
			continue
		}
		// Remove the row only after the object is gone, so a failed COS delete
		// leaves the asset reclaimable on the next run.
		if err := assets.Delete(ctx, asset.Id); err != nil {
			log.Printf("delete asset row %s failed: %v", asset.Id, err)
			failed++
			continue
		}
		deleted++
	}

	log.Printf("asset-gc done: %d orphan(s) found, %d deleted, %d failed", len(orphans), deleted, failed)
}

func purgeSoftDeletedUpdates(ctx context.Context, conn sqlx.SqlConn) (deletedTotal, blocked int64, err error) {
	for {
		result, err := conn.ExecCtx(ctx, `
			delete from "public"."updates" u
			where u.deleted_at is not null
			  and not exists (
				select 1 from "public"."updates" child
				where child.rolled_back_from = u.id
			  )`)
		if err != nil {
			return 0, 0, err
		}
		affected, err := result.RowsAffected()
		if err != nil {
			return 0, 0, err
		}
		deletedTotal += affected
		if affected == 0 {
			break
		}
	}

	err = conn.QueryRowCtx(ctx, &blocked, `
		select count(*) from "public"."updates"
		where deleted_at is not null`)
	if err != nil {
		return 0, 0, err
	}
	return deletedTotal, blocked, nil
}
