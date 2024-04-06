package info

import "sync"

type Info struct {
	pacMutex   *sync.Mutex
	Pac        *PacOpts
	Kube       *KubeOpts
	Controller *ControllerInfo
}

func (i *Info) GetPacOpts() PacOpts {
	if i.pacMutex == nil {
		i.pacMutex = &sync.Mutex{}
	}
	i.pacMutex.Lock()
	defer i.pacMutex.Unlock()
	return *i.Pac
}

func (i *Info) DeepCopy(out *Info) {
	*out = *i
}

type (
	contextKey string
)
