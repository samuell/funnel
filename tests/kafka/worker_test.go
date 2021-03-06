package kafka

import (
	"context"
	workerCmd "github.com/ohsu-comp-bio/funnel/cmd/worker"
	"github.com/ohsu-comp-bio/funnel/config"
	"github.com/ohsu-comp-bio/funnel/events"
	"github.com/ohsu-comp-bio/funnel/logger"
	"github.com/ohsu-comp-bio/funnel/proto/tes"
	"github.com/ohsu-comp-bio/funnel/tests"
	"os"
	"testing"
)

var log = logger.NewLogger("kafka-worker-test", logger.DefaultConfig())
var fun *tests.Funnel
var conf config.Config

func TestMain(m *testing.M) {
	conf := tests.DefaultConfig()
	conf.Backend = "noop"

	var active bool
	for _, val := range conf.Worker.ActiveEventWriters {
		if val == "kafka" {
			active = true
		}
	}

	if !active {
		logger.Debug("Skipping kafka e2e tests...")
		os.Exit(0)
	}

	fun = tests.NewFunnel(conf)
	fun.StartServer()

	os.Exit(m.Run())
}

func TestKafkaWorkerRun(t *testing.T) {
	tests.SetLogOutput(log, t)

	task := &tes.Task{}
	b := events.TaskBuilder{Task: task}
	l := &events.Logger{Log: log}
	m := &events.MultiWriter{b, l}

	r, err := events.NewKafkaReader(conf.Worker.EventWriters.Kafka, m)
	defer r.Close()
	if err != nil {
		t.Fatal(err)
	}

	// this only writes the task to the DB since the 'noop'
	// compute backend is in use
	id := fun.Run(`
    --sh 'echo hello world'
  `)

	err = workerCmd.Run(context.Background(), conf.Worker, id, log)
	if err != nil {
		t.Fatal("unexpected error", err)
	}
	fun.Wait(id)

	if task.State != tes.State_COMPLETE {
		t.Fatal("unexpected state")
	}
}
