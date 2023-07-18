package handler

import (
	"context"
	"reflect"

	"github.com/hashicorp/hcl/v2"
	"github.com/spf13/viper"
	"github.com/turbot/flowpipe/fperr"
	"github.com/turbot/flowpipe/internal/es/event"
	"github.com/turbot/flowpipe/internal/es/execution"
	"github.com/turbot/flowpipe/internal/fplog"
	"github.com/turbot/flowpipe/internal/types"
	"github.com/turbot/flowpipe/pipeparser"
)

type PipelinePlanned EventHandler

func (h PipelinePlanned) HandlerName() string {
	return "handler.pipeline_planned"
}

func (PipelinePlanned) NewEvent() interface{} {
	return &event.PipelinePlanned{}
}

func (h PipelinePlanned) Handle(ctx context.Context, ei interface{}) error {

	logger := fplog.Logger(ctx)
	e, ok := ei.(*event.PipelinePlanned)
	if !ok {
		logger.Error("invalid event type", "expected", "*event.PipelinePlanned", "actual", ei)
		return fperr.BadRequestWithMessage("invalid event type expected *event.PipelinePlanned")
	}

	logger.Info("[9] pipeline planned event handler #1", "executionID", e.Event.ExecutionID)

	ex, err := execution.NewExecution(ctx, execution.WithEvent(e.Event))
	if err != nil {
		return h.CommandBus.Send(ctx, event.NewPipelineFail(event.ForPipelinePlannedToPipelineFail(e, err)))
	}

	defn, err := ex.PipelineDefinition(e.PipelineExecutionID)
	if err != nil {
		return h.CommandBus.Send(ctx, event.NewPipelineFail(event.ForPipelinePlannedToPipelineFail(e, err)))
	}

	// Convenience
	pe := ex.PipelineExecutions[e.PipelineExecutionID]

	// If the pipeline has been canceled or paused, then no planning is required as no
	// more work should be done.
	if pe.IsCanceled() || pe.IsPaused() || pe.IsFinishing() || pe.IsFinished() {
		return nil
	}

	if len(e.NextSteps) == 0 {
		logger.Info("[9] pipeline planned event handler #2", "executionID", e.Event.ExecutionID)

		// PRE: No new steps to execute, so the planner should just check to see if
		// all existing steps are complete.
		if pe.IsComplete() {
			if pe.ShouldFail() {
				logger.Info("[9] pipeline planned event handler #4 - should fail", "executionID", e.Event.ExecutionID)

				// TODO: what is the error on the pipeline?
				cmd := event.NewPipelineFail(event.ForPipelinePlannedToPipelineFail(e, fperr.InternalWithMessage("pipeline failed")))
				if err != nil {
					return h.CommandBus.Send(ctx, event.NewPipelineFail(event.ForPipelinePlannedToPipelineFail(e, err)))
				}
				return h.CommandBus.Send(ctx, &cmd)
			} else {

				logger.Info("[9] pipeline planned event handler #5 - complete", "executionID", e.Event.ExecutionID)
				cmd, err := event.NewPipelineFinish(event.ForPipelinePlannedToPipelineFinish(e))
				if err != nil {
					return h.CommandBus.Send(ctx, event.NewPipelineFail(event.ForPipelinePlannedToPipelineFail(e, err)))
				}
				return h.CommandBus.Send(ctx, &cmd)
			}
		}

		return nil
	}

	// PRE: The planner has told us what steps to run next, our job is to start them
	for _, nextStep := range e.NextSteps {

		// data, err := ex.PipelineData(e.PipelineExecutionID)
		_, err := ex.PipelineData(e.PipelineExecutionID)
		if err != nil {
			return h.CommandBus.Send(ctx, event.NewPipelineFail(event.ForPipelinePlannedToPipelineFail(e, err)))
		}

		var forInputs reflect.Value
		// var forInputsType string

		stepDefn := defn.GetStep(nextStep.StepName)

		evalContext := hcl.EvalContext{
			Variables: ex.ExecutionVariables,
			Functions: pipeparser.ContextFunctions(viper.GetString("work.dir")),
		}

		stepForEach := stepDefn.GetForEach()
		if stepForEach != nil {
			paramsCtyVal, err := defn.ParamsAsCty()
			if err != nil {
				return h.CommandBus.Send(ctx, event.NewPipelineFail(event.ForPipelinePlannedToPipelineFail(e, err)))
			}
			evalContext.Variables["param"] = paramsCtyVal

			val, diags := stepForEach.Value(&evalContext)

			// resolve ForEach
			// var foreach interface{}
			// diags := gohcl.DecodeExpression(stepForEach, &evalContext, &foreach)
			if diags.HasErrors() {
				err := pipeparser.DiagsToError("execution", diags)
				return h.CommandBus.Send(ctx, event.NewPipelineFail(event.ForPipelinePlannedToPipelineFail(e, err)))
			}
			logger.Info("Val is", "val", val)
		}

		// if stepDefn.GetFor() != "" {
		// 	// Use go template with the step outputs to generate the items
		// 	t, err := template.New("for").Parse(stepDefn.GetFor())
		// 	if err != nil {
		// 		return h.CommandBus.Send(ctx, event.NewPipelineFail(event.ForPipelinePlannedToPipelineFail(e, err)))
		// 	}
		// 	var itemsBuffer bytes.Buffer
		// 	err = t.Execute(&itemsBuffer, data)
		// 	if err != nil {
		// 		return h.CommandBus.Send(ctx, event.NewPipelineFail(event.ForPipelinePlannedToPipelineFail(e, err)))
		// 	}
		// 	var rawForInputs interface{}
		// 	err = json.Unmarshal(itemsBuffer.Bytes(), &rawForInputs)
		// 	if err != nil {
		// 		return h.CommandBus.Send(ctx, event.NewPipelineFail(event.ForPipelinePlannedToPipelineFail(e, err)))
		// 	}
		// 	switch reflect.TypeOf(rawForInputs).Kind() {
		// 	case reflect.Map:
		// 		forInputsType = "map"
		// 		forInputs = reflect.ValueOf(rawForInputs)
		// 	case reflect.Slice:
		// 		forInputsType = "slice"
		// 		forInputs = reflect.ValueOf(rawForInputs)
		// 	}
		// 	if forInputs.Len() == 0 {
		// 		// A for loop was defined, but it returned no items, so we can
		// 		// skip this step
		// 		return nil
		// 	}
		// }

		// inputs will gather the input data for each step execution
		inputs := []types.Input{}

		// forEaches will record the "each" variable data for each step
		// execution in the loop
		forEaches := []*types.Input{}

		// Get input needs the Eval Context to resolve references
		stepInputs, err := stepDefn.GetInputs(&evalContext)
		if err != nil {
			logger.Error("Error resolving step inputs", "error", err)
			return err
		}

		if len(stepInputs) == 0 {
			// No input, so just use an empty input for each step execution.

			// There is always one input (e.g. no for loop). If the for loop had
			// no items, then we would have returned above.
			inputs = append(inputs, types.Input{})
			forEaches = append(forEaches, nil)

			// TODO: what happen if forInputs is invalid? Is this a real issue or not?
			if forInputs.IsValid() {
				// Add extra items if the for loop required them, skipping the one
				// we added already above.
				for i := 0; i < forInputs.Len()-1; i++ {
					// logger.Info("[8] pipeline planned event handler #9", "i", i, "inputs", inputs)
					inputs = append(inputs, types.Input{})
				}
			}

		} else {
			// We have an input

			// TODO: remove this line after ForEach is implemented, it should be done as part of the for_each calculation
			inputs = append(inputs, stepInputs)
			forEaches = append(forEaches, nil)

			// Parse the input template once

			// TODO: parse the input?
			// t, err := template.New("input").Parse(stepDefn.GetDeprecatedInput())
			// if err != nil {
			// 	return h.CommandBus.Send(ctx, event.NewPipelineFail(event.ForPipelinePlannedToPipelineFail(e, err)))
			// }

			// TODO: should we check for foInputs.IsValid() here? It was causing a panic before
			// TODO: when I didn't load the yaml file correctly
			// if stepDefn.GetFor() == "" {
			// 	// No for loop

			// 	// TODO: parse the input for each step execution ... do we need to do it here?

			// 	// stepInputs, err := stepDefn.GetInputs(nil)
			// 	// if err != nil {
			// 	// 	return err
			// 	// }
			// 	inputs = append(inputs, stepInputs)
			// 	forEaches = append(forEaches, nil)

			// } else {

			// 	switch forInputsType {
			// 	case "map":
			// 		// Create a step for each input in forInputs
			// 		for _, key := range forInputs.MapKeys() {
			// 			// TODO - this updates the same map each time ... is that safe?
			// 			var dataWithEach = data
			// 			forEach := types.Input{"key": key.Interface(), "value": forInputs.MapIndex(key).Interface()}
			// 			dataWithEach["each"] = forEach
			// 			// TODO: implement for
			// 		}

			// 		// var itemsBuffer bytes.Buffer

			// 		// 	err = t.Execute(&itemsBuffer, dataWithEach)
			// 		// 	if err != nil {
			// 		// 		return h.CommandBus.Send(ctx, event.NewPipelineFail(event.ForPipelinePlannedToPipelineFail(e, err)))
			// 		// 	}
			// 		// 	var input types.Input
			// 		// 	err = json.Unmarshal(itemsBuffer.Bytes(), &input)
			// 		// 	if err != nil {
			// 		// 		return h.CommandBus.Send(ctx, event.NewPipelineFail(event.ForPipelinePlannedToPipelineFail(e, err)))
			// 		// 	}
			// 		// 	inputs = append(inputs, input)
			// 		// 	forEaches = append(forEaches, &forEach)
			// 		// }

			// 	case "slice":

			// 		// Create a step for each input in forInputs
			// 		for i := 0; i < forInputs.Len(); i++ {
			// 			// TODO - this updates the same map each time ... is that safe?
			// 			var dataWithEach = data
			// 			forEach := types.Input{"key": i, "value": forInputs.Index(i).Interface()}
			// 			dataWithEach["each"] = forEach

			// 			// TODO: implement foreach

			// 			// var itemsBuffer bytes.Buffer
			// 			// err = t.Execute(&itemsBuffer, dataWithEach)
			// 			// if err != nil {
			// 			// 	return h.CommandBus.Send(ctx, event.NewPipelineFail(event.ForPipelinePlannedToPipelineFail(e, err)))
			// 			// }
			// 			// var input types.Input
			// 			// err = json.Unmarshal(itemsBuffer.Bytes(), &input)
			// 			// if err != nil {
			// 			// 	return h.CommandBus.Send(ctx, event.NewPipelineFail(event.ForPipelinePlannedToPipelineFail(e, err)))
			// 			// }
			// 			// inputs = append(inputs, input)
			// 			// forEaches = append(forEaches, &forEach)
			// 		}

			// 	default:
			// 		return h.CommandBus.Send(ctx, event.NewPipelineFail(event.ForPipelinePlannedToPipelineFail(e, fmt.Errorf("for loop must be a map or slice"))))

			// 	}
			// }

		}

		for i, input := range inputs {
			// Start each step in parallel
			go func(nextStep types.NextStep, input types.Input, forEach *types.Input) {
				cmd, err := event.NewPipelineStepQueue(event.PipelineStepQueueForPipelinePlanned(e), event.PipelineStepQueueWithStep(nextStep.StepName, input, forEach, nextStep.DelayMs))
				if err != nil {
					err := h.CommandBus.Send(ctx, event.NewPipelineFail(event.ForPipelinePlannedToPipelineFail(e, err)))
					if err != nil {
						fplog.Logger(ctx).Error("Error publishing event", "error", err)
					}

					return
				}

				logger.Info("[8] pipeline planned event handler #3.B - sending pipeline step QUEUE command", "command", cmd)
				if err := h.CommandBus.Send(ctx, &cmd); err != nil {
					err := h.CommandBus.Send(ctx, event.NewPipelineFail(event.ForPipelinePlannedToPipelineFail(e, err)))
					if err != nil {
						fplog.Logger(ctx).Error("Error publishing event", "error", err)
					}
					return
				}
			}(nextStep, input, forEaches[i])
		}
	}

	return nil
}
