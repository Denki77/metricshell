# Long-running worker with Prometheus

Build the standalone-copy context with release assets, run `docker compose up --build -d`, then query
`http://127.0.0.1:19090/api/v1/query?query=example_jobs_total`. Values change while the worker remains alive. Run
`docker compose down --volumes --remove-orphans` to clean up.
