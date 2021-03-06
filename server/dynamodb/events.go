package dynamodb

import (
	"context"
	"fmt"
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/awserr"
	"github.com/aws/aws-sdk-go/service/dynamodb"
	"github.com/aws/aws-sdk-go/service/dynamodb/dynamodbattribute"
	"github.com/ohsu-comp-bio/funnel/events"
	"github.com/ohsu-comp-bio/funnel/proto/tes"
	"strconv"
)

// WriteEvent creates an event for the server to handle.
func (db *DynamoDB) WriteEvent(ctx context.Context, e *events.Event) error {
	if e.Type == events.Type_TASK_CREATED {
		return db.createTask(ctx, e.GetTask())
	}

	item := &dynamodb.UpdateItemInput{
		TableName: aws.String(db.taskTable),
		Key: map[string]*dynamodb.AttributeValue{
			db.partitionKey: {
				S: aws.String(db.partitionValue),
			},
			"id": {
				S: aws.String(e.Id),
			},
		},
	}

	switch e.Type {

	case events.Type_TASK_STATE:
		task, err := db.GetTask(ctx, &tes.GetTaskRequest{
			Id:   e.Id,
			View: tes.TaskView_MINIMAL,
		})
		if err != nil {
			return err
		}

		from := task.State
		to := e.GetState()
		if err := tes.ValidateTransition(from, to); err != nil {
			return err
		}
		item.ExpressionAttributeNames = map[string]*string{
			"#state": aws.String("state"),
		}
		item.ExpressionAttributeValues = map[string]*dynamodb.AttributeValue{
			":to": {
				N: aws.String(strconv.Itoa(int(to))),
			},
		}
		item.UpdateExpression = aws.String("SET #state = :to")

	case events.Type_TASK_START_TIME:
		if err := db.ensureTaskLog(ctx, e.Id, e.Attempt); err != nil {
			return err
		}
		item.UpdateExpression = aws.String(fmt.Sprintf("SET logs[%v].start_time = :c", e.Attempt))
		item.ExpressionAttributeValues = map[string]*dynamodb.AttributeValue{
			":c": {
				S: aws.String(e.GetStartTime()),
			},
		}

	case events.Type_TASK_END_TIME:
		if err := db.ensureTaskLog(ctx, e.Id, e.Attempt); err != nil {
			return err
		}
		item.UpdateExpression = aws.String(fmt.Sprintf("SET logs[%v].end_time = :c", e.Attempt))
		item.ExpressionAttributeValues = map[string]*dynamodb.AttributeValue{
			":c": {
				S: aws.String(e.GetEndTime()),
			},
		}

	case events.Type_TASK_OUTPUTS:
		if err := db.ensureTaskLog(ctx, e.Id, e.Attempt); err != nil {
			return err
		}
		val, err := dynamodbattribute.MarshalList(e.GetOutputs().Value)
		if err != nil {
			return fmt.Errorf("failed to DynamoDB marshal TaskLog Outputs, %v", err)
		}
		item.UpdateExpression = aws.String(fmt.Sprintf("SET logs[%v].outputs = :c", e.Attempt))
		item.ExpressionAttributeValues = map[string]*dynamodb.AttributeValue{
			":c": {
				L: val,
			},
		}

	case events.Type_TASK_METADATA:
		if err := db.ensureTaskLog(ctx, e.Id, e.Attempt); err != nil {
			return err
		}
		val, err := dynamodbattribute.MarshalMap(e.GetMetadata().Value)
		if err != nil {
			return fmt.Errorf("failed to DynamoDB marshal TaskLog Metadata, %v", err)
		}

		item.UpdateExpression = aws.String(fmt.Sprintf("SET logs[%v].metadata = :c", e.Attempt))
		item.ExpressionAttributeValues = map[string]*dynamodb.AttributeValue{
			":c": {
				M: val,
			},
		}

	case events.Type_EXECUTOR_START_TIME:
		if err := db.ensureExecLog(ctx, e.Id, e.Attempt, e.Index); err != nil {
			return err
		}
		item.UpdateExpression = aws.String(fmt.Sprintf("SET logs[%v].logs[%v].start_time = :c", e.Attempt, e.Index))
		item.ExpressionAttributeValues = map[string]*dynamodb.AttributeValue{
			":c": {
				S: aws.String(e.GetStartTime()),
			},
		}

	case events.Type_EXECUTOR_END_TIME:
		if err := db.ensureExecLog(ctx, e.Id, e.Attempt, e.Index); err != nil {
			return err
		}
		item.UpdateExpression = aws.String(fmt.Sprintf("SET logs[%v].logs[%v].end_time = :c", e.Attempt, e.Index))
		item.ExpressionAttributeValues = map[string]*dynamodb.AttributeValue{
			":c": {
				S: aws.String(e.GetEndTime()),
			},
		}

	case events.Type_EXECUTOR_EXIT_CODE:
		if err := db.ensureExecLog(ctx, e.Id, e.Attempt, e.Index); err != nil {
			return err
		}
		item.UpdateExpression = aws.String(fmt.Sprintf("SET logs[%v].logs[%v].exit_code = :c", e.Attempt, e.Index))
		item.ExpressionAttributeValues = map[string]*dynamodb.AttributeValue{
			":c": {
				N: aws.String(strconv.Itoa(int(e.GetExitCode()))),
			},
		}

	case events.Type_EXECUTOR_STDOUT:
		stdout := e.GetStdout()
		item = &dynamodb.UpdateItemInput{
			TableName: aws.String(db.stdoutTable),
			Key: map[string]*dynamodb.AttributeValue{
				"id": {
					S: aws.String(e.Id),
				},
				"attempt_index": {
					S: aws.String(fmt.Sprintf("%v-%v", e.Attempt, e.Index)),
				},
			},
			ExpressionAttributeNames: map[string]*string{
				"#index": aws.String("index"),
			},
			UpdateExpression: aws.String("SET stdout = :stdout, attempt = :attempt, #index = :index"),
			ExpressionAttributeValues: map[string]*dynamodb.AttributeValue{
				":stdout": {
					S: aws.String(stdout),
				},
				":attempt": {
					N: aws.String(strconv.Itoa(int(e.Attempt))),
				},
				":index": {
					N: aws.String(strconv.Itoa(int(e.Index))),
				},
			},
		}

	case events.Type_EXECUTOR_STDERR:
		item = &dynamodb.UpdateItemInput{
			TableName: aws.String(db.stderrTable),
			Key: map[string]*dynamodb.AttributeValue{
				"id": {
					S: aws.String(e.Id),
				},
				"attempt_index": {
					S: aws.String(fmt.Sprintf("%v-%v", e.Attempt, e.Index)),
				},
			},
			ExpressionAttributeNames: map[string]*string{
				"#index": aws.String("index"),
			},
			UpdateExpression: aws.String("SET stderr = :stderr, attempt = :attempt, #index = :index"),
			ExpressionAttributeValues: map[string]*dynamodb.AttributeValue{
				":stderr": {
					S: aws.String(e.GetStderr()),
				},
				":attempt": {
					N: aws.String(strconv.Itoa(int(e.Attempt))),
				},
				":index": {
					N: aws.String(strconv.Itoa(int(e.Index))),
				},
			},
		}
	}

	_, err := db.client.UpdateItemWithContext(ctx, item)
	return checkErrNotFound(err)
}

func (db *DynamoDB) ensureTaskLog(ctx context.Context, id string, attempt uint32) error {

	// create the log structure for the attempt if it doesnt already exist
	attemptItem := &dynamodb.UpdateItemInput{
		TableName: aws.String(db.taskTable),
		Key: map[string]*dynamodb.AttributeValue{
			db.partitionKey: {
				S: aws.String(db.partitionValue),
			},
			"id": {
				S: aws.String(id),
			},
		},
		UpdateExpression: aws.String(fmt.Sprintf("SET logs[%v] = if_not_exists(logs[%v], :v)", attempt, attempt)),
		ExpressionAttributeValues: map[string]*dynamodb.AttributeValue{
			":v": {
				M: map[string]*dynamodb.AttributeValue{},
			},
		},
	}
	_, err := db.client.UpdateItemWithContext(ctx, attemptItem)
	return checkErrNotFound(err)
}

func (db *DynamoDB) ensureExecLog(ctx context.Context, id string, attempt, index uint32) error {
	// create the log structure for the executor if it doesnt already exist
	indexItem := &dynamodb.UpdateItemInput{
		TableName: aws.String(db.taskTable),
		Key: map[string]*dynamodb.AttributeValue{
			db.partitionKey: {
				S: aws.String(db.partitionValue),
			},
			"id": {
				S: aws.String(id),
			},
		},
		UpdateExpression: aws.String(fmt.Sprintf("SET logs[%v].logs[%v] = if_not_exists(logs[%v].logs[%v], :v)", attempt, index, attempt, index)),
		ExpressionAttributeValues: map[string]*dynamodb.AttributeValue{
			":v": {
				M: map[string]*dynamodb.AttributeValue{},
			},
		},
	}
	_, err := db.client.UpdateItemWithContext(ctx, indexItem)
	return checkErrNotFound(err)
}

func checkErrNotFound(err error) error {
	if aerr, ok := err.(awserr.Error); ok {
		if aerr.Code() == dynamodb.ErrCodeResourceNotFoundException {
			return tes.ErrNotFound
		}
	}
	return err
}
