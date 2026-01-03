package crow

import "fmt"

const (
	pathAgents     = "%s/api/v1/agents"
	pathAgent      = "%s/api/v1/agents/%d"
	pathAgentTasks = "%s/api/v1/agents/%d/tasks"

	pathOrgAgents = "%s/api/v1/orgs/%d/agents"
	pathOrgAgent  = "%s/api/v1/orgs/%d/agents/%d"
)

// AgentCreate creates a new agent.
func (c *client) AgentCreate(in *Agent) (*Agent, error) {
	out := new(Agent)
	uri := fmt.Sprintf(pathAgents, c.addr)
	return out, c.post(uri, in, out)
}

// AgentList returns a list of all registered agents.
func (c *client) AgentList() ([]*Agent, error) {
	out := make([]*Agent, 0, 5)
	uri := fmt.Sprintf(pathAgents, c.addr)
	return out, c.get(uri, &out)
}

// Agent returns an agent by id.
func (c *client) Agent(agentID int64) (*Agent, error) {
	out := new(Agent)
	uri := fmt.Sprintf(pathAgent, c.addr, agentID)
	return out, c.get(uri, out)
}

// AgentUpdate updates the agent with the provided Agent struct.
func (c *client) AgentUpdate(in *Agent) (*Agent, error) {
	out := new(Agent)
	uri := fmt.Sprintf(pathAgent, c.addr, in.ID)
	return out, c.patch(uri, in, out)
}

// AgentDelete deletes the agent with the given id.
func (c *client) AgentDelete(agentID int64) error {
	uri := fmt.Sprintf(pathAgent, c.addr, agentID)
	return c.delete(uri)
}

// AgentTasksList returns a list of all tasks for the agent with the given id.
func (c *client) AgentTasksList(agentID int64) ([]*Task, error) {
	out := make([]*Task, 0, 5)
	uri := fmt.Sprintf(pathAgentTasks, c.addr, agentID)
	return out, c.get(uri, &out)
}

//
// Organization Agents.
//

// OrgAgentList returns a list of all agents for an organization.
func (c *client) OrgAgentList(orgID int64, opt ListOptions) ([]*Agent, error) {
	out := make([]*Agent, 0, 5)
	uri := fmt.Sprintf(pathOrgAgents+"?page=%d&perPage=%d", c.addr, orgID, opt.Page, opt.PerPage)
	return out, c.get(uri, &out)
}

// OrgAgentCreate creates a new organization-scoped agent.
func (c *client) OrgAgentCreate(orgID int64, in *Agent) (*Agent, error) {
	out := new(Agent)
	uri := fmt.Sprintf(pathOrgAgents, c.addr, orgID)
	return out, c.post(uri, in, out)
}

// OrgAgentUpdate updates an organization agent.
func (c *client) OrgAgentUpdate(orgID int64, in *Agent) (*Agent, error) {
	out := new(Agent)
	uri := fmt.Sprintf(pathOrgAgent, c.addr, orgID, in.ID)
	return out, c.patch(uri, in, out)
}

// OrgAgentDelete deletes an organization agent.
func (c *client) OrgAgentDelete(orgID, agentID int64) error {
	uri := fmt.Sprintf(pathOrgAgent, c.addr, orgID, agentID)
	return c.delete(uri)
}
