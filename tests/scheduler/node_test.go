package scheduler

import (
	"context"
	workercmd "github.com/ohsu-comp-bio/funnel/cmd/worker"
	"github.com/ohsu-comp-bio/funnel/compute/scheduler"
	"github.com/ohsu-comp-bio/funnel/logger"
	pbs "github.com/ohsu-comp-bio/funnel/proto/scheduler"
	"github.com/ohsu-comp-bio/funnel/proto/tes"
	"github.com/ohsu-comp-bio/funnel/tests"
	"testing"
	"time"
)

// When the node's context is canceled (e.g. because the process
// is being killed) the node should signal the database/server
// that it is gone, and the server will delete the node.
func TestNodeGoneOnCanceledContext(t *testing.T) {
	conf := tests.DefaultConfig()
	conf.Backend = "manual"
	conf.Scheduler.NodeInitTimeout = time.Second * 10
	conf.Scheduler.NodePingTimeout = time.Second * 10
	conf.Scheduler.NodeDeadTimeout = time.Second * 10

	bg := context.Background()
	log := logger.NewLogger("node", tests.LogConfig())
	tests.SetLogOutput(log, t)
	srv := tests.NewFunnel(conf)
	srv.StartServer()

	srv.Conf.Scheduler.Node.ID = "test-node-gone-on-cancel"
	n, err := scheduler.NewNode(srv.Conf, log, workercmd.Run)
	if err != nil {
		t.Fatal("failed to start node")
	}
	ctx, cancel := context.WithCancel(bg)
	defer cancel()
	go n.Run(ctx)

	srv.Scheduler.CheckNodes()
	time.Sleep(conf.Scheduler.Node.UpdateRate * 2)

	resp, err := srv.Scheduler.Nodes.ListNodes(bg, &pbs.ListNodesRequest{})
	if err != nil {
		t.Fatal(err)
	}
	nodes := resp.Nodes

	if len(nodes) != 1 {
		t.Fatal("failed to register node", nodes)
	}

	cancel()
	time.Sleep(conf.Scheduler.Node.UpdateRate * 2)
	srv.Scheduler.CheckNodes()

	resp, err = srv.Scheduler.Nodes.ListNodes(bg, &pbs.ListNodesRequest{})
	if err != nil {
		t.Fatal(err)
	}
	nodes = resp.Nodes

	if len(nodes) != 0 {
		t.Error("expected node to be deleted")
	}
}

// Run some tasks with the manual backend
func TestManualBackend(t *testing.T) {
	conf := tests.DefaultConfig()
	conf.Backend = "manual"
	conf.Scheduler.NodeInitTimeout = time.Second * 10
	conf.Scheduler.NodePingTimeout = time.Second * 10
	conf.Scheduler.NodeDeadTimeout = time.Second * 10

	log := logger.NewLogger("node", tests.LogConfig())
	tests.SetLogOutput(log, t)
	srv := tests.NewFunnel(conf)
	srv.StartServer()

	srv.Conf.Scheduler.Node.ID = "test-node-manual"
	n, err := scheduler.NewNode(srv.Conf, log, workercmd.Run)
	if err != nil {
		t.Fatal("failed to create node")
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go n.Run(ctx)

	tasks := []string{}
	for i := 0; i < 10; i++ {
		id := srv.Run(`
      --sh 'echo hello world'
    `)
		tasks = append(tasks, id)
	}

	for _, id := range tasks {
		task := srv.Wait(id)
		time.Sleep(time.Millisecond * 100)
		if task.State != tes.State_COMPLETE {
			t.Fatal("unexpected task state")
		}

		if task.Logs[0].Logs[0].Stdout != "hello world\n" {
			t.Fatalf("Missing stdout for task %s", id)
		}
	}
}
