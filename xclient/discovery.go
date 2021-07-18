package xclient

import (
	"math"
	"math/rand"
	"sync"
	"time"
)

type SelectMode int

const (
	RandomSelect SelectMode = iota
	RoundRobinSelect
)

// Refresh 从注册中心更新服务列表
// Update 手动更新服务列表
// Get 根据负载均衡策略，选择一个服务实例

type Discovery interface {
	Refresh() error
	Update(servers []string) error
	Get(mod SelectMode) (string, error)
	GetAll() ([]string, error)
}

// MultiServerDiscovery a discovery for multi servers. (without registry center)
type MultiServerDiscovery struct {
	r       *rand.Rand
	mu      sync.Mutex // protect following
	servers []string
	index   int // record selected position for robin
}

func NewMultiServerDiscovery(servers []string) *MultiServerDiscovery {
	d := &MultiServerDiscovery{
		servers: servers,
		r: rand.New(rand.NewSource(time.Now().UnixNano())),
	}
	d.index = d.r.Intn(math.MaxInt32 - 1)
	return d
}

var _ Discovery = (*MultiServerDiscovery)(nil)


