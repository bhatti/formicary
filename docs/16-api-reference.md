# API Reference

The Formicary API provides a RESTful interface for managing all aspects of the system, from job definitions to user management.

## Authentication

All API endpoints require authentication via a JSON Web Token (JWT). The token must be included in the `Authorization` header with the `Bearer` scheme.

```
Authorization: Bearer <YOUR_API_TOKEN>
```

You can generate API tokens from the user profile page in the Formicary dashboard.

---

## Job Definitions

Resource for managing job blueprints (`job_type`).

### `GET /api/jobs/definitions`
Queries and lists all job definitions visible to the user.

-   **Permissions:** `JobDefinition:Query`
-   **Query Parameters:**
    -   `job_type` (string): Filter by job type.
    -   `platform` (string): Filter by platform.
    -   `tags` (string): Filter by tags.
    -   `page`, `page_size` (int): For pagination.
-   **Success Response (200 OK):** A paginated list of `JobDefinition` objects.

### `POST /api/jobs/definitions`
Creates a new job definition or updates an existing one if the `job_type` already exists. The body can be `application/json` or `application/yaml`.

-   **Permissions:** `JobDefinition:Create`
-   **Request Body:** A full `JobDefinition` object.
-   **Success Response (201 Created or 200 OK):** The saved `JobDefinition` object.

### `GET /api/jobs/definitions/{id}`
Retrieves a single job definition by its unique ID or its `job_type`.

-   **Permissions:** `JobDefinition:View`
-   **Path Parameters:**
    -   `id` (string): The UUID or `job_type` of the definition.
-   **Success Response (200 OK):** A single `JobDefinition` object.

### `DELETE /api/jobs/definitions/{id}`
Deletes a job definition by its ID.

-   **Permissions:** `JobDefinition:Delete`
-   **Path Parameters:**
    -   `id` (string): The UUID of the definition.
-   **Success Response (200 OK):** Empty body.

### `POST /api/jobs/definitions/{id}/disable`
Disables a job definition, preventing new job requests from being scheduled.

-   **Permissions:** `JobDefinition:Disable`
-   **Path Parameters:**
    -   `id` (string): The UUID of the definition.
-   **Success Response (200 OK):** Empty body.

### `POST /api/jobs/definitions/{id}/enable`
Enables a disabled job definition.

-   **Permissions:** `JobDefinition:Enable`
-   **Path Parameters:**
    -   `id` (string): The UUID of the definition.
-   **Success Response (200 OK):** Empty body.

### `GET /api/jobs/definitions/type/{type}/versions`
Lists all stored versions of a job definition identified by its `job_type`, ordered by version number descending. Useful for auditing history or pinning a restart to a specific version.

-   **Permissions:** `JobDefinition:View`
-   **Path Parameters:**
    -   `type` (string): The `job_type` of the definition.
-   **Query Parameters:**
    -   `page`, `page_size` (int): Pagination.
-   **Success Response (200 OK):** Paginated list of version summaries.
    ```json
    {
      "records": [
        {
          "id": "01JXY...",
          "version": 3,
          "sem_version": "1.2.0",
          "active": true,
          "created_at": "2026-06-04T12:00:00Z",
          "updated_at": "2026-06-04T12:00:00Z"
        }
      ],
      "total_records": 3,
      "page": 0,
      "page_size": 10
    }
    ```

---

## Job Requests

Resource for submitting and managing individual runs of jobs.

### `POST /api/jobs/requests`
Submits a new job for execution.

-   **Permissions:** `JobRequest:Submit`
-   **Request Body:**
    ```json
    {
      "job_type": "string",
      "job_version": "string (optional)",
      "scheduled_at": "datetime (optional, RFC3339)",
      "params": {
        "key": "value"
      }
    }
    ```
-   **Success Response (201 Created):** The created `JobRequest` object.

### `GET /api/jobs/requests`
Queries and lists job requests.

-   **Permissions:** `JobRequest:Query`
-   **Query Parameters:**
    -   `job_type`, `job_state`, `platform` (string): Filters.
    -   `page`, `page_size` (int): Pagination.
-   **Success Response (200 OK):** A paginated list of `JobRequest` objects.

### `GET /api/jobs/requests/{id}`
Retrieves the status and execution details of a specific job request.

-   **Permissions:** `JobRequest:View`
-   **Path Parameters:**
    -   `id` (string): The ID of the job request.
-   **Success Response (200 OK):** A single `JobRequest` object, including its `execution` details.

### `POST /api/jobs/requests/{id}/cancel`
Cancels a pending or executing job request.

-   **Permissions:** `JobRequest:Cancel`
-   **Path Parameters:**
    -   `id` (string): The ID of the job request to cancel.
-   **Success Response (200 OK):** Empty body.

### `POST /api/jobs/requests/{id}/restart`
Restarts a failed or completed job request. Supports pinning the restart to a specific version of the job definition.

-   **Permissions:** `JobRequest:Restart`
-   **Path Parameters:**
    -   `id` (string): The ID of the job request to restart.
-   **Query Parameters:**
    -   `hard` (boolean, default `false`): If `true`, forces all tasks to re-run from scratch rather than resuming from the last failed task. A hard restart also upgrades to the latest job definition unless `version` is specified.
    -   `version` (string, optional): Pin the restart to a specific job definition version. Accepted values:
        -   `latest` — always use the most recently deployed definition (default for hard restarts).
        -   `<sem_version>` — e.g. `"1.2.0"`, matches the definition's `sem_version` field.
        -   `<definition_id>` — the UUID of a specific definition row (from the versions API).
        -   `""` (empty, default for soft restart) — keep the same definition the original request used.
-   **Success Response (200 OK):** Empty body.

**Examples:**

```bash
# Soft restart — resume from the last failed task using the original definition
curl -X POST http://localhost:7777/api/jobs/requests/01JXY.../restart \
  -H "Authorization: Bearer <token>"

# Hard restart — re-run all tasks using the latest definition
curl -X POST "http://localhost:7777/api/jobs/requests/01JXY.../restart?hard=true" \
  -H "Authorization: Bearer <token>"

# Soft restart pinned to semantic version 1.2.0
curl -X POST "http://localhost:7777/api/jobs/requests/01JXY.../restart?version=1.2.0" \
  -H "Authorization: Bearer <token>"
```

### `POST /api/jobs/requests/{id}/review`
Approves a task that is awaiting manual intervention, allowing the job to continue.

-   **Permissions:** `JobRequest:Approve`
-   **Path Parameters:**
    -   `id` (string): The ID of the job request containing the paused task.
-   **Request Body:**
    ```json
    {
      "execution_id": "string",
      "task_type": "string",
      "comments": "string (optional)",
      "status": "string (APPROVED|REJECTED)"
    }
    ```
-   **Success Response (200 OK):** Empty body.

---

## Artifacts

Resource for managing task outputs.

### `GET /api/artifacts`
Queries artifacts based on metadata.

-   **Permissions:** `Artifact:Query`
-   **Query Parameters:** `job_request_id`, `task_type`, `name`, `kind`, etc.
-   **Success Response (200 OK):** Paginated list of `Artifact` metadata objects.

### `GET /api/artifacts/{id}/download`
Downloads the content of a specific artifact.

-   **Permissions:** `Artifact:View`
-   **Path Parameters:**
    -   `id` (string): The ID or SHA256 of the artifact.
-   **Success Response (200 OK):** The raw file data with an appropriate `Content-Disposition` header.

### `DELETE /api/artifacts/{id}`
Deletes an artifact from the object store and its metadata from the database.

-   **Permissions:** `Artifact:Delete`
-   **Path Parameters:**
    -   `id` (string): The ID of the artifact to delete.
-   **Success Response (200 OK):** Empty body.

---

## System Administration

Endpoints for managing the Formicary system. **Admin permissions are required for all of these endpoints.**

### `GET /api/ants`
Lists all currently registered Ant workers and their status.

-   **Permissions:** `AntExecutor:Query`
-   **Success Response (200 OK):** A list of `AntRegistration` objects.

### `GET /api/executors`
Lists all active task executors (e.g., Docker containers, Kubernetes pods) across all Ant workers.

-   **Permissions:** `Container:Query`
-   **Success Response (200 OK):** A paginated list of `ContainerLifecycleEvent` objects.

### `DELETE /api/executors/{id}`
Forcibly terminates an active executor (e.g., a running container).

-   **Permissions:** `Container:Delete`
-   **Path Parameters:**
    -   `id` (string): The ID of the container/executor to terminate.
-   **Query Parameters:**
    -   `antID` (string): The ID of the Ant worker hosting the executor.
    -   `method` (string): The method of the executor (e.g., `DOCKER`).
-   **Success Response (200 OK):** Empty body.

### `GET /api/health`
Returns the health status of the Queen server and its dependencies.

-   **Permissions:** `Health:Query`
-   **Success Response (200 OK or 503 Service Unavailable):** A `HealthQueryResponse` object.

### `GET /api/metrics`
Exposes system metrics in Prometheus format.

-   **Permissions:** `Health:Metrics`
-   **Success Response (200 OK):** Prometheus metrics text.

---

## User & Organization Management

Endpoints for managing users, organizations, and API access.

### `GET /api/users`
Queries users. Admins can query all users; non-admins can only see users within their organization.

-   **Permissions:** `User:Query`
-   **Success Response (200 OK):** Paginated list of `User` objects.

### `GET /api/users/{id}`
Retrieves a specific user's profile.

-   **Permissions:** `User:View`
-   **Success Response (200 OK):** A `User` object.

### `PUT /api/users/{id}`
Updates a user's profile.

-   **Permissions:** `User:Update`
-   **Request Body:** A `User` object.
-   **Success Response (200 OK):** The updated `User` object.

### `GET /api/users/{userId}/tokens`
Lists all API tokens for a given user.

-   **Permissions:** `User:View` (must be the user themselves or an admin)
-   **Success Response (200 OK):** A list of `UserToken` metadata objects (the token secret is not returned).

### `POST /api/users/{userId}/tokens`
Creates a new API token for a user.

-   **Permissions:** `User:Update` (must be the user themselves or an admin)
-   **Request Body (form-data):** `token=Your-Token-Name`
-   **Success Response (200 OK):** A `UserToken` object containing the **one-time** view of the generated API token.
