package config

import "testing"

func TestNodeResourceConfigParsing(t *testing.T) {
	yaml := `
Scheduler:
  Node:
    Resources:
      Cpus: 42
      RamGb: 2.5
      DiskGb: 50.0
`
	conf := Config{}
	Parse([]byte(yaml), &conf)

	if conf.Scheduler.Node.Resources.Cpus != 42 {
		t.Fatal("unexpected cpus")
	}
	if conf.Scheduler.Node.Resources.RamGb != 2.5 {
		t.Fatal("unexpected ram")
	}
	if conf.Scheduler.Node.Resources.DiskGb != 50.0 {
		t.Fatal("unexpected disk")
	}
}

func TestEnsureServerProperties(t *testing.T) {
	serverAddress := "test:9999"

	conf := Config{}
	conf.Server.HostName = "test"
	conf.Server.RPCPort = "9999"
	result := EnsureServerProperties(conf)

	if result.Server.RPCAddress() != serverAddress {
		t.Fatal("unexpected server address")
	}
	if result.Scheduler.Node.ServerAddress != serverAddress {
		t.Fatal("unexpected node server address")
	}
	if result.Worker.EventWriters.RPC.ServerAddress != serverAddress {
		t.Fatal("unexpected server address in worker config")
	}
	if result.Worker.TaskReaders.RPC.ServerAddress != serverAddress {
		t.Fatal("unexpected server address in worker config")
	}
}

func TestConfigParsing(t *testing.T) {
	conf := &Config{}
	err := ParseFile("./default-config.yaml", conf)
	if err != nil {
		t.Error("unexpected error:", err)
	}

	yaml := `
BadKey: foo
Scheduler:
  Node:
    Resources:
      Cpus: 42
      RamGb: 2.5
      DiskGb: 50.0
`
	conf = &Config{}
	err = Parse([]byte(yaml), conf)
	if err == nil {
		t.Error("expected error")
	}
}
