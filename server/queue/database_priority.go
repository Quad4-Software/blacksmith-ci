package queue

import (
	"context"
	"time"

	"github.com/rs/zerolog/log"

	"codeberg.org/crowci/crow/v4/server/model"
)

// Poll retrieves and assigns a task to an agent with priority-based assignment.
func (q *DatabaseQueue) PollWithPriority(ctx context.Context, agentID int64, filter FilterFn) (*model.Task, error) {
	// Create a timeout to prevent indefinite blocking
	pollCtx, cancel := context.WithTimeout(ctx, maxPollDuration)
	defer cancel()

	// Create a channel to receive assigned task
	taskChan := make(chan *model.Task, 1)

	// Register this polling agent
	q.registerPollingAgent(agentID, filter, taskChan)
	defer q.unregisterPollingAgent(agentID)

	// Trigger assignment if we're the leader and enough time has passed
	q.triggerAssignmentIfNeeded()

	// Wait for task assignment or timeout
	select {
	case <-pollCtx.Done():
		// Return nil task without error to allow client to retry
		return nil, nil
	case task := <-taskChan:
		if task != nil {
			log.Debug().Str("taskID", task.ID).Int64("agentID", agentID).Msg("Agent received assigned task")
		}
		return task, nil
	}
}

// registerPollingAgent registers an agent as polling for work.
func (q *DatabaseQueue) registerPollingAgent(agentID int64, filter FilterFn, taskChan chan *model.Task) {
	q.pollingAgentsMu.Lock()
	defer q.pollingAgentsMu.Unlock()

	// Determine agent priority from filter (run with a dummy task)
	var priority int32
	if filter != nil {
		dummyTask := &model.Task{}
		_, _, priority = filter(dummyTask)
	}

	q.pollingAgents[agentID] = &pollingAgent{
		agentID:  agentID,
		filter:   filter,
		priority: priority,
		pollTime: time.Now(),
		taskChan: taskChan,
	}

	log.Debug().Int64("agentID", agentID).Int32("priority", priority).Msg("Agent registered for polling")
}

// unregisterPollingAgent removes an agent from the polling list.
func (q *DatabaseQueue) unregisterPollingAgent(agentID int64) {
	q.pollingAgentsMu.Lock()
	defer q.pollingAgentsMu.Unlock()

	if agent, exists := q.pollingAgents[agentID]; exists {
		close(agent.taskChan)
		delete(q.pollingAgents, agentID)
		log.Debug().Int64("agentID", agentID).Msg("Agent unregistered from polling")
	}
}

// triggerAssignmentIfNeeded checks if we should run task assignment.
func (q *DatabaseQueue) triggerAssignmentIfNeeded() {
	q.leaderMu.RLock()
	isLeader := q.isLeader
	q.leaderMu.RUnlock()

	if !isLeader {
		return
	}

	q.pollingAgentsMu.Lock()
	defer q.pollingAgentsMu.Unlock()

	// Run assignment if enough time has passed since last run
	// This batches assignment decisions for better priority handling
	if time.Since(q.lastAssignmentRun) > 50*time.Millisecond {
		q.lastAssignmentRun = time.Now()
		go q.runPriorityBasedAssignment()
	}
}

// runPriorityBasedAssignment assigns tasks to agents based on priority.
func (q *DatabaseQueue) runPriorityBasedAssignment() {
	q.mu.RLock()
	isPaused := q.paused
	q.mu.RUnlock()

	if isPaused {
		return
	}

	// Get all pending tasks
	tasks, err := q.store.TaskList()
	if err != nil {
		log.Error().Err(err).Msg("Failed to list tasks for priority assignment")
		return
	}

	// Get snapshot of polling agents
	q.pollingAgentsMu.RLock()
	agents := make([]*pollingAgent, 0, len(q.pollingAgents))
	for _, agent := range q.pollingAgents {
		agents = append(agents, agent)
	}
	q.pollingAgentsMu.RUnlock()

	if len(agents) == 0 || len(tasks) == 0 {
		return
	}

	// Group agents by priority
	agentsByPriority := make(map[int32][]*pollingAgent)
	var priorities []int32
	prioritySet := make(map[int32]bool)

	for _, agent := range agents {
		if !prioritySet[agent.priority] {
			priorities = append(priorities, agent.priority)
			prioritySet[agent.priority] = true
		}
		agentsByPriority[agent.priority] = append(agentsByPriority[agent.priority], agent)
	}

	// Sort priorities in descending order (highest first)
	for i := 0; i < len(priorities)-1; i++ {
		for j := i + 1; j < len(priorities); j++ {
			if priorities[i] < priorities[j] {
				priorities[i], priorities[j] = priorities[j], priorities[i]
			}
		}
	}

	// Track agent task counts for capacity-based assignment
	agentTaskCount := make(map[int64]int)
	agentPollingCount := make(map[int64]int)

	// Count existing running tasks per agent
	for _, task := range tasks {
		if task.AgentID != 0 {
			agentTaskCount[task.AgentID]++
		}
	}

	// Count polling instances per agent (similar to worker count in FIFO)
	for _, agent := range agents {
		agentPollingCount[agent.agentID]++
	}

	// Process tasks and assign to agents
	for _, task := range tasks {
		// Skip already assigned tasks
		if task.AgentID != 0 {
			continue
		}

		// Check if task is already in our local assignment tracking
		q.assignmentMu.Lock()
		if assignedAgent, exists := q.assignedTasks[task.ID]; exists && assignedAgent != 0 {
			q.assignmentMu.Unlock()
			continue
		}
		q.assignmentMu.Unlock()

		// Check dependencies
		if !q.areDependenciesMet(task, tasks) {
			continue
		}

		// Try to assign to highest priority agents first
		assigned := false
		for _, priority := range priorities {
			if assigned {
				break
			}

			priorityAgents := agentsByPriority[priority]

			// Find eligible agents for this task
			var eligibleAgents []*pollingAgent
			for _, agent := range priorityAgents {
				if agent.filter != nil {
					matched, _, _ := agent.filter(task)
					if matched {
						eligibleAgents = append(eligibleAgents, agent)
					}
				}
			}

			if len(eligibleAgents) == 0 {
				continue
			}

			// Among eligible agents with same priority, prefer those with more available capacity
			var bestAgent *pollingAgent
			const minScore = -1000
			bestScore := minScore

			for _, agent := range eligibleAgents {
				// Calculate available capacity score
				// Score = (polling instances - running tasks)
				// This mirrors the FIFO queue logic
				runningTasks := agentTaskCount[agent.agentID]
				pollingInstances := agentPollingCount[agent.agentID]
				score := pollingInstances - runningTasks

				if bestAgent == nil || score > bestScore {
					bestScore = score
					bestAgent = agent
				}
			}

			if bestAgent != nil {
				// Try to atomically assign the task
				atomicAssigned, err := q.atomicTaskAssignment(task.ID, bestAgent.agentID)
				if err != nil {
					log.Error().Err(err).Str("taskID", task.ID).Int64("agentID", bestAgent.agentID).
						Msg("Failed to atomically assign task in priority assignment")
					continue
				}

				if atomicAssigned {
					// Success! Update tracking and send task to agent
					q.assignmentMu.Lock()
					q.assignedTasks[task.ID] = bestAgent.agentID
					q.assignmentMu.Unlock()

					task.AgentID = bestAgent.agentID
					agentTaskCount[bestAgent.agentID]++

					// Send task to the agent's channel
					q.pollingAgentsMu.RLock()
					if agent, exists := q.pollingAgents[bestAgent.agentID]; exists {
						select {
						case agent.taskChan <- task:
							log.Debug().Str("taskID", task.ID).Int64("agentID", bestAgent.agentID).
								Int32("priority", priority).Int("score", bestScore).
								Msg("Task assigned to agent via priority assignment")
						default:
							log.Warn().Str("taskID", task.ID).Int64("agentID", bestAgent.agentID).
								Msg("Agent channel full, couldn't send task")
						}
					}
					q.pollingAgentsMu.RUnlock()

					assigned = true
				}
			}
		}
	}
}

// cleanupStalePollingAgents removes agents that have been polling for too long.
func (q *DatabaseQueue) cleanupStalePollingAgents() {
	q.pollingAgentsMu.Lock()
	defer q.pollingAgentsMu.Unlock()

	now := time.Now()
	for agentID, agent := range q.pollingAgents {
		if now.Sub(agent.pollTime) > maxPollDuration {
			close(agent.taskChan)
			delete(q.pollingAgents, agentID)
			log.Debug().Int64("agentID", agentID).Msg("Cleaned up stale polling agent")
		}
	}
}

// StartPriorityAssignmentLoop starts the background task assignment processor.
func (q *DatabaseQueue) StartPriorityAssignmentLoop() {
	ticker := time.NewTicker(100 * time.Millisecond)
	defer ticker.Stop()

	cleanupTicker := time.NewTicker(30 * time.Second)
	defer cleanupTicker.Stop()

	for {
		select {
		case <-q.ctx.Done():
			return
		case <-ticker.C:
			q.leaderMu.RLock()
			isLeader := q.isLeader
			q.leaderMu.RUnlock()

			if isLeader {
				q.runPriorityBasedAssignment()
			}
		case <-cleanupTicker.C:
			q.cleanupStalePollingAgents()
		}
	}
}
