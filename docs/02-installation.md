# Installation

This guide covers how to get Formicary up and running. The recommended method for local evaluation and testing is using Docker Compose.

## Prerequisites

-   **Docker & Docker Compose:** You need a recent version of Docker and Docker Compose installed on your system.
    -   [Install Docker Engine](https://docs.docker.com/engine/install/)
    -   [Install Docker Compose](https://docs.docker.com/compose/install/)
-   **Git:** To clone the repository.
-   **(Optional) Kubernetes:** If you want to use the Kubernetes executor, you need access to a Kubernetes cluster (e.g., [MicroK8s](https://microk8s.io/), [k3s](https://k3s.io/), minikube, or a cloud provider's managed service).
-   **(Optional) Go:** To build from source, you need Go version 1.22 or newer.

## Recommended: Running with Docker Compose

This is the fastest way to start a complete Formicary environment, including the Queen server, an embedded Ant worker, Redis for messaging, and MinIO for artifact storage. This setup uses a local SQLite database for simplicity.

1.  **Clone the Repository**
    ```bash
    git clone https://github.com/bhatti/formicary.git
    cd formicary
    ```

2.  **Create an Environment File**
    Formicary uses a `.env` file to manage secrets and environment-specific settings. An example is provided.

    ```bash
    # Copy the example file
    cp .env.example .env
    ```

3.  **Generate a Session Secret**
    You **must** replace the placeholder secret in the `.env` file. This key is used for securing user sessions.

    Run this command on Linux or macOS to generate a secure key and update the file:
    ```bash
    # This command generates a random key and replaces the placeholder in-place.
    SECRET_KEY=$(openssl rand -base64 32)
    sed -i.bak "s/your_strong_secret_key_here/$SECRET_KEY/" .env
    ```
    On other systems, generate a 32-byte, base64-encoded random string and manually paste it into `.env` for the `COMMON_AUTH_SESSION_KEY` variable.

4.  **(Optional) Configure OAuth**
    To enable login via GitHub or Google, you must create an OAuth application with the respective provider and add the Client ID and Secret to your `.env` file.

5.  **Start the System**
    From the root of the repository, run the `sqlite-docker-compose.yaml` file:
    ```bash
    docker-compose -f sqlite-docker-compose.yaml up --build
    ```
    This will build the Formicary image and start all the necessary services. You'll see logs from all services in your terminal.

6.  **Verify the Installation**
    -   **Formicary Dashboard:** Open your web browser to [http://localhost:7777](http://localhost:7777).
    -   **MinIO Console:** Check the object storage console at [http://localhost:9001](http://localhost:9001) (Use the credentials from your `.env` file, default is `admin`/`password`).

## Running from Source (for Development)

If you plan to contribute to Formicary, you'll want to run it directly from source.

1.  **Start Dependencies:**
    You still need the database, message queue, and object store. You can start just these services from the `docker-compose.yaml` file:
    ```bash
    docker-compose up -d redis minio mysql
    ```
2.  **Configure Queen Server:**
    Ensure your configuration file (`.formicary-queen.yaml` or a custom one) points to the correct addresses for Redis, MinIO, and your database.

3.  **Run the Queen Server:**
    ```bash
    go run ./main.go --config ./.formicary-queen.yaml --id=queen-server-1
    ```

4.  **Run an Ant Worker:**
    In a separate terminal, run the Ant worker:
    ```bash
    go run ./main.go ant --config=./.formicary-ant.yaml --id=local-ant-1 --tags="shell,docker"
    ```
