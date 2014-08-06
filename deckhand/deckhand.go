package deckhand

import (
	"github.com/samalba/dockerclient"
	"log"
	"os"
	"os/signal"
	"syscall"
)

var (
	HOST_NAME string
	ROOT_KEY  string
)

// Deckhand Config
type Config struct {
	Mode         string
	HandlerDir   string
	AqueductHost string
	DockerHost   string
	EtcdHosts    []string
}

// Where stuff is stored
type Repo interface {
	Put(containerInfo *dockerclient.ContainerInfo) error
	Remove(containerInfo *dockerclient.ContainerInfo) error
	Watch(key string)
}

// Manages the Proxy
type Proxy interface {
	ContainerUpdate(containerInfo *dockerclient.ContainerInfo)
	ContainerRemoved(containerInfo *dockerclient.ContainerInfo)
}

// Deckhand Client
type Deckhand struct {
	*Config
	Docker *dockerclient.DockerClient
	Repo
}

func init() {
	HOST_NAME, _ = os.Hostname()
	log.Printf("HOST_NAME set to %s", HOST_NAME)
}

// Default is Deckhand with ETCD and HAProxy
func NewDeckhand(config *Config) *Deckhand {
	if config.Mode == "master" {
		ROOT_KEY = "/masters"
	} else {
		ROOT_KEY = "/slaves/" + HOST_NAME
	}

	docker, err := dockerclient.NewDockerClient(config.DockerHost)
	if err != nil {
		log.Fatal(err)
		os.Exit(1)
	}

	proxy, perr := NewHAProxy(config.AqueductHost, docker)
	if perr != nil {
		log.Fatal(err)
		os.Exit(1)
	}

	repo := NewEtcdRepo(config.EtcdHosts, proxy)

	deckhand := &Deckhand{
		Config: config,
		Docker: docker,
		Repo:   repo,
	}

	return deckhand
}

// Start client
func (deckhand *Deckhand) Start() {
	log.Print("Starting Deckhand ...")

	// watch for new containers in etcd
	deckhand.Watch(ROOT_KEY)
	// watch for new dockers on host
	deckhand.Docker.StartMonitorEvents(dockerEventCallback, deckhand)
	// do until killed
	waitForInterrupt()
}

// Docker event callback
func (deckhand *Deckhand) dockerEvent(event *dockerclient.Event) {
	log.Printf("Processing new Docker event:\n%+v", *event)

	deckhand.shellEvents(event)

	switch event.Status {
	case "start":
		deckhand.containerStart(event)
	case "die":
		deckhand.containerDie(event)
	case "kill":
		// deckhand.containerKill(event)
	case "destroy":
		deckhand.containerDestroy(event)
	}
}

// Callback for Docker events
func dockerEventCallback(event *dockerclient.Event, args ...interface{}) {
	log.Printf("Received event:\n%+v", *event)

	deckhand := args[0].(*Deckhand)
	deckhand.dockerEvent(event)
}

// Wait until proc killed
func waitForInterrupt() {
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM, syscall.SIGQUIT)
	for _ = range sigChan {
		os.Exit(0)
	}
}
