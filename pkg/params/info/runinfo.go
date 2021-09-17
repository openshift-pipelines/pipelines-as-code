package info

const defaultApplicationName = "Pipelines as Code CI"

type Info struct {
	Event *Event
	Pac   PacOpts
	Kube  KubeOpts
}
