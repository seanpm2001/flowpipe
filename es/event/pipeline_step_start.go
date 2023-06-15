package event

import (
	"fmt"

	"github.com/turbot/flowpipe/types"
	"github.com/turbot/flowpipe/util"
)

type PipelineStepStart struct {
	// Event metadata
	Event *Event `json:"event"`
	// Step execution details
	PipelineExecutionID string       `json:"pipeline_execution_id"`
	StepExecutionID     string       `json:"step_execution_id"`
	StepName            string       `json:"step_name"`
	StepInput           types.Input  `json:"input"`
	ForEach             *types.Input `json:"for_each,omitempty"`
	DelayMs             int          `json:"delay_ms,omitempty"` // delay start in milliseconds
}

// ExecutionOption is a function that modifies an Execution instance.
type PipelineStepStartOption func(*PipelineStepStart) error

// NewPipelineStepStart creates a new PipelineStepStart event.
func NewPipelineStepStart(opts ...PipelineStepStartOption) (*PipelineStepStart, error) {
	// Defaults
	e := &PipelineStepStart{
		StepExecutionID: util.NewStepExecutionID(),
	}
	// Set options
	for _, opt := range opts {
		err := opt(e)
		if err != nil {
			return e, err
		}
	}
	return e, nil
}

func ForPipelinePlanned(e *PipelinePlanned) PipelineStepStartOption {
	return func(cmd *PipelineStepStart) error {
		cmd.Event = NewChildEvent(e.Event)
		if e.PipelineExecutionID != "" {
			cmd.PipelineExecutionID = e.PipelineExecutionID
		} else {
			return fmt.Errorf("missing pipeline execution ID in pipeline planned event: %v", e)
		}
		return nil
	}
}

func WithDelay(delayMs int) PipelineStepStartOption {
	return func(cmd *PipelineStepStart) error {
		cmd.DelayMs = delayMs
		return nil
	}
}

func WithStep(name string, input types.Input, forEach *types.Input) PipelineStepStartOption {
	return func(cmd *PipelineStepStart) error {
		cmd.StepName = name
		cmd.StepInput = input
		cmd.ForEach = forEach
		return nil
	}
}
