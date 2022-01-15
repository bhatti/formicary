package types

// TaskMethod defines enum for method of task when communicating with remote ant follower.
// The ant followers registers with the methods that they support and the task is then routed
// based on method, tags and concurrency of the ant follower.
type TaskMethod string

const (
	// ExpireArtifacts method expires old artifacts
	ExpireArtifacts TaskMethod = "EXPIRE_ARTIFACTS"
	// ForkJob method starts new job
	ForkJob TaskMethod = "FORK_JOB"
	// AwaitForkedJob method waits for the job completion that was forked
	AwaitForkedJob TaskMethod = "AWAIT_FORKED_JOB"
	// Messaging method for messaging
	Messaging TaskMethod = "MESSAGING"
	// HTTPGet method sends HTTP GET request
	HTTPGet TaskMethod = "HTTP_GET"
	// HTTPPostForm method sends HTTP POST request as Form
	HTTPPostForm TaskMethod = "HTTP_POST_FORM"
	// HTTPPostJSON method sends HTTP POST request as JSON Body
	HTTPPostJSON TaskMethod = "HTTP_POST_JSON"
	// HTTPPutFORM method sends HTTP PUT request
	HTTPPutFORM TaskMethod = "HTTP_PUT_FORM"
	// HTTPPutJSON method sends HTTP PUT request
	HTTPPutJSON TaskMethod = "HTTP_PUT_JSON"
	// HTTPDelete method sends HTTP DELETE request
	HTTPDelete TaskMethod = "HTTP_DELETE"
	// WebSocket method sends websocket request
	WebSocket TaskMethod = "WEBSOCKET"
	// Shell method runs ant using shell command
	Shell TaskMethod = "SHELL"
	// Docker method runs ant using docker container
	Docker TaskMethod = "DOCKER"
	// Kubernetes method runs ant using kubernetes container
	Kubernetes TaskMethod = "KUBERNETES"
)

// SupportsCaptureStdout checks method can store stdout to a file
func (m TaskMethod) SupportsCaptureStdout() bool {
	return m == HTTPGet ||
		m == HTTPDelete ||
		m == HTTPPostForm ||
		m == HTTPPostJSON ||
		m == HTTPPutFORM ||
		m == HTTPPutJSON ||
		m == Shell
}

// RequiresScript checks method needs script
func (m TaskMethod) RequiresScript() bool {
	return m == HTTPGet ||
		m == HTTPDelete ||
		m == HTTPPostForm ||
		m == HTTPPostJSON ||
		m == HTTPPutFORM ||
		m == HTTPPutJSON ||
		m == Shell ||
		m == Docker ||
		m == Kubernetes
}

// IsValid checks method if it's valid
func (m TaskMethod) IsValid() bool {
	return m == HTTPGet ||
		m == HTTPDelete ||
		m == HTTPPostForm ||
		m == HTTPPostJSON ||
		m == HTTPPutFORM ||
		m == HTTPPutJSON ||
		m == WebSocket ||
		m == Shell ||
		m == Docker ||
		m == Kubernetes ||
		m == ForkJob ||
		m == AwaitForkedJob ||
		m == Messaging ||
		m == ExpireArtifacts
}

// SupportsDependentArtifacts  checks if method allows downloading dependent artifacts
func (m TaskMethod) SupportsDependentArtifacts() bool {
	return m == Shell ||
		m == Docker ||
		m == Kubernetes
}

// SupportsCache checks if method allows caching -- Shell doesn't need it because it can internally cache
func (m TaskMethod) SupportsCache() bool {
	return m == Docker || m == Kubernetes
}

// IsHTTP check if method is HTTP API
func (m TaskMethod) IsHTTP() bool {
	return m == HTTPGet ||
		m == HTTPDelete ||
		m == HTTPPostForm ||
		m == HTTPPostJSON ||
		m == HTTPPutJSON
}
