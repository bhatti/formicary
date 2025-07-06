# Guide: Security

Security is a first-class citizen in Formicary. This guide covers the key security concepts and provides best practices for securing your deployment.

## Authentication

Formicary secures its API and dashboard endpoints using an authentication layer that can be enabled or disabled via configuration. When enabled, it uses an OAuth2 and JWT-based flow.

### OAuth2 Flow
1.  A user attempts to access the dashboard and is redirected to a configured OAuth2 provider (e.g., Google or GitHub).
2.  After successful login with the provider, the user is redirected back to Formicary with an authorization code.
3.  Formicary exchanges this code for user information and creates or retrieves the corresponding user record from its database.
4.  A JSON Web Token (JWT) is generated for the user's session.

### JWT Session Management
-   **API Access:** For programmatic access, API tokens (which are long-lived JWTs) should be generated from the user's profile page. This token must be sent in the `Authorization` header with the `Bearer` scheme.
-   **Dashboard Access:** For web UI access, the JWT is stored in a secure, `HttpOnly` browser cookie, managing the user's session automatically.

## Authorization (ACL / RBAC)

Formicary uses a Role-Based Access Control (RBAC) system to manage what users are allowed to do. This is built on a system of **Resources**, **Actions**, and **Roles**.

### Resources
A resource is a type of object in Formicary. Examples include:
-   `JobDefinition`
-   `JobRequest`
-   `User`
-   `Organization`
-   `SystemConfig`
-   `Artifact`

### Actions
An action is an operation that can be performed on a resource. Examples include:
-   `Create` (2)
-   `View` / `Read` (4)
-   `Query` (8)
-   `Update` (16)
-   `Delete` (32)
-   `Submit` (64)
-   `Cancel` (128)

### Permissions
A permission is a combination of a resource and one or more actions. By default, users are granted a set of permissions that allow them to manage their own jobs and artifacts.

### Roles
Roles grant broad permissions that can bypass standard ACL checks. Formicary has two built-in system roles:
-   **`Admin`:** Grants full read/write access to all resources across the entire system.
-   **`ReadAdmin`:** Grants read-only access to all resources across the system.

An administrator can assign these roles to users to grant them system-wide privileges.

## Secrets Management

Properly managing secrets like API keys, tokens, and passwords is vital for security.

### Encrypted Configuration
When creating a **Job Config** or **Organization Config**, you can set the `Secret` flag to `true`.
```json
{
  "Name": "GithubToken",
  "Value": "ghp_YourSecretTokenHere",
  "Secret": true
}
```
-   **Encryption at Rest:** When this flag is set, Formicary uses the `db.encryption_key` from your server configuration to encrypt the value before storing it in the database.
-   **Log Redaction:** The value of any variable marked as a secret is automatically redacted from all job logs, appearing as `[****]`. This prevents accidental exposure.

**Important:** The `db.encryption_key` and `common.encryption_key` are critical. **You must back up your `formicary-queen.yaml` file.** Losing this key will result in being unable to decrypt your stored secrets.

### Webhook Security
When configuring webhooks (e.g., from GitHub), always set a **Secret**. Formicary uses this secret to verify the HMAC signature of incoming webhook payloads, ensuring they are legitimate and not from a malicious actor. This secret should be stored as an encrypted `JobDefinitionConfig` named `GithubWebhookSecret`.

