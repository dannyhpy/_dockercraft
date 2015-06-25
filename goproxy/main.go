package main

import (
	"fmt"
	"github.com/samalba/dockerclient"
	"io"
	"net/http"
	"net/url"
	"os"
	"strconv"
	"strings"
)

var DOCKER_CLIENT *dockerclient.DockerClient

// Callback used to listen to Docker's events
func eventCallback(event *dockerclient.Event, ec chan error, args ...interface{}) {

	fmt.Println("---")
	fmt.Printf("%+v\n", *event)

	client := &http.Client{}

	id := event.Id

	switch event.Status {
	case "create":
		fmt.Println("create event")
	case "start":
		fmt.Println("start event")

		repo, tag := splitRepoAndTag(event.From)

		containerName := "<name>"

		containerInfo, err := DOCKER_CLIENT.InspectContainer(id)

		if err != nil {
			fmt.Print("InspectContainer error:", err.Error())
		} else {
			containerName = containerInfo.Name
		}

		data := url.Values{
			"action":    {"startContainer"},
			"id":        {id},
			"name":      {containerName},
			"imageRepo": {repo},
			"imageTag":  {tag}}

		MCServerRequest(data, client)

	case "stop":
		fmt.Println("stop event")

		repo, tag := splitRepoAndTag(event.From)

		containerName := "<name>"

		containerInfo, err := DOCKER_CLIENT.InspectContainer(id)

		if err != nil {
			fmt.Print("InspectContainer error:", err.Error())
		} else {
			containerName = containerInfo.Name
		}

		data := url.Values{
			"action":    {"stopContainer"},
			"id":        {id},
			"name":      {containerName},
			"imageRepo": {repo},
			"imageTag":  {tag}}

		MCServerRequest(data, client)

	case "restart":
		fmt.Println("restart event")
	case "kill":
		fmt.Println("kill event")
	case "die":
		fmt.Println("die event")
	}
}

func listContainers(w http.ResponseWriter, r *http.Request) {

	// answer right away to avoid dead locks in LUA
	io.WriteString(w, "OK")

	go func() {
		containers, err := DOCKER_CLIENT.ListContainers(true, false, "")

		if err != nil {
			fmt.Println(err.Error())
			return
		}

		images, err := DOCKER_CLIENT.ListImages()

		if err != nil {
			fmt.Println(err.Error())
			return
		}

		client := &http.Client{}

		for i := 0; i < len(containers); i++ {

			id := containers[i].Id
			info, _ := DOCKER_CLIENT.InspectContainer(id)
			name := info.Name[1:]
			imageRepo := ""
			imageTag := ""

			for _, image := range images {
				if image.Id == info.Image {
					if len(image.RepoTags) > 0 {
						imageRepo, imageTag = splitRepoAndTag(image.RepoTags[0])
					}
					break
				}
			}

			data := url.Values{
				"action":    {"containerInfos"},
				"id":        {id},
				"name":      {name},
				"imageRepo": {imageRepo},
				"imageTag":  {imageTag},
				"running":   {strconv.FormatBool(info.State.Running)},
			}

			MCServerRequest(data, client)
		}
	}()
}

func main() {

	// If there's an argument
	// It will be considered as a path for an HTTP GET request
	// That's a way to communicate with goproxy daemon
	if len(os.Args) > 1 {
		reqPath := "http://127.0.0.1:8000/" + os.Args[1]

		resp, err := http.Get(reqPath)
		if err != nil {
			fmt.Println("Error on request:", reqPath, "ERROR:", err.Error())
		} else {
			fmt.Println("Request sent", reqPath, "StatusCode:", resp.StatusCode)
		}
		return
	}

	// Init the client
	DOCKER_CLIENT, _ = dockerclient.NewDockerClient("unix:///var/run/docker.sock", nil)

	// Monitor events
	DOCKER_CLIENT.StartMonitorEvents(eventCallback, nil)

	go func() {
		http.HandleFunc("/containers", listContainers)
		http.ListenAndServe(":8000", nil)
	}()

	// wait for interruption
	<-make(chan int)
}

func splitRepoAndTag(repoTag string) (string, string) {

	repo := ""
	tag := ""

	repoAndTag := strings.Split(repoTag, ":")

	if len(repoAndTag) > 0 {
		repo = repoAndTag[0]
	}

	if len(repoAndTag) > 1 {
		tag = repoAndTag[1]
	}

	return repo, tag
}

// MCServerRequest send a POST request that will be handled
// by our MCServer Docker plugin.
func MCServerRequest(data url.Values, client *http.Client) {

	if client == nil {
		client = &http.Client{}
	}

	req, _ := http.NewRequest("POST", "http://127.0.0.1:8080/webadmin/Docker/Docker", strings.NewReader(data.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.SetBasicAuth("admin", "admin")
	client.Do(req)
}