package elastic

import (
	"context"
	"github.com/golang/protobuf/jsonpb"
	"github.com/ohsu-comp-bio/funnel/events"
	"github.com/ohsu-comp-bio/funnel/proto/tes"
	elastic "gopkg.in/olivere/elastic.v5"
)

var updateTaskLogs = `
if (ctx._source.logs == null) {
  ctx._source.logs = new ArrayList();
}

// Ensure the task logs array is long enough.
for (; params.attempt > ctx._source.logs.length - 1; ) {
  Map m = new HashMap();
  m.logs = new ArrayList();
  ctx._source.logs.add(m);
}

// Set the field.
ctx._source.logs[params.attempt][params.field] = params.value;
`

var updateExecutorLogs = `
if (ctx._source.logs == null) {
  ctx._source.logs = new ArrayList();
}

// Ensure the task logs array is long enough.
for (; params.attempt > ctx._source.logs.length - 1; ) {
  Map m = new HashMap();
  m.logs = new ArrayList();
  ctx._source.logs.add(m);
}

// Ensure the executor logs array is long enough.
for (; params.index > ctx._source.logs[params.attempt].logs.length - 1; ) {
  Map m = new HashMap();
  ctx._source.logs[params.attempt].logs.add(m);
}

// Set the field.
ctx._source.logs[params.attempt].logs[params.index][params.field] = params.value;
`

func taskLogUpdate(attempt uint32, field string, value interface{}) *elastic.Script {
	return elastic.NewScript(updateTaskLogs).
		Lang("painless").
		Param("attempt", attempt).
		Param("field", field).
		Param("value", value)
}

func execLogUpdate(attempt, index uint32, field string, value interface{}) *elastic.Script {
	return elastic.NewScript(updateExecutorLogs).
		Lang("painless").
		Param("attempt", attempt).
		Param("index", index).
		Param("field", field).
		Param("value", value)
}

// WriteEvent writes a task update event.
func (es *Elastic) WriteEvent(ctx context.Context, ev *events.Event) error {
	// Skipping system logs for now. Will add them to the task logs when this PR is resolved (soon):
	// https://github.com/ga4gh/task-execution-schemas/pull/80
	if ev.Type == events.Type_SYSTEM_LOG {
		return nil
	}

	u := es.client.Update().
		Index(es.taskIndex).
		Type("task").
		RetryOnConflict(3).
		Id(ev.Id)

	switch ev.Type {
	case events.Type_TASK_CREATED:
		task := ev.GetTask()
		mar := jsonpb.Marshaler{}
		s, err := mar.MarshalToString(task)
		if err != nil {
			return err
		}

		_, err = es.client.Index().
			Index(es.taskIndex).
			Type("task").
			Id(task.Id).
			BodyString(s).
			Do(ctx)
		return err

	case events.Type_TASK_STATE:
		res, err := es.GetTask(ctx, &tes.GetTaskRequest{
			Id: ev.Id,
		})
		if err != nil {
			return err
		}

		from := res.State
		to := ev.GetState()
		if err := tes.ValidateTransition(from, to); err != nil {
			return err
		}
		u = u.Doc(map[string]string{"state": to.String()})

	case events.Type_TASK_START_TIME:
		u = u.Script(taskLogUpdate(ev.Attempt, "start_time", ev.GetStartTime()))

	case events.Type_TASK_END_TIME:
		u = u.Script(taskLogUpdate(ev.Attempt, "end_time", ev.GetEndTime()))

	case events.Type_TASK_OUTPUTS:
		u = u.Script(taskLogUpdate(ev.Attempt, "outputs", ev.GetOutputs().Value))

	case events.Type_TASK_METADATA:
		u = u.Script(taskLogUpdate(ev.Attempt, "metadata", ev.GetMetadata().Value))

	case events.Type_EXECUTOR_START_TIME:
		u = u.Script(execLogUpdate(ev.Attempt, ev.Index, "start_time", ev.GetStartTime()))

	case events.Type_EXECUTOR_END_TIME:
		u = u.Script(execLogUpdate(ev.Attempt, ev.Index, "end_time", ev.GetEndTime()))

	case events.Type_EXECUTOR_EXIT_CODE:
		u = u.Script(execLogUpdate(ev.Attempt, ev.Index, "exit_code", ev.GetExitCode()))

	case events.Type_EXECUTOR_STDOUT:
		u = u.Script(execLogUpdate(ev.Attempt, ev.Index, "stdout", ev.GetStdout()))

	case events.Type_EXECUTOR_STDERR:
		u = u.Script(execLogUpdate(ev.Attempt, ev.Index, "stderr", ev.GetStderr()))
	}

	_, err := u.Do(ctx)
	if elastic.IsNotFound(err) {
		return tes.ErrNotFound
	}
	return err
}
