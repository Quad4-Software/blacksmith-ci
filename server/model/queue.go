package model

// QueueTask represents a task in the queue with additional API-specific fields.
type QueueTask struct {
	Task
	PipelineNumber  int64  `json:"pipeline_number"`
	AgentName       string `json:"agent_name"`
	PipelineCreated int64  `json:"pipeline_created"` // Unix timestamp when the pipeline was created
	PipelineStarted int64  `json:"pipeline_started"` // Unix timestamp when the pipeline started running (0 if not started)
	// StuckReason is "blocked" when the task waits on dependencies that left
	// the queue without succeeding, "waited_too_long" when it has waited past
	// the stuck threshold, and empty otherwise.
	StuckReason string `json:"stuck_reason,omitempty"`
}

// QueueInfo represents the response structure for queue information API.
type QueueInfo struct {
	Pending       []QueueTask `json:"pending"`
	WaitingOnDeps []QueueTask `json:"waiting_on_deps"`
	Running       []QueueTask `json:"running"`
	Stats         struct {
		WorkerCount        int `json:"worker_count"`
		PendingCount       int `json:"pending_count"`
		WaitingOnDepsCount int `json:"waiting_on_deps_count"`
		RunningCount       int `json:"running_count"`
	} `json:"stats"`
	Paused bool `json:"paused"`
	// StuckThreshold is how long, in seconds, a task may wait before it
	// counts as stuck; 0 when the age check is disabled.
	StuckThreshold int64 `json:"stuck_threshold"`
} //	@name	QueueInfo

// QueueCancelTasksRequest is the body of a batch cancel of queue tasks.
type QueueCancelTasksRequest struct {
	TaskIDs []string `json:"task_ids"`
} //	@name	QueueCancelTasksRequest

// QueueCancelTasksResponse reports the tasks a batch cancel failed for.
type QueueCancelTasksResponse struct {
	Canceled int               `json:"canceled"`
	Failed   map[string]string `json:"failed"`
} //	@name	QueueCancelTasksResponse

// StuckWorkflow is a queued workflow that is not making progress.
type StuckWorkflow struct {
	Name string `json:"name"`
	// Reason is "blocked" or "waited_too_long", see QueueTask.StuckReason.
	Reason string `json:"reason"`
} //	@name	StuckWorkflow

// StuckPipeline lists the stuck workflows of one pipeline for its author.
type StuckPipeline struct {
	RepoID         int64           `json:"repo_id"`
	RepoFullName   string          `json:"repo_full_name"`
	PipelineNumber int64           `json:"pipeline_number"`
	Created        int64           `json:"created"`
	Workflows      []StuckWorkflow `json:"workflows"`
} //	@name	StuckPipeline
