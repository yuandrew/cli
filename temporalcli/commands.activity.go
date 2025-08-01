package temporalcli

import (
	"fmt"
	"time"

	"github.com/temporalio/cli/temporalcli/internal/printer"
	activitypb "go.temporal.io/api/activity/v1"
	"go.temporal.io/api/batch/v1"
	"go.temporal.io/api/common/v1"
	"go.temporal.io/api/failure/v1"
	taskqueuepb "go.temporal.io/api/taskqueue/v1"
	"go.temporal.io/api/workflowservice/v1"
	"google.golang.org/protobuf/types/known/durationpb"
	"google.golang.org/protobuf/types/known/fieldmaskpb"
)

type (
	updateOptionsDescribe struct {
		TaskQueue string

		ScheduleToCloseTimeout time.Duration
		ScheduleToStartTimeout time.Duration
		StartToCloseTimeout    time.Duration
		HeartbeatTimeout       time.Duration

		InitialInterval    time.Duration
		BackoffCoefficient float64
		MaximumInterval    time.Duration
		MaximumAttempts    int32
	}
)

func (c *TemporalActivityCompleteCommand) run(cctx *CommandContext, args []string) error {
	cl, err := c.Parent.ClientOptions.dialClient(cctx)
	if err != nil {
		return err
	}
	defer cl.Close()

	metadata := map[string][]byte{"encoding": []byte("json/plain")}
	resultPayloads, err := CreatePayloads([][]byte{[]byte(c.Result)}, metadata, false)
	if err != nil {
		return err
	}

	_, err = cl.WorkflowService().RespondActivityTaskCompletedById(cctx, &workflowservice.RespondActivityTaskCompletedByIdRequest{
		Namespace:  c.Parent.Namespace,
		WorkflowId: c.WorkflowId,
		RunId:      c.RunId,
		ActivityId: c.ActivityId,
		Result:     resultPayloads,
		Identity:   c.Identity,
	})
	if err != nil {
		return fmt.Errorf("unable to complete Activity: %w", err)
	}
	return nil
}

func (c *TemporalActivityFailCommand) run(cctx *CommandContext, args []string) error {
	cl, err := c.Parent.ClientOptions.dialClient(cctx)
	if err != nil {
		return err
	}
	defer cl.Close()

	var detailPayloads *common.Payloads
	if len(c.Detail) > 0 {
		metadata := map[string][]byte{"encoding": []byte("json/plain")}
		detailPayloads, err = CreatePayloads([][]byte{[]byte(c.Detail)}, metadata, false)
		if err != nil {
			return err
		}
	}
	_, err = cl.WorkflowService().RespondActivityTaskFailedById(cctx, &workflowservice.RespondActivityTaskFailedByIdRequest{
		Namespace:  c.Parent.Namespace,
		WorkflowId: c.WorkflowId,
		RunId:      c.RunId,
		ActivityId: c.ActivityId,
		Failure: &failure.Failure{
			Message: c.Reason,
			Source:  "CLI",
			FailureInfo: &failure.Failure_ApplicationFailureInfo{ApplicationFailureInfo: &failure.ApplicationFailureInfo{
				NonRetryable: true,
				Details:      detailPayloads,
			}},
		},
		Identity: c.Identity,
	})
	if err != nil {
		return fmt.Errorf("unable to fail Activity: %w", err)
	}
	return nil
}

func (c *TemporalActivityUpdateOptionsCommand) run(cctx *CommandContext, args []string) error {
	cl, err := c.Parent.ClientOptions.dialClient(cctx)
	if err != nil {
		return err
	}
	defer cl.Close()

	updatePath := []string{}
	activityOptions := &activitypb.ActivityOptions{}

	if c.Command.Flags().Changed("task-queue") {
		activityOptions.TaskQueue = &taskqueuepb.TaskQueue{Name: c.TaskQueue}
		updatePath = append(updatePath, "task_queue_name")
	}

	if c.Command.Flags().Changed("schedule-to-close-timeout") {
		activityOptions.ScheduleToCloseTimeout = durationpb.New(c.ScheduleToCloseTimeout.Duration())
		updatePath = append(updatePath, "schedule_to_close_timeout")
	}

	if c.Command.Flags().Changed("schedule-to-start-timeout") {
		activityOptions.ScheduleToStartTimeout = durationpb.New(c.ScheduleToStartTimeout.Duration())
		updatePath = append(updatePath, "schedule_to_start_timeout")
	}

	if c.Command.Flags().Changed("start-to-close-timeout") {
		activityOptions.StartToCloseTimeout = durationpb.New(c.StartToCloseTimeout.Duration())
		updatePath = append(updatePath, "start_to_close_timeout")
	}

	if c.Command.Flags().Changed("heartbeat-timeout") {
		activityOptions.HeartbeatTimeout = durationpb.New(c.HeartbeatTimeout.Duration())
		updatePath = append(updatePath, "heartbeat_timeout")
	}

	if c.Command.Flags().Changed("retry-initial-interval") ||
		c.Command.Flags().Changed("retry-maximum-interval") ||
		c.Command.Flags().Changed("retry-backoff-coefficient") ||
		c.Command.Flags().Changed("retry-maximum-attempts") {
		activityOptions.RetryPolicy = &common.RetryPolicy{}
	}

	if c.Command.Flags().Changed("retry-initial-interval") {
		activityOptions.RetryPolicy.InitialInterval = durationpb.New(c.RetryInitialInterval.Duration())
		updatePath = append(updatePath, "retry_policy.initial_interval")
	}

	if c.Command.Flags().Changed("retry-maximum-interval") {
		activityOptions.RetryPolicy.MaximumInterval = durationpb.New(c.RetryMaximumInterval.Duration())
		updatePath = append(updatePath, "retry_policy.maximum_interval")
	}

	if c.Command.Flags().Changed("retry-backoff-coefficient") {
		activityOptions.RetryPolicy.BackoffCoefficient = float64(c.RetryBackoffCoefficient)
		updatePath = append(updatePath, "retry_policy.backoff_coefficient")
	}

	if c.Command.Flags().Changed("retry-maximum-attempts") {
		activityOptions.RetryPolicy.MaximumAttempts = int32(c.RetryMaximumAttempts)
		updatePath = append(updatePath, "retry_policy.maximum_attempts")
	}

	result, err := cl.WorkflowService().UpdateActivityOptions(cctx, &workflowservice.UpdateActivityOptionsRequest{
		Namespace: c.Parent.Namespace,
		Execution: &common.WorkflowExecution{
			WorkflowId: c.WorkflowId,
			RunId:      c.RunId,
		},
		Activity:        &workflowservice.UpdateActivityOptionsRequest_Id{Id: c.ActivityId},
		ActivityOptions: activityOptions,
		UpdateMask: &fieldmaskpb.FieldMask{
			Paths: updatePath,
		},
		Identity: c.Identity,
	})
	if err != nil {
		return fmt.Errorf("unable to update Activity options: %w", err)
	}

	updatedOptions := updateOptionsDescribe{
		TaskQueue: result.GetActivityOptions().TaskQueue.GetName(),

		ScheduleToCloseTimeout: result.GetActivityOptions().ScheduleToCloseTimeout.AsDuration(),
		ScheduleToStartTimeout: result.GetActivityOptions().ScheduleToStartTimeout.AsDuration(),
		StartToCloseTimeout:    result.GetActivityOptions().StartToCloseTimeout.AsDuration(),
		HeartbeatTimeout:       result.GetActivityOptions().HeartbeatTimeout.AsDuration(),

		InitialInterval:    result.GetActivityOptions().RetryPolicy.InitialInterval.AsDuration(),
		BackoffCoefficient: result.GetActivityOptions().RetryPolicy.BackoffCoefficient,
		MaximumInterval:    result.GetActivityOptions().RetryPolicy.MaximumInterval.AsDuration(),
		MaximumAttempts:    result.GetActivityOptions().RetryPolicy.MaximumAttempts,
	}

	_ = cctx.Printer.PrintStructured(updatedOptions, printer.StructuredOptions{})

	return nil
}

func (c *TemporalActivityPauseCommand) run(cctx *CommandContext, args []string) error {
	cl, err := c.Parent.ClientOptions.dialClient(cctx)
	if err != nil {
		return err
	}
	defer cl.Close()

	request := &workflowservice.PauseActivityRequest{
		Namespace: c.Parent.Namespace,
		Execution: &common.WorkflowExecution{
			WorkflowId: c.WorkflowId,
			RunId:      c.RunId,
		},
		Identity: c.Identity,
	}

	if c.ActivityId != "" && c.ActivityType != "" {
		return fmt.Errorf("either Activity Type or Activity Id, but not both")
	}

	if c.ActivityType != "" {
		request.Activity = &workflowservice.PauseActivityRequest_Type{Type: c.ActivityType}
	} else if c.ActivityId != "" {
		request.Activity = &workflowservice.PauseActivityRequest_Id{Id: c.ActivityId}
	} else {
		return fmt.Errorf("either Activity Type or Activity Id must be provided")
	}

	_, err = cl.WorkflowService().PauseActivity(cctx, request)
	if err != nil {
		return fmt.Errorf("unable to pause Activity: %w", err)
	}

	return nil
}

func (c *TemporalActivityUnpauseCommand) run(cctx *CommandContext, args []string) error {
	cl, err := c.Parent.ClientOptions.dialClient(cctx)
	if err != nil {
		return err
	}
	defer cl.Close()

	opts := SingleWorkflowOrBatchOptions{
		WorkflowId: c.WorkflowId,
		RunId:      c.RunId,
		Query:      c.Query,
		Reason:     c.Reason,
		Yes:        c.Yes,
		Rps:        c.Rps,
	}

	exec, batchReq, err := opts.workflowExecOrBatch(cctx, c.Parent.Namespace, cl, singleOrBatchOverrides{
		// You're allowed to specify a reason when terminating a workflow
		AllowReasonWithWorkflowID: true,
	})
	if err != nil {
		return err
	}

	if exec != nil { // single workflow operation
		request := &workflowservice.UnpauseActivityRequest{
			Namespace: c.Parent.Namespace,
			Execution: &common.WorkflowExecution{
				WorkflowId: c.WorkflowId,
				RunId:      c.RunId,
			},
			ResetAttempts:  c.ResetAttempts,
			ResetHeartbeat: c.ResetHeartbeats,
			Jitter:         durationpb.New(c.Jitter.Duration()),
			Identity:       c.Identity,
		}

		if c.ActivityId != "" && c.ActivityType != "" {
			return fmt.Errorf("either Activity Type or Activity Id, but not both")
		}

		if c.ActivityType != "" {
			request.Activity = &workflowservice.UnpauseActivityRequest_Type{Type: c.ActivityType}
		} else if c.ActivityId != "" {
			request.Activity = &workflowservice.UnpauseActivityRequest_Id{Id: c.ActivityId}
		} else {
			return fmt.Errorf("either Activity Type or Activity Id must be provided")
		}

		_, err = cl.WorkflowService().UnpauseActivity(cctx, request)
		if err != nil {
			return fmt.Errorf("unable to uppause an Activity: %w", err)
		}
	} else { // batch operation
		unpauseActivitiesOperation := &batch.BatchOperationUnpauseActivities{
			Identity:       clientIdentity(),
			ResetAttempts:  c.ResetAttempts,
			ResetHeartbeat: c.ResetHeartbeats,
			Jitter:         durationpb.New(c.Jitter.Duration()),
		}
		if c.ActivityType != "" {
			unpauseActivitiesOperation.Activity = &batch.BatchOperationUnpauseActivities_Type{Type: c.ActivityType}
		} else if c.MatchAll == true {
			unpauseActivitiesOperation.Activity = &batch.BatchOperationUnpauseActivities_MatchAll{MatchAll: true}
		} else {
			return fmt.Errorf("either Activity Type must be provided or MatchAll must be set to true")
		}

		batchReq.Operation = &workflowservice.StartBatchOperationRequest_UnpauseActivitiesOperation{
			UnpauseActivitiesOperation: unpauseActivitiesOperation,
		}

		if err := startBatchJob(cctx, cl, batchReq); err != nil {
			return err
		}
	}

	return nil
}

func (c *TemporalActivityResetCommand) run(cctx *CommandContext, args []string) error {
	cl, err := c.Parent.ClientOptions.dialClient(cctx)
	if err != nil {
		return err
	}
	defer cl.Close()

	request := &workflowservice.ResetActivityRequest{
		Namespace: c.Parent.Namespace,
		Execution: &common.WorkflowExecution{
			WorkflowId: c.WorkflowId,
			RunId:      c.RunId,
		},
		Identity:       c.Identity,
		KeepPaused:     c.KeepPaused,
		ResetHeartbeat: c.ResetHeartbeats,
	}

	if c.ActivityId != "" && c.ActivityType != "" {
		return fmt.Errorf("either Activity Type or Activity Id, but not both")
	}

	if c.ActivityType != "" {
		request.Activity = &workflowservice.ResetActivityRequest_Type{Type: c.ActivityType}
	} else if c.ActivityId != "" {
		request.Activity = &workflowservice.ResetActivityRequest_Id{Id: c.ActivityId}
	} else {
		return fmt.Errorf("either Activity Type or Activity Id must be provided")
	}

	resp, err := cl.WorkflowService().ResetActivity(cctx, request)
	if err != nil {
		return fmt.Errorf("unable to reset an Activity: %w", err)
	}

	resetResponse := struct {
		KeepPaused      bool `json:"keepPaused"`
		ResetHeartbeats bool `json:"resetHeartbeats"`
		ServerResponse  bool `json:"-"`
	}{
		ServerResponse:  resp != nil,
		KeepPaused:      c.KeepPaused,
		ResetHeartbeats: c.ResetHeartbeats,
	}

	_ = cctx.Printer.PrintStructured(resetResponse, printer.StructuredOptions{})

	return nil
}
