package info

type Info struct {
	Pac        *PacOpts
	Kube       *KubeOpts
	Controller *ControllerInfo
}

func (i *Info) DeepCopy(out *Info) {
	*out = *i
}

type (
	contextKey string
)
