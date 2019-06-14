package nginx

import (
	"bufio"
	"bytes"
	"encoding/json"
	"io/ioutil"
	text_template "text/template"

	"github.com/pkg/errors"
	"k8s.io/klog"
)

// Template NGINX template useed to render the configuration file
var Template = "/etc/nginx/template/nginx.tmpl"

type template struct {
	instance *text_template.Template
}

// newTemplate returns a new Template instance or an
// error if the specified template file contains errors
func newTemplate(path string) (*template, error) {
	data, err := ioutil.ReadFile(path)
	if err != nil {
		return nil, errors.Wrapf(err, "unexpected error reading template %v", path)
	}

	tmpl, err := text_template.New("nginx.tmpl").Funcs(funcMap).Parse(string(data))
	if err != nil {
		return nil, err
	}

	return &template{
		instance: tmpl,
	}, nil
}

// Write populates a buffer using a template with NGINX configuration
// and the servers and upstreams created by Ingress rules
func (t *template) Render(conf *Configuration) ([]byte, error) {
	if klog.V(3) {
		b, err := json.Marshal(conf)
		if err != nil {
			klog.Errorf("unexpected error: %v", err)
		}
		klog.Infof("NGINX configuration: %v", string(b))
	}

	var b bytes.Buffer
	bw := bufio.NewWriter(&b)

	err := t.instance.Execute(bw, conf)
	if err != nil {
		return nil, err
	}

	bw.Flush()
	return b.Bytes(), nil
}

var (
	funcMap = text_template.FuncMap{}
)
