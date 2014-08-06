package deckhand

import (
	"fmt"
	"github.com/samalba/dockerclient"
	"github.com/zollie/thalassa-aqueduct-client"
	"log"
	"strconv"
	"strings"
)

type HAProxy struct {
	docker *dockerclient.DockerClient
	client *aqueduct.Client
}

// holds the basics of a NAT
type nat struct {
	backIp    string
	backPort  int64
	frontPort int64
}

// easier container
type container struct {
	host string
	*dockerclient.ContainerInfo
	nats map[int64][]nat
}

func NewHAProxy(aqueductHost string, docker *dockerclient.DockerClient) (Proxy, error) {
	client, err := aqueduct.NewClient(aqueductHost)
	if err != nil {
		return nil, err
	}
	proxy := &HAProxy{
		docker: docker,
		client: client,
	}
	return proxy, nil
}

// New or updated container found in store
func (proxy *HAProxy) ContainerUpdate(containerInfo *dockerclient.ContainerInfo) {
	log.Printf("New or Updated Container found: %s", containerInfo.Id)

	c := &container{
		ContainerInfo: containerInfo,
		nats:          getNats(containerInfo),
	}

	log.Printf("nats are: %+v", c.nats)

	backends := getBackends(c)
	for k, v := range backends {
		live, _ := proxy.client.GetBackendByKey(k)
		mergeMembers(v, live)
		err := proxy.client.PutBackend(k, v)
		if err != nil {
			log.Printf("Error ensuring backend alive: %s", err)
		}
	}

	frontends := getFrontends(c)
	for k, v := range frontends {
		err := proxy.client.PutFrontend(k, v)
		if err != nil {
			log.Printf("Error ensuring frontend alive: %s", err)
		}
	}
}

// Container removed from store
func (proxy *HAProxy) ContainerRemoved(containerInfo *dockerclient.ContainerInfo) {
	log.Printf("Container removed: %s", containerInfo.Id)

	c := &container{
		ContainerInfo: containerInfo,
		nats:          getNats(containerInfo),
	}

	log.Printf("nats are: %+v", c.nats)

	backends := getBackends(c)
	for _, b := range backends {
		live, err := proxy.client.GetBackendByKey(b.Name)
		if err != nil {
			log.Printf("Error getting backend by key %s: %s", b.Name, err)
			return
		}
		if isLastMembers(live, b) {
			log.Print("This backend contains the last members, will tear down NATs")
			// drop the frontend first
			proxy.client.DeleteFrontend(live.Key)
			// now the back
			proxy.client.DeleteBackend(live.Key)
		} else {
			log.Print("Removing backend members")
			// remove Members from Backend
			for i, m := range live.Members {
				for _, bm := range b.Members {
					if m.Host == bm.Host && m.Port == bm.Port {
						end := i + 1
						if end == len(live.Members) {
							end--
						}
						live.Members = append(live.Members[:i], live.Members[end:]...)
					}
				}
			}
			key := live.Key
			live.Key = ""
			live.Id = ""
			err := proxy.client.PutBackend(key, live)
			if err != nil {
				log.Printf("Error ensuring backend alive: %s", err)
			}
		}
	}
}

func isLastMembers(live *aqueduct.Backend, b *aqueduct.Backend) bool {
	if len(live.Members) > len(b.Members) {
		return false
	}
	for m := range live.Members {
		for mm := range b.Members {
			if m != mm {
				return false
			}
		}
	}
	return true
}

// Build Frontends for container
func getFrontends(c *container) map[string]*aqueduct.Frontend {
	frontends := make(map[string]*aqueduct.Frontend)

	for frontPort, _ := range c.nats {
		fpStr := fmt.Sprintf("%d", frontPort)
		key := getEndKey(c, frontPort)
		f := &aqueduct.Frontend{
			Bind:    "*:" + fpStr,
			Backend: key,
			Type:    "static",
			Mode:    "tcp",
		}
		frontends[key] = f
	}
	return frontends
}

// Build Backends for container
func getBackends(c *container) map[string]*aqueduct.Backend {
	backends := make(map[string]*aqueduct.Backend)
	for frontPort, nats := range c.nats {
		key := getEndKey(c, frontPort)
		b := &aqueduct.Backend{
			Name:    key,
			Type:    "static",
			Mode:    "tcp",
			Balance: "roundrobin",
			Members: getMembers(nats),
		}
		backends[key] = b
	}
	return backends
}

// get an end key for a container and nat
func getEndKey(c *container, frontPort int64) string {
	key := fmt.Sprintf("%s:%d", c.Config.Image, frontPort)
	return key
}

// build Members from NAT
func getMembers(nats []nat) []aqueduct.Member {
	members := make([]aqueduct.Member, 0)
	for _, n := range nats {
		m := aqueduct.Member{
			Host: n.backIp,
			Port: n.backPort,
		}
		members = append(members, m)
	}
	return members
}

// Get NAT simpler structs
func getNats(containerInfo *dockerclient.ContainerInfo) map[int64][]nat {
	net := containerInfo.NetworkSettings
	natMap := make(map[int64][]nat, 0)
	for k, v := range net.Ports {
		fp, _ := stripPort(k) // TODO handle err
		nats := make([]nat, 0)
		for _, p := range v {
			hostPort, _ := strconv.ParseInt(p.HostPort, 10, 64)
			n := nat{
				backIp:    net.IpAddress,
				backPort:  hostPort,
				frontPort: fp,
			}
			nats = append(nats, n)
		}
		natMap[fp] = nats
	}

	return natMap
}

// merge members of two backends
func mergeMembers(b1 *aqueduct.Backend, b2 *aqueduct.Backend) {
	if b2 == nil {
		return
	}
	for _, m := range b1.Members {
		for _, mm := range b2.Members {
			if m.Host == mm.Host && m.Port == mm.Port {
				continue
			}
			b1.Members = append(b1.Members, mm)
		}
	}
}

// Get front port from docker port map
func stripPort(pmap string) (int64, error) {
	p := strings.Split(pmap, "/")[0]
	port, err := strconv.ParseInt(p, 10, 64)
	return port, err
}
