package main

type status string

const (
	queued        status = "queued"
	starting      status = "starting"
	bootstrapping status = "bootstrapping"
	running       status = "running"
	stopping      status = "stopping"
	stopped       status = "stopped"
	failed        status = "failed"
	stopFailed    status = "stop_failed"
)

func active(s status) bool {
	return s == queued || s == starting || s == bootstrapping || s == running || s == stopping || s == stopFailed
}
