package main

import (
  "context"
  "fmt"
  "github.com/joho/godotenv"
  "github.com/orvo-sh/orvo/internal/config"
  "github.com/orvo-sh/orvo/internal/infra/postgres"
)

func main(){
  _ = godotenv.Load(".env")
  cfg, _ := config.Load()
  pg, _ := postgres.New(context.Background(), postgres.Config{URL: cfg.Postgres.URL})
  defer pg.Close()
  jobID := "sbj_01kjd4qf2c2rppsc0nzzak62fq"
  var state, errText string
  var prURL *string
  _ = pg.Pool().QueryRow(context.Background(), `SELECT state, error, pull_request_url FROM sandbox_jobs WHERE id=$1`, jobID).Scan(&state, &errText, &prURL)
  fmt.Printf("job=%s state=%s error=%q pr=%v\n", jobID, state, errText, prURL)
  rows, _ := pg.Pool().Query(context.Background(), `SELECT seq, stream, message FROM sandbox_job_logs WHERE sandbox_job_id=$1 ORDER BY seq DESC LIMIT 40`, jobID)
  defer rows.Close()
  for rows.Next(){
    var seq int64
    var stream, msg string
    _ = rows.Scan(&seq, &stream, &msg)
    fmt.Printf("%d [%s] %s\n", seq, stream, msg)
  }
}
