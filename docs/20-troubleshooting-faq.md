# Troubleshooting & FAQ

This guide provides solutions to common problems and answers frequently asked questions about Formicary.

## Frequently Asked Questions (FAQ)

### **Q: My job is stuck in the `PENDING` state. Why isn't it running?**

This is the most common issue and usually has one of these causes:

1.  **No Matching Ant Worker:** The job's tasks require a specific `method` (like `DOCKER`) or `tags` that no currently registered Ant worker provides.
    -   **Solution:** Check the **Ants** page on the dashboard. Verify that at least one Ant is online and that its listed "Methods" and "Tags" match what your job's tasks require.

2.  **Concurrency Limit Reached:** The job's `max_concurrency` limit has been reached, or the organization/user's concurrency limit is full.
    -   **Solution:** Wait for other jobs to complete, or increase the `max_concurrency` in the job definition if appropriate.

3.  **Required Resources Are In Use:** If the job requires a `resource` (acting as a mutex or semaphore), it will remain pending until that resource is released by another job.
    -   **Solution:** Check the execution status of other jobs that might be holding the resource.

### **Q: My Kubernetes or Docker task fails immediately. How do I debug it?**

This typically happens when the container cannot be created by the executor.

1.  **Image Pull Errors:** This is common with private container registries.
    -   **Solution:** Ensure you have configured `image_pull_secrets` in your Ant's Kubernetes configuration or are logged into the Docker registry on the Ant worker machine.

2.  **Permission Errors:** The Ant worker might not have permission to create pods or containers.
    -   **Solution:** Check the permissions of the `service_account` used by the Ant's pod in Kubernetes, or ensure the user running the Ant process is in the `docker` group for the Docker executor.

3.  **Invalid Configuration:** Incorrect volume mounts, device mappings, or other container settings.
    -   **Solution:** For Kubernetes, use `kubectl describe pod <pod-name>` to see events and detailed error messages. The pod name is visible in the task execution logs. For Docker, check the Ant worker's logs for errors from the Docker client.

### **Q: I'm getting a "403 Forbidden" or "Unauthorized" error when using the API.**

This is an authorization (ACL) issue. The user or API token you are using does not have the required permission for that action.

-   **Solution:** An administrator needs to review the user's **Roles & Permissions**. For example, to create a new job definition, the user needs the `JobDefinition:Create` permission. Refer to the [Security Guide](./19-security.md) for a detailed explanation of the ACL system.

### **Q: How do I debug a hanging or slow job?**

If a Formicary process (Queen or Ant) seems unresponsive, you can get a full stack trace of all running goroutines without killing the process.

1.  Find the Process ID (PID) of the `formicary` process: `pgrep formicary`
2.  Send the `SIGHUP` signal to the process: `kill -HUP <PID>`

The full stack trace will be printed to the standard output of the process, which can be invaluable for diagnosing deadlocks or performance issues.

### **Q: My job failed with the error "ant resources not available".**

This error comes directly from the Job Scheduler. It means that while there might be Ants that *could* run the job, none are currently available (i.e., they are all at their `max_capacity`). The scheduler will automatically retry scheduling the job after a short, exponentially increasing delay.

### **Q: My job keeps retrying but the delays between attempts seem the same. How do I get exponential backoff?**

By default, Formicary uses a small random delay between retries (5–14 seconds for jobs, 1–3 seconds for tasks) to prevent thundering-herd problems when many jobs fail at the same time. To get deterministic exponential backoff, add a `retry_backoff_policy` to your job or task definition:

```yaml
retry: 5
retry_backoff_policy:
  min: 2s      # first retry waits 2s
  max: 5m      # never wait longer than 5 minutes
  factor: 2.0  # 2s → 4s → 8s → 16s → 32s
  jitter: true # ±50% random noise to spread concurrent retries
```

See [Advanced Workflows — Exponential Backoff](./14-advanced-workflows.md#exponential-backoff-for-retries) for the full reference.

### **Q: How do I trace a specific job execution end-to-end?**

Enable OTel tracing in your configuration (see [Observability Guide](./observability.md)) and point the `endpoint` at a Jaeger or Grafana Tempo instance. Every job request generates a trace tree covering the full path: HTTP API → scheduler → job supervisor → task dispatch → ant execution. The `job.request_id` span attribute is present on every span, so you can search by it directly in Jaeger.

```yaml
common:
  tracing:
    enabled: true
    endpoint: http://jaeger:4318
    sample_ratio: 1.0
```
