package nginx

import (
	"bytes"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"os"
	"os/exec"
	"syscall"
	"time"

	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/klog"
	logf "sigs.k8s.io/controller-runtime/pkg/runtime/log"
)

var log = logf.Log.WithName("controller")

// NGINX defines
type NGINX interface {
	// Start creates a new NGINX process
	Start(stopCh <-chan struct{}) error

	// Update changes the running configuration in NGINX
	Update(*Configuration) error
}

// NewInstance returns an NGINX instance
func NewInstance(path string) (NGINX, error) {
	tpl, err := newTemplate(path)
	if err != nil {
		return nil, err
	}

	return &nginx{
		template:             tpl,
		runningConfiguration: &Configuration{},
	}, nil
}

type nginx struct {
	template *template

	runningConfiguration *Configuration
}

func (ngx *nginx) Start(stopCh <-chan struct{}) error {
	cmd := nginxExecCommand()
	// put NGINX in another process group to prevent it
	// to receive signals meant for the controller
	cmd.SysProcAttr = &syscall.SysProcAttr{
		Setpgid: true,
		Pgid:    0,
	}

	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Start(); err != nil {
		return err
	}

	ngxErrCh := make(chan error)

	go func() {
		ngxErrCh <- cmd.Wait()
	}()

	go func(stopCh <-chan struct{}) {
		for {
			select {
			case err := <-ngxErrCh:
				log.Error(err, "nginx was terminated")
				cmd.Process.Release()
				// healthcheck will start failing
				break
			case <-stopCh:
				break
			}
		}
	}(stopCh)

	return nil
}

func (ngx *nginx) Update(cfg *Configuration) error {
	nginxConf, err := ngx.template.Render(cfg)
	if err != nil {
		return err
	}

	err = reloadIfRequired(nginxConf)
	if err != nil {
		return err
	}

	if ngx.runningConfiguration.Equal(cfg) {
		return nil
	}

	time.Sleep(2 * time.Second)

	err = updateConfiguration(cfg.Servers)
	if err != nil {
		return err
	}

	ngx.runningConfiguration = cfg

	log.V(2).Info("NGINX configuration", "cfg", cfg)

	return nil
}

// Binary location of NGINX binary.
var Binary = "/usr/local/openresty/nginx/sbin/nginx"

const (
	cfgPath         = "/etc/nginx/nginx.conf"
	readWriteByUser = 0660
)

func nginxExecCommand(args ...string) *exec.Cmd {
	cmdArgs := []string{}

	cmdArgs = append(cmdArgs, "-c", cfgPath)
	cmdArgs = append(cmdArgs, args...)
	return exec.Command(Binary, cmdArgs...)
}

// reloadIfRequired checks if the new configuration file is different from
// the one actually being used and a reload is required, triggering one
// after the check
func reloadIfRequired(data []byte) error {
	src, err := ioutil.ReadFile(cfgPath)
	if err != nil {
		return err
	}

	if bytes.Equal(src, data) {
		return nil
	}

	tmpfile, err := ioutil.TempFile("", "new-nginx-cfg")
	if err != nil {
		return err
	}

	tempFileName := tmpfile.Name()

	err = ioutil.WriteFile(tempFileName, data, readWriteByUser)
	if err != nil {
		return err
	}

	diffOutput, _ := exec.Command("diff", "-u", cfgPath, tempFileName).CombinedOutput()
	klog.Infof("NGINX configuration: \n%v", string(diffOutput))

	defer func() {
		tmpfile.Close()
		os.Remove(tempFileName)
	}()

	destination, err := os.Create(cfgPath)
	if err != nil {
		return err
	}

	_, err = io.Copy(destination, tmpfile)
	if err != nil {
		return err
	}

	log.Info("reloading nginx")
	cmd := nginxExecCommand("-s", "reload")
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	err = cmd.Run()
	if err != nil {
		return err
	}

	return nil
}

func updateConfiguration(servers []Server) error {
	retry := wait.Backoff{
		Steps:    15,
		Duration: 1 * time.Second,
		Factor:   0.8,
		Jitter:   0.1,
	}

	err := wait.ExponentialBackoff(retry, func() (bool, error) {
		statusCode, _, err := newPostStatusRequest("/configuration/backends", servers)
		if err != nil {
			return false, err
		}

		if statusCode != http.StatusCreated {
			return false, fmt.Errorf("unexpected error code: %d", statusCode)
		}

		return true, nil
	})

	return err
}
