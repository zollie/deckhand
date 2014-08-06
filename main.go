package main

import (
	"flag"
	"github.com/zollie/deckhand/deckhand"
	"strings"
)

// Main Entrypoint
func main() {
	aqueductHost := flag.String("aqueductHost", "http://localhost:10000", "aqueduct api url")
	dockerHost := flag.String("dockerHost", "unix:///var/run/docker.sock", "script handler directory")
	etcdHosts := flag.String("etcdHosts", "http://localhost:4001", "comma seperated list of etcd hosts")
	handlerDir := flag.String("handlerDir", "/tmp", "docker api url")
	mode := flag.String("mode", "slave", "master|slave")

	flag.Parse()

	machines := strings.Split(*etcdHosts, ",")

	config := &deckhand.Config{
		Mode:         *mode,
		HandlerDir:   *handlerDir,
		AqueductHost: *aqueductHost,
		DockerHost:   *dockerHost,
		EtcdHosts:    machines,
	}

	deckhand := deckhand.NewDeckhand(config)
	deckhand.Start()
}
