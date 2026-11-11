package main

import (
 "context"
 "errors"
 "fmt"
 "log"
 "os"
 "time"

 "github.com/jackc/pgx/v5"
 "github.com/jackc/pgx/v5/pgxpool"
)

func main() {
 ctx := context.Background()
 url := os.Getenv("DATABASE_URL")
 if url == "" { log.Fatal("DATABASE_URL is required") }
 pool, err := pgxpool.New(ctx, url)
 if err != nil { log.Fatal(err) }
 defer pool.Close()
 if err := pool.Ping(ctx); err != nil { log.Fatal(err) }

 llmClient, err := NewLLMClient()
 if err != nil { log.Fatal(err) }
 hostname, err := os.Hostname()
 if err != nil { log.Fatal(err) }
 workerID := fmt.Sprintf("%s-%d", hostname, os.Getpid())
 log.Printf("ATLAS worker started: %s", workerID)
 for {
  if err := processTask(ctx, pool, workerID, llmClient); err != nil {
   log.Printf("Worker error: %v", err)
  }
  time.Sleep(2 * time.Second)
 }
}

func processTask(ctx context.Context, pool *pgxpool.Pool, workerID string, llmClient *LLMClient) error {
 var taskID string
 var prompt string
 var attempts int

 // Expired leases that exhausted their retry budget become terminal failures.
 _, err := pool.Exec(ctx, `
  UPDATE tasks SET status = 'failed', lease_expires_at = NULL
  WHERE status = 'running' AND lease_expires_at < NOW()
    AND attempts >= max_attempts
 `)
 if err != nil { return fmt.Errorf("mark exhausted tasks failed: %w", err) }

 err = pool.QueryRow(ctx, `
  WITH candidate AS (
   SELECT id FROM tasks
   WHERE (status = 'queued' AND attempts < max_attempts)
      OR (status = 'running' AND lease_expires_at < NOW() AND attempts < max_attempts)
   ORDER BY created_at
   FOR UPDATE SKIP LOCKED LIMIT 1
  )
  UPDATE tasks t
  SET status = 'running', attempts = attempts + 1,
      worker_id = $1, lease_expires_at = NOW() + INTERVAL '60 seconds'
  FROM candidate WHERE t.id = candidate.id
  RETURNING t.id, t.prompt, t.attempts
 `, workerID).Scan(&taskID, &prompt, &attempts)
 if errors.Is(err, pgx.ErrNoRows) { return nil }
 if err != nil { return err }
 log.Printf("Processing task=%s attempt=%d", taskID, attempts)

 taskCtx, cancel := context.WithTimeout(ctx, 45*time.Second)
 defer cancel()
 result, err := llmClient.Generate(taskCtx, prompt)
 if err != nil {
  // Leave the task in running state; its lease expires and existing retry logic reclaims it.
  log.Printf("LLM task=%s attempt=%d failed: %v", taskID, attempts, err)
  return nil
 }

 tag, err := pool.Exec(ctx, `
  UPDATE tasks
  SET status = 'completed', result = $2, lease_expires_at = NULL
  WHERE id = $1 AND worker_id = $3 AND status = 'running'
    AND lease_expires_at > NOW()
 `, taskID, result, workerID)
 if err != nil { return err }
 if tag.RowsAffected() == 0 {
  return fmt.Errorf("task %s lease expired or ownership changed", taskID)
 }
 log.Printf("Completed task=%s", taskID)
 return nil
}
