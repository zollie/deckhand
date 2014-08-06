package deckhand

import (
	"encoding/json"
	"github.com/cenkalti/backoff"
	"github.com/coreos/go-etcd/etcd"
	"github.com/samalba/dockerclient"
	"log"
	// "time"
)

var etcdClient *etcd.Client

type EtcdRepo struct {
	Proxy
}

func NewEtcdRepo(machines []string, proxy Proxy) Repo {
	etcdClient = etcd.NewClient(machines)
	repo := &EtcdRepo{Proxy: proxy}
	return repo
}

func (repo *EtcdRepo) Put(containerInfo *dockerclient.ContainerInfo) error {
	log.Printf("Saving ContainerInfo to ETCD:\n%s", *containerInfo)

	b, err := json.MarshalIndent(containerInfo, "", "    ")
	if err != nil {
		log.Printf("Error marshaling ContainerInfo to JSON: %s - ignoring event", err)
		return err
	}

	str := string(b)
	log.Print(str)

	key := ROOT_KEY + "/" + containerInfo.Id

	log.Printf("Setting key: %s in ETCD for Container: %s ", key, containerInfo.Id)
	_, seterr := etcdClient.Set(key, str, 10000)

	return seterr
}

func (repo *EtcdRepo) Remove(containerInfo *dockerclient.ContainerInfo) error {
	key := ROOT_KEY + "/" + containerInfo.Id
	log.Printf("Removing ContainerInfo from ETCD with Key: %s", key)

	// We ignore error as another event prob already removed
	// Not sure thats the best approach though?
	etcdClient.Delete(key, true)

	return nil
}

// Watch the given key for changes
func (repo *EtcdRepo) Watch(key string) {
	repo.doWatch(key)
}

func (repo *EtcdRepo) doWatch(key string) {
	go func() {
		for {
			gotdata := false
			var c chan *etcd.Response

			go func() {
				for {
					r := <-c
					if r == nil {
						continue
					}
					gotdata = true
					log.Printf("New Action value is: %s", r.Action)
					log.Printf("Node is:\n%+v", r.Node)
					switch r.Action {
					case "set":
						repo.ContainerUpdate(getContainerInfo(r.Node.Value))
					case "delete":
						repo.ContainerRemoved(getContainerInfo(r.PrevNode.Value))
					}
				}
			}()

			b := backoff.NewExponentialBackOff()
			ticker := backoff.NewTicker(b)

			for _ = range ticker.C {
				c = make(chan *etcd.Response)
				_, err := etcdClient.Watch(key, 0, true, c, nil)
				if err != nil && !gotdata {
					log.Printf("etcd watch failed with %s, will retry using exponential backoff with max duration of 1m", err)
					continue // TODO
				}
				break
			}
		}
	}()
}

// util to get ContainerInfo from JSON
func getContainerInfo(v string) *dockerclient.ContainerInfo {
	var ci *dockerclient.ContainerInfo
	json.Unmarshal([]byte(v), &ci)
	log.Printf("Unmarshaled JSON is:\n%+v", ci)
	return ci
}
