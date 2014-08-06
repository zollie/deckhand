package deckhand

import (
	"github.com/samalba/dockerclient"
	"log"
	"os"
	"os/exec"
	"strings"
)

func (deckhand *Deckhand) containerDestroy(event *dockerclient.Event) {
	log.Printf("Processing destroy event for: %#v\n", *event)

	containerInfo, err := deckhand.Docker.InspectContainer(event.Id)
	if err != nil {
		log.Printf("Error inspecting container: %s - ignoring event", err)
		return
	}

	deckhand.Remove(containerInfo)
}

func (deckhand *Deckhand) containerStart(event *dockerclient.Event) {
	log.Printf("Processing start event for: %#v\n", *event)

	containerInfo, err := deckhand.Docker.InspectContainer(event.Id)
	if err != nil {
		log.Printf("Error inspecting container: %s - ignoring event", err)
		return
	}

	deckhand.Put(containerInfo)
}

func (deckhand *Deckhand) containerDie(event *dockerclient.Event) {
	log.Printf("Processing die event for: %#v\n", *event)

	containerInfo, err := deckhand.Docker.InspectContainer(event.Id)
	if err != nil {
		log.Printf("Error inspecting container: %s - ignoring event", err)
		return
	}

	deckhand.Remove(containerInfo)
}

// Run shell scripts based on event type
// This is modeled after serf's event handling mechanism
func (deckhand *Deckhand) shellEvents(event *dockerclient.Event) {
	log.Printf("Processing shell events for: %#v\n", *event)

	dir, err := os.Open(deckhand.HandlerDir)
	if err != nil {
		log.Printf("Error: %s", err)
		return
	}

	defer dir.Close()

	handlers, err := dir.Readdir(-1)
	if err != nil {
		log.Printf("Error: %s", err)
		return
	}

	// look for handlers that start with
	// event status and run them
	for _, h := range handlers {
		fname := h.Name()
		log.Printf("Found handler with name: %s", fname)
		if strings.HasPrefix(fname, event.Status) {
			cname := dir.Name() + fname
			log.Printf("Runnin command: %s", cname)
			cmd := exec.Command(cname, event.Id)
			out, err := cmd.Output()

			if err != nil {
				log.Fatal(err)
				return
			}

			log.Printf("Successfully ran handler %s: %s", fname, string(out))
		}
	}
}
