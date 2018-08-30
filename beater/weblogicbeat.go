package beater

import (
	"crypto/tls"
	"fmt"
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
		bt.ServerStatusEvent()

		// DATASOURCES STATUS
		bt.DatasourceStatusEvent()

		// APPLICATIONS STATUS
		bt.ApplicationStatusEvent()

		counter++
	}
}

func (bt *Weblogicbeat) ServerStatusEvent() {
	resp_server_status, err_server_status := resty.R().
		SetHeader("Accept", "application/json").
		SetHeader("X-Requested-By", "weblogicbeat").
		SetBasicAuth(bt.config.Username, bt.config.Password).
		Get(bt.config.Host + "/management/weblogic/latest/domainRuntime/serverRuntimes/" + bt.config.ServerName + "?links=none&fields=name,state,healthState")

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

	processCpuLoad := server_jvm["processCpuLoad"]
	if processCpuLoad == nil {
		processCpuLoad = 0
	} else {
		processCpuLoad = server_jvm["processCpuLoad"].(float64)
	}

	server_status_event := beat.Event{
		Timestamp: time.Now(),
		Fields: common.MapStr{
			"server":               bt.config.ServerName,
			"metric_type":          "server_status",
			"srv_name":             bt.ValidateString(server["name"]),
			"srv_state":            bt.ValidateString(server["state"]),
			"srv_heapFreeCurrent":  int(bt.ValidateFloat(server_jvm["heapFreeCurrent"]) / 1000000),
			"srv_heapSizeCurrent":  int(bt.ValidateFloat(server_jvm["heapSizeCurrent"]) / 1000000),
			"srv_heapSizeMax":      int(bt.ValidateFloat(server_jvm["heapSizeMax"]) / 1000000),
			"srv_jvmProcessorLoad": bt.ValidateInt(processCpuLoad),
			"srv_symptoms":         bt.ValidateString(fmt.Sprintf("%v", server_health["symptoms"])),
			"srv_health":           bt.ValidateString(server_health["state"]),
		},
	}
	bt.client.Publish(server_status_event)
	logp.Info("Server status - event sent")
}

func (bt *Weblogicbeat) DatasourceStatusEvent() {

	for _, datasource := range bt.config.Datasources {
		resp_ds, error_ds := resty.R().
			SetHeader("Accept", "application/json").
			SetHeader("X-Requested-By", "weblogicbeat").
			SetBasicAuth(bt.config.Username, bt.config.Password).
			Get(bt.config.Host + "/management/weblogic/latest/domainRuntime/serverRuntimes/" + bt.config.ServerName + "/JDBCServiceRuntime/JDBCDataSourceRuntimeMBeans/" + datasource + "?links=none&fields=activeConnectionsCurrentCount,activeConnectionsAverageCount,connectionsTotalCount,enabled,state,name")

		if error_ds != nil {
			bt.SendErrorEvent(bt.config.ServerName, "datasource_status", fmt.Sprintf("%v", error_ds))
			return
		}

		json_ds_status, _ := gabs.ParseJSON([]byte(resp_ds.String()))
		dsinfo := json_ds_status.Data().(map[string]interface{})

		_, error_ds_test := resty.R().
			SetHeader("Accept", "application/json").
			SetHeader("X-Requested-By", "weblogicbeat").
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
				"ds_state":                         bt.ValidateString(dsinfo["state"]),
				"ds_enabled":                       bt.ValidateString(dsinfo["enabled"]),
				"ds_activeConnectionsCurrentCount": bt.ValidateInt(dsinfo["activeConnectionsCurrentCount"]),
				"ds_connectionsTotalCount":         bt.ValidateInt(dsinfo["connectionsTotalCount"]),
				"ds_activeConnectionsAverageCount": bt.ValidateInt(dsinfo["activeConnectionsAverageCount"]),
				"ds_testpool":                      dstest_value,
			},
		}
		bt.client.Publish(datasource_status_event)
		logp.Info("Datasource status - event sent")
	}
}

func (bt *Weblogicbeat) ApplicationStatusEvent() {

	for _, application := range bt.config.Applications {
		resp_app, err_app := resty.R().
			SetHeader("Accept", "application/json").
			SetHeader("X-Requested-By", "weblogicbeat").
			SetBasicAuth(bt.config.Username, bt.config.Password).
			Get(bt.config.Host + "/management/weblogic/latest/domainRuntime/serverRuntimes/" + bt.config.ServerName + "/applicationRuntimes/" + application + "?links=none&fields=name,healthState")

		if err_app != nil {
			bt.SendErrorEvent(bt.config.ServerName, "application_status", fmt.Sprintf("%v", err_app))
			return
		}

		json_application_status, _ := gabs.ParseJSON([]byte(resp_app.String()))
		appinfo := json_application_status.Data().(map[string]interface{})
		server_health := appinfo["healthState"].(map[string]interface{})

		resp_app_comp, err_app_comp := resty.R().
			SetHeader("Accept", "application/json").
			SetHeader("X-Requested-By", "weblogicbeat").
			SetBasicAuth(bt.config.Username, bt.config.Password).
			Get(bt.config.Host + "/management/weblogic/latest/domainRuntime/serverRuntimes/" + bt.config.ServerName + "/applicationRuntimes/" + application + "/componentRuntimes?fields=openSessionsCurrentCount,sessionsOpenedTotalCount,openSessionsHighCount,applicationIdentifier,status,componentName&links=none")

		if err_app_comp != nil {
			bt.SendErrorEvent(bt.config.ServerName, "application_status", fmt.Sprintf("%v", err_app_comp))
			continue
		}

		json_comp_status, _ := gabs.ParseJSON([]byte(resp_app_comp.String()))
		items, _ := json_comp_status.S("items").Children()

		for _, child := range items {
			comp := child.Data().(map[string]interface{})

			application_status_event := beat.Event{
				Timestamp: time.Now(),
				Fields: common.MapStr{
					"server":                       bt.config.ServerName,
					"metric_type":                  "application_status",
					"app_name":                     application,
					"app_componentName":            bt.ValidateString(comp["componentName"]),
					"app_state":                    bt.ValidateString(comp["status"]),
					"app_health":                   bt.ValidateString(server_health["state"]),
					"app_openSessionsCurrentCount": bt.ValidateInt(comp["openSessionsCurrentCount"]),
					"app_sessionsOpenedTotalCount": bt.ValidateInt(comp["sessionsOpenedTotalCount"]),
					"app_openSessionsHighCount":    bt.ValidateInt(comp["openSessionsHighCount"]),
				},
			}
			bt.client.Publish(application_status_event)
			logp.Info("Application status - event sent")
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

func (bt *Weblogicbeat) ValidateString(value interface{}) string {
	r, err := value.(string)
	if !err {
		return "DATA_ERROR"
	} else if len(r) == 0 {
		return "NO_DATA"
	}
	return r
}

func (bt *Weblogicbeat) ValidateInt(value interface{}) int {
	r, err := value.(int)
	if !err {
		return -1
	}

	return r
}

func (bt *Weblogicbeat) ValidateFloat(value interface{}) float64 {
	r, err := value.(float64)
	if !err {
		return -1.0
	}
	return r
}

// Stop stops weblogicbeat.
func (bt *Weblogicbeat) Stop() {
	bt.client.Close()
	close(bt.done)
}
