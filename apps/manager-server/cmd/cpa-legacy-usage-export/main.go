package main

import (
	"context"
	"database/sql"
	"flag"
	"fmt"
	"io"
	"log"
	"os"

	"github.com/seakee/cpa-manager-plus/apps/manager-server/internal/legacyusage"
	_ "modernc.org/sqlite"
)

func main() {
	input := flag.String("input", "", "path to a legacy CPA usage.db snapshot")
	output := flag.String("output", "", "path to write Manager Plus JSONL; stdout when empty")
	limit := flag.Int("limit", 0, "maximum rows to export; 0 exports all rows")
	offset := flag.Int("offset", 0, "rows to skip before exporting")
	flag.Parse()

	if *input == "" {
		log.Fatal("--input is required")
	}
	db, err := sql.Open("sqlite", *input)
	if err != nil {
		log.Fatalf("open legacy usage db: %v", err)
	}
	defer func() {
		if err := db.Close(); err != nil {
			log.Printf("close legacy usage db: %v", err)
		}
	}()

	writer, closeOutput, err := openOutput(*output)
	if err != nil {
		log.Fatalf("open output: %v", err)
	}
	defer closeOutput()

	summary, err := legacyusage.ExportRequestLogs(context.Background(), db, writer, legacyusage.ExportOptions{
		Limit:  *limit,
		Offset: *offset,
	})
	if err != nil {
		log.Fatalf("export legacy request_logs: %v", err)
	}
	_, _ = fmt.Fprintf(os.Stderr, "exported request_logs rows=%d written=%d\n", summary.Rows, summary.Written)
}

func openOutput(path string) (io.Writer, func(), error) {
	if path == "" {
		return os.Stdout, func() {}, nil
	}
	file, err := os.Create(path)
	if err != nil {
		return nil, func() {}, err
	}
	return file, func() {
		if err := file.Close(); err != nil {
			log.Printf("close output: %v", err)
		}
	}, nil
}
