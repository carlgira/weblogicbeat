package beater

import (
	"crypto/tls"
	"fmt"
	"time"

	"github.com/elastic/beats/libbeat/beat"
	"github.com/elastic/beats/libbeat/common"
	"github.com/elastic/beats/libbeat/logp"

	"github.com/carlgira/weblogicbeat/config"
	resty "gopkg.in/resty.v1"
)

// Weblogicbeat configuration.
type Weblogicbeat struct {
	done   chan struct{}
	config config.Config
	client beat.Client
}

// New creates an instance of weblogicbeat.
func New(b *beat.Beat, cfg *common.Config) (beat.Beater, error) {
	c := config.DefaultConfig
	if err := cfg.Unpack(&c); err != nil {
		return nil, fmt.Errorf("Error reading config file: %v", err)
	}

	bt := &Weblogicbeat{
		done:   make(chan struct{}),
		config: c,
	}

	// FIX add flag as parameter
	resty.SetTLSClientConfig(&tls.Config{InsecureSkipVerify: true})

	return bt, nil
}

// Run starts weblogicbeat.
func (bt *Weblogicbeat) Run(b *beat.Beat) error {
	logp.Info("weblogicbeat is running! Hit CTRL-C to stop it.")

	var err error
	bt.client, err = b.Publisher.Connect()
	if err != nil {
		return err
	}

	ticker := time.NewTicker(bt.config.Period)
	counter := 1
	for {
		select {
		case <-bt.done:
			return nil
		case <-ticker.C:
		}

		if bt.config.WlsVersion == "12.1.2" {
			wls := &Weblogic1212{
				bt:     *bt,
				config: bt.config,
			}
			wls.ServerStatusEvent()
			wls.DatasourceStatusEvent()
			wls.ApplicationStatusEvent()
			wls.ThreadStatusEvent()

		} else {
			wls := &Weblogic122{
				bt:     *bt,
				config: bt.config,
			}
			wls.ServerStatusEvent()
			wls.DatasourceStatusEvent()
			wls.ApplicationStatusEvent()
			wls.ThreadStatusEvent()
		}
		counter++
	}
}

// Stop stops weblogicbeat.
func (bt *Weblogicbeat) Stop() {
	bt.client.Close()
	close(bt.done)
}
