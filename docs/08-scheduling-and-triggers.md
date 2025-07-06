# Guide: Scheduling & Triggers

A job in Formicary can be initiated in several ways: manually via an API call, automatically on a schedule, or in response to an external event like a Git push.

## 1. Manual Submission

The most direct way to run a job is by submitting a **Job Request** to the Formicary API. This is useful for on-demand tasks or for testing your job definitions.

You can use a tool like `curl` to submit a request.

```bash
curl -X POST http://localhost:7777/api/jobs/requests \
     -H "Content-Type: application/json" \
     -d '{"job_type": "your-job-type"}'
```

### Passing Parameters

You can pass parameters to a job run, which can be used by templates in your job definition.

```bash
curl -X POST http://localhost:7777/api/jobs/requests \
     -H "Content-Type: application/json" \
     -d '{
           "job_type": "go-build-ci",
           "params": {
             "GitBranch": "feature/new-login",
             "GitCommitID": "a1b2c3d4"
           }
         }'
```

## 2. Time-Based Scheduling

### Future-Dated Jobs

You can submit a job request that is scheduled to run at a specific time in the future by including the `scheduled_at` field in your API call. The timestamp must be in RFC3339 format.

```bash
curl -X POST http://localhost:7777/api/jobs/requests \
     -H "Content-Type: application/json" \
     -d '{
           "job_type": "daily-report",
           "scheduled_at": "2024-10-26T02:00:00Z"
         }'
```
The job will remain in the `PENDING` state until the specified time.

### Cron Triggers

For recurring schedules, you can add a `cron_trigger` property directly to your job definition YAML. Formicary's scheduler will automatically create a new job request each time the cron expression matches.

The format uses 7 fields, including seconds, providing fine-grained control.

```yaml
# Format: <second> <minute> <hour> <day-of-month> <month> <day-of-week> <year>
job_type: hourly-cleanup
cron_trigger: "0 0 * * * *" # Runs at the beginning of every hour

tasks:
  - task_type: cleanup
    method: SHELL
    script:
      - echo "Running hourly cleanup..."
```

## 3. Event-Driven Triggers

### GitHub Webhooks

You can configure a GitHub repository to trigger a Formicary job on events like `push`. This is the foundation for most CI/CD workflows.

**Setup Steps:**

1.  **Generate an API Token:** In the Formicary dashboard, go to your user settings and create a new API token. Copy this token.

2.  **Configure the Webhook in GitHub:**
    -   In your GitHub repository, go to `Settings > Webhooks > Add webhook`.
    -   **Payload URL:** Set this to `https://<YOUR_FORMICARY_HOST>/api/auth/github/webhook?job_type=<YOUR_JOB_TYPE>`.
    -   **Content type:** Set to `application/json`.
    -   **Secret:** Create a strong secret string and enter it here. You will need this for the next step.
    -   **Events:** Select the events you want to trigger the job, typically just "Pushes".

3.  **Configure the Secret in Formicary:**
    The webhook secret must be stored as a **Job Config** so Formicary can verify the payload signature.
    ```bash
    # Replace <job-id>, <YOUR_SECRET>, and <YOUR_API_TOKEN>
    curl -X POST http://localhost:7777/api/jobs/definitions/<job-id>/configs \
      -H "Authorization: Bearer <YOUR_API_TOKEN>" \
      -H "Content-Type: application/json" \
      -d '{"Name": "GithubWebhookSecret", "Value": "<YOUR_SECRET>", "Secret": true}'
    ```

When the webhook fires, Formicary will automatically start your job and populate it with parameters from the Git event, such as:
-   `GitBranch`
-   `GitCommitID`
-   `GitCommitMessage`
-   `GitRepository`


