// milvus-migrate copies legacy Milvus collections into new collections that support Chinese and English BM25.
package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"os"
	"os/signal"
	"strings"
	"time"

	milvusRepo "github.com/jrmatherly/ai-kb/internal/application/repository/retriever/milvus"
	client "github.com/milvus-io/milvus/client/v3/milvusclient"
)

func main() {
	sourceDefault := strings.TrimSpace(os.Getenv("MILVUS_COLLECTION"))
	if sourceDefault == "" {
		sourceDefault = "aikb_embeddings"
	}
	targetDefault := sourceDefault + "_multilingual"

	source := flag.String("source", sourceDefault, "Legacy collection prefix, for example aikb_embeddings")
	target := flag.String("target", targetDefault, "New collection prefix, for example aikb_embeddings_multilingual")
	address := flag.String("address", envOr("MILVUS_ADDRESS", "localhost:19530"), "Milvus address")
	username := flag.String("username", os.Getenv("MILVUS_USERNAME"), "Milvus username")
	password := flag.String("password", os.Getenv("MILVUS_PASSWORD"), "Milvus password")
	database := flag.String("database", os.Getenv("MILVUS_DB_NAME"), "Milvus database name")
	metric := flag.String(
		"metric-type",
		strings.TrimSpace(os.Getenv("MILVUS_METRIC_TYPE")),
		"Dense vector metric: IP, COSINE or L2; omit to keep the source collection's metric",
	)
	batchSize := flag.Int("batch-size", 64, "Rows migrated per batch; lower it for long texts")
	flag.Parse()

	metricType, err := milvusRepo.ParseMetricType(*metric)
	if err != nil {
		log.Fatal(err)
	}
	if strings.TrimSpace(*source) == strings.TrimSpace(*target) {
		log.Fatal("--source and --target must differ; migration copies into new collections, never over old ones")
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()

	connectCtx, cancelConnect := context.WithTimeout(ctx, 30*time.Second)
	milvusClient, err := client.New(connectCtx, &client.ClientConfig{
		Address:  *address,
		Username: *username,
		Password: *password,
		DBName:   *database,
	})
	cancelConnect()
	if err != nil {
		log.Fatalf("failed to connect to Milvus: %v", err)
	}
	defer func() {
		closeCtx, cancelClose := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancelClose()
		if err := milvusClient.Close(closeCtx); err != nil {
			log.Printf("failed to close the Milvus connection: %v", err)
		}
	}()

	summary, err := milvusRepo.MigrateLegacyCollections(ctx, milvusClient, milvusRepo.MultilingualMigrationOptions{
		SourceCollectionBaseName: *source,
		TargetCollectionBaseName: *target,
		MetricType:               metricType,
		BatchSize:                *batchSize,
	})
	if err != nil {
		log.Fatalf(
			"migration failed (checked %d collections, copied %d collections and %d rows): %v",
			summary.ExaminedCollections,
			summary.MigratedCollections,
			summary.MigratedRows,
			err,
		)
	}
	fmt.Printf(
		"Migration complete: checked %d collections, copied %d collections and %d rows; old ones kept.\n",
		summary.ExaminedCollections,
		summary.MigratedCollections,
		summary.MigratedRows,
	)
}

func envOr(name, fallback string) string {
	if value := strings.TrimSpace(os.Getenv(name)); value != "" {
		return value
	}
	return fallback
}
