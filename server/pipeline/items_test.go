package pipeline

import (
	"testing"

	"codefloe.com/crowci/crow/v5/pipeline/backend/types"
	"codefloe.com/crowci/crow/v5/server/model"
	sharedPipeline "codefloe.com/crowci/crow/v5/server/pipeline/stepbuilder"
)

func TestSetPipelineStepsOnPipeline(t *testing.T) {
	t.Parallel()

	pipeline := &model.Pipeline{
		ID:    1,
		Event: model.EventPush,
	}

	pipelineItems := []*sharedPipeline.Item{{
		Workflow: &model.Workflow{
			PID: 1,
		},
		Config: &types.Config{
			Stages: []*types.Stage{
				{
					Steps: []*types.Step{
						{
							Name: "clone",
						},
					},
				},
				{
					Steps: []*types.Step{
						{
							Name: "step",
						},
					},
				},
			},
		},
	}}
	pipeline = setPipelineStepsOnPipeline(pipeline, pipelineItems)
	if len(pipeline.Workflows) != 1 {
		t.Fatal("Should generate three in total")
	}
	if pipeline.Workflows[0].PipelineID != 1 {
		t.Fatal("Should set workflow's pipeline ID")
	}
	if pipeline.Workflows[0].Children[0].PPID != 1 {
		t.Fatal("Should set step PPID")
	}
}

func TestSetPipelineStepsOnPipelineWithDependsOn(t *testing.T) {
	t.Parallel()

	pipeline := &model.Pipeline{
		ID:    1,
		Event: model.EventPush,
	}

	pipelineItems := []*sharedPipeline.Item{{
		Workflow: &model.Workflow{
			PID: 1,
		},
		Config: &types.Config{
			Stages: []*types.Stage{
				{
					Steps: []*types.Step{
						{
							Name: "clone",
						},
					},
				},
				{
					Steps: []*types.Step{
						{
							Name:      "test",
							DependsOn: []string{"clone"},
						},
					},
				},
			},
		},
	}}
	pipeline = setPipelineStepsOnPipeline(pipeline, pipelineItems)

	if len(pipeline.Workflows) != 1 {
		t.Fatal("Should generate one workflow")
	}

	workflow := pipeline.Workflows[0]
	if len(workflow.Children) != 2 {
		t.Fatalf("Should have 2 steps, got %d", len(workflow.Children))
	}

	// Check that the test step has the correct depends_on
	testStep := workflow.Children[1] // Second step should be "test"
	if testStep.Name != "test" {
		t.Fatalf("Expected step name 'test', got '%s'", testStep.Name)
	}

	if len(testStep.DependsOn) != 1 {
		t.Fatalf("Expected 1 dependency, got %d", len(testStep.DependsOn))
	}

	if testStep.DependsOn[0] != "clone" {
		t.Fatalf("Expected dependency 'clone', got '%s'", testStep.DependsOn[0])
	}
}
