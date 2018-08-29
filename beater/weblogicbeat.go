package beater

import (
	"crypto/tls"
	"fmt"
	"reflect"
	"time"

	gabs "github.com/Jeffail/gabs"
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

		// SERVERS STATUS
		//bt.ServerStatusEvent()

		// DATASOURCES STATUS
		bt.DatasourceStatusEvent()

		// APPLICATIONS STATUS
		//	bt.ApplicationStatusEvent()

		counter++
	}
}

func (bt *Weblogicbeat) ServerStatusEvent() {
	resp_server_status, err_server_status := resty.R().
		SetHeader("Accept", "application/json").
		SetHeader("X-Requested-By", "weblogicbeat").
		SetBasicAuth(bt.config.Username, bt.config.Password).
		Get(bt.config.Host + "/management/weblogic/latest/domainRuntime/serverRuntimes?links=none&fields=name,state,healthState")

	if err_server_status != nil {
		bt.SendErrorEvent(bt.config.ServerName, "server_status", fmt.Sprintf("%v", err_server_status))
		return
	}

	json_server_status, _ := gabs.ParseJSON([]byte(resp_server_status.String()))
	server := json_server_status.Data().(map[string]interface{})
	server_health := server["healthState"].(map[string]interface{})

	resp_server_jvm, err_server_jvm := resty.R().
		SetHeader("Accept", "application/json").
		SetHeader("X-Requested-By", "weblogicbeat").
		SetBasicAuth(bt.config.Username, bt.config.Password).
		Get(bt.config.Host + "/management/weblogic/latest/domainRuntime/serverRuntimes/" + bt.config.ServerName + "/JVMRuntime?links=none&fields=heapSizeCurrent,heapFreeCurrent,heapFreePercent,heapSizeMax,processCpuLoad")

	if err_server_jvm != nil {
		bt.SendErrorEvent(bt.config.ServerName, "server_status", fmt.Sprintf("%v", err_server_jvm))
		return
	}

	json_server_jvm, _ := gabs.ParseJSON([]byte(resp_server_jvm.String()))
	server_jvm := json_server_jvm.Data().(map[string]interface{})

	server_status_event := beat.Event{
		Timestamp: time.Now(),
		Fields: common.MapStr{
			"server":               server["name"],
			"metric_type":          "server_status",
			"srv_name":             server["name"],
			"srv_state":            server["state"],
			"srv_heapFreeCurrent":  int(server_jvm["heapFreeCurrent"].(float64) / 1000000),
			"srv_heapSizeCurrent":  int(server_jvm["heapSizeCurrent"].(float64) / 1000000),
			"srv_heapSizeMax":      int(server_jvm["heapSizeMax"].(float64) / 1000000),
			"srv_jvmProcessorLoad": server_jvm["processCpuLoad"].(float64),
			"srv_symptoms":         fmt.Sprintf("%v", server_health["symptoms"]),
			"srv_health":           server_health["state"],
		},
	}
	bt.client.Publish(server_status_event)
	logp.Info("Server status - event sent")
}

func (bt *Weblogicbeat) DatasourceStatusEvent() {

	for _, datasource := range bt.config.Datasources {
		resp_ds, error_ds := resty.R().
			SetHeader("Accept", "application/json").
			SetBasicAuth(bt.config.Username, bt.config.Password).
			Get(bt.config.Host + "/management/weblogic/latest/domainRuntime/serverRuntimes/" + bt.config.ServerName + "/JDBCServiceRuntime/JDBCDataSourceRuntimeMBeans/" + datasource + "?links=none&fields=activeConnectionsCurrentCount,activeConnectionsAverageCount,connectionsTotalCount,enabled,state,name")

		json_ds_status, _ := gabs.ParseJSON([]byte(resp_ds.String()))
		dsinfo := json_ds_status.Data().(map[string]interface{})

		if error_ds != nil {
			bt.SendErrorEvent(bt.config.ServerName, "datasource_status", fmt.Sprintf("%v", error_ds))
			return
		}

		_, error_ds_test := resty.R().
			SetHeader("Accept", "application/json").
			SetBasicAuth(bt.config.Username, bt.config.Password).
			Get(bt.config.Host + "/management/weblogic/latest/domainRuntime/serverRuntimes/" + bt.config.ServerName + "/JDBCServiceRuntime/JDBCDataSourceRuntimeMBeans/" + datasource + "/testPool")

		dstest_value := error_ds_test == nil

		if error_ds != nil {
			logp.Info("Error test datasource %s pool: %s", datasource, error_ds_test)
		}

		datasource_status_event := beat.Event{
			Timestamp: time.Now(),
			Fields: common.MapStr{
				"server":                           bt.config.ServerName,
				"metric_type":                      "datasource_status",
				"ds_name":                          datasource,
				"ds_state":                         dsinfo["state"],
				"ds_enabled":                       dsinfo["enabled"],
				"ds_activeConnectionsCurrentCount": dsinfo["activeConnectionsCurrentCount"],
				"ds_connectionsTotalCount":         dsinfo["connectionsTotalCount"],
				"ds_activeConnectionsAverageCount": dsinfo["activeConnectionsAverageCount"],
				"ds_testpool":                      dstest_value,
			},
		}
		bt.client.Publish(datasource_status_event)
		logp.Info("Datasource status - event sent")
	}
}

func (bt *Weblogicbeat) ApplicationStatusEvent() {
	for _, application := range bt.config.Applications {
		resp, _ := resty.R().
			SetHeader("Accept", "application/json").
			SetBasicAuth(bt.config.Username, bt.config.Password).
			Get(bt.config.Host + "/management/tenant-monitoring/applications/" + application)

		json_application_status, _ := gabs.ParseJSON([]byte(resp.String()))
		servers := reflect.ValueOf(json_application_status.Path("body.item.targetStates.target").Data())
		states := reflect.ValueOf(json_application_status.Path("body.item.targetStates.state").Data())
		health := fmt.Sprintf("%v", reflect.ValueOf(json_application_status.Path("body.item.health").Data()))

		wm_servers := reflect.ValueOf(json_application_status.Path("body.item.workManagers.server").Data())
		wm_pendingRequests := reflect.ValueOf(json_application_status.Path("body.item.workManagers.pendingRequests").Data())
		wm_completedRequests := reflect.ValueOf(json_application_status.Path("body.item.workManagers.completedRequests").Data())

		for i := 0; i < servers.Len(); i++ {
			server := fmt.Sprintf("%v", servers.Index(i))
			state := fmt.Sprintf("%v", states.Index(i))

			for e := 0; e < wm_servers.Len(); e++ {
				wm_server := fmt.Sprintf("%v", wm_servers.Index(e))
				if server == wm_server {
					wm_pendingRequest := fmt.Sprintf("%v", wm_pendingRequests.Index(e))
					wm_completedRequest := fmt.Sprintf("%v", wm_completedRequests.Index(e))

					application_status_event := beat.Event{
						Timestamp: time.Now(),
						Fields: common.MapStr{
							"server":                server,
							"metric_type":           "application_status",
							"app_name":              application,
							"app_state":             state,
							"app_health":            health,
							"app_pendingRequests":   wm_pendingRequest,
							"app_completedRequests": wm_completedRequest,
						},
					}
					bt.client.Publish(application_status_event)
					logp.Info("Application status - event sent")
				}
			}
		}
	}
}

func (bt *Weblogicbeat) SendErrorEvent(serverName string, metricType string, err string) {
	error_event := beat.Event{
		Timestamp: time.Now(),
		Fields: common.MapStr{
			"server":       serverName,
			"metric_type":  metricType,
			"metric_error": err,
		},
	}
	bt.client.Publish(error_event)
	logp.Info("Error : %s", err)
}

// Stop stops weblogicbeat.
func (bt *Weblogicbeat) Stop() {
	bt.client.Close()
	close(bt.done)
}
