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

type NGINX interface {
	Start(stopCh <-chan struct{}) error

	Update(*Configuration) error
}

func NewInstance(path string) (NGINX, error) {
	tpl, err := newTemplate(path)
	if err != nil {
		return nil, err
	}

	return &nginx{
		template: tpl,
	}, nil
}

type nginx struct {
	template *template
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
	log.Info("building nginx.conf")
	nginxConf, err := ngx.template.Render(cfg)
	if err != nil {
		return err
	}

	log.Info("checking if a reload is required")
	err = reloadIfRequired(nginxConf)
	if err != nil {
		return err
	}

	time.Sleep(100 * time.Millisecond)

	retry := wait.Backoff{
		Steps:    15,
		Duration: 1 * time.Second,
		Factor:   0.8,
		Jitter:   0.1,
	}

	log.Info("updating dynamic configuration")
	err = wait.ExponentialBackoff(retry, func() (bool, error) {
		statusCode, _, err := newPostStatusRequest("/configuration/backends", cfg.Servers)
		if err != nil {
			return false, err
		}

		if statusCode != http.StatusCreated {
			return false, fmt.Errorf("unexpected error code: %d", statusCode)
		}

		log.Info("dynamic reconfiguration succeeded")
		return true, nil
	})

	return err
}

// DefaultNGINXBinary default location of NGINX binary.
var DefaultNGINXBinary = "/usr/local/openresty/nginx/sbin/nginx"

const (
	cfgPath         = "/etc/nginx/nginx.conf"
	readWriteByUser = 0660
)

func nginxExecCommand(args ...string) *exec.Cmd {
	cmdArgs := []string{}

	cmdArgs = append(cmdArgs, "-c", cfgPath)
	cmdArgs = append(cmdArgs, args...)
	return exec.Command(DefaultNGINXBinary, cmdArgs...)
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
		log.Info("no need to reload nginx")
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
