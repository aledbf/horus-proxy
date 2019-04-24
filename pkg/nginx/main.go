package nginx

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"time"

	"github.com/tv42/httpunix"
)

// StatusSocket defines the location of the unix socket used by NGINX for the status server
var StatusSocket = "/tmp/nginx-config-socket.sock"

var socketClient = buildUnixSocketClient()

var statusLocation = "nginx-status"

// newPostStatusRequest creates a new POST request to the internal NGINX status server
func newPostStatusRequest(path, data interface{}) (int, []byte, error) {
	url := fmt.Sprintf("http+unix://%v%v", statusLocation, path)

	buf, err := json.Marshal(data)
	if err != nil {
		return 0, nil, err
	}

	res, err := socketClient.Post(url, "application/json", bytes.NewReader(buf))
	if err != nil {
		return 0, nil, err
	}
	defer res.Body.Close()

	body, err := ioutil.ReadAll(res.Body)
	if err != nil {
		return 0, nil, err
	}

	return res.StatusCode, body, nil
}

func buildUnixSocketClient() *http.Client {
	u := &httpunix.Transport{
		DialTimeout:           1 * time.Second,
		RequestTimeout:        10 * time.Second,
		ResponseHeaderTimeout: 10 * time.Second,
	}
	u.RegisterLocation(statusLocation, StatusSocket)

	return &http.Client{
		Transport: u,
	}
}
