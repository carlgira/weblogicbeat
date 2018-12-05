package beater

import (
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
type Weblogic1212 struct {
	bt     Weblogicbeat
	config config.Config
}

func (wls *Weblogic1212) ServerStatusEvent() {

	for _, server_name := range wls.config.ServerNames {
		resp_server_status, err_server_status := resty.R().
			SetHeader("Accept", "application/json").
			SetHeader("X-Requested-By", "weblogicbeat").
			SetBasicAuth(wls.config.Username, wls.config.Password).
			Get(wls.config.Host + "/management/tenant-monitoring/servers/" + server_name)

		if resp_server_status.StatusCode() != 200 {
			wls.SendErrorEvent(server_name, "server_status", fmt.Sprintf("%v", err_server_status), fmt.Sprintf("%v", resp_server_status))
			continue
		}

		json_server_status, _ := gabs.ParseJSON([]byte(resp_server_status.String()))
		server := json_server_status.Path("body.item").Data().(map[string]interface{})

		server_status_event := beat.Event{
			Timestamp: time.Now(),
			Fields: common.MapStr{
				"wb_server":           server_name,
				"wb_metric_type":      "server_status",
				"srv_name":            server["name"],
				"srv_state":           server["state"],
				"srv_heapFreeCurrent": int(server["heapFreeCurrent"].(float64) / 1000000),
				"srv_heapSizeCurrent": int(server["heapSizeCurrent"].(float64) / 1000000),
				"srv_heapSizeMax":     int(server["heapSizeMax"].(float64) / 1000000),
				"srv_health":          server["health"],
			},
		}
		wls.bt.client.Publish(server_status_event)
		logp.Info("Server status %s - event sent", server_name)
	}
}

func (wls *Weblogic1212) DatasourceStatusEvent() {

	for _, datasource := range wls.config.Datasources {
		resp_ds, error_ds := resty.R().
			SetHeader("Accept", "application/json").
			SetHeader("X-Requested-By", "weblogicbeat").
			SetBasicAuth(wls.config.Username, wls.config.Password).
			Get(wls.config.Host + "/management/tenant-monitoring/datasources/" + datasource)

		if resp_ds.StatusCode() != 200 {
			wls.SendErrorEvent(datasource, "datasource_status", fmt.Sprintf("%v", error_ds), fmt.Sprintf("%v", resp_ds))
			continue
		}

		json_ds_status, _ := gabs.ParseJSON([]byte(resp_ds.String()))
		dsinfo, _ := json_ds_status.Path("body.item.instances").Children()

		for _, child := range dsinfo {
			ds := child.Data().(map[string]interface{})

			if !stringInSlice(ds["server"].(string), wls.config.ServerNames) {
				continue
			}

			datasource_status_event := beat.Event{
				Timestamp: time.Now(),
				Fields: common.MapStr{
					"wb_server":                        ds["server"],
					"wb_metric_type":                   "datasource_status",
					"ds_server":                        ds["server"],
					"ds_name":                          datasource,
					"ds_state":                         ds["state"],
					"ds_enabled":                       ds["enabled"],
					"ds_activeConnectionsCurrentCount": ds["activeConnectionsCurrentCount"],
					"ds_connectionsTotalCount":         ds["connectionsTotalCount"],
					"ds_activeConnectionsAverageCount": ds["activeConnectionsAverageCount"],
				},
			}
			wls.bt.client.Publish(datasource_status_event)
			logp.Info("Datasource status %s - event sent", ds["server"])
		}
	}

}

func (wls *Weblogic1212) ApplicationStatusEvent() {

	for _, application := range wls.config.Applications {
		resp_app, err_app := resty.R().
			SetHeader("Accept", "application/json").
			SetHeader("X-Requested-By", "weblogicbeat").
			SetBasicAuth(wls.config.Username, wls.config.Password).
			Get(wls.config.Host + "/management/tenant-monitoring/applications/" + application)

		if resp_app.StatusCode() != 200 {
			wls.SendErrorEvent(application, "application_status", fmt.Sprintf("%v", err_app), fmt.Sprintf("%v", resp_app))
			continue
		}

		json_application_status, _ := gabs.ParseJSON([]byte(resp_app.String()))
		appinfo := json_application_status.Path("body.item").Data().(map[string]interface{})

		for _, server_name := range wls.config.ServerNames {
			application_status_event := beat.Event{
				Timestamp: time.Now(),
				Fields: common.MapStr{
					"wb_server":         server_name,
					"wb_metric_type":    "application_status",
					"app_server":        server_name,
					"app_name":          application,
					"app_componentName": application,
					"app_state":         appinfo["state"],
					"app_health":        appinfo["health"],
				},
			}
			wls.bt.client.Publish(application_status_event)
			logp.Info("Application status %s - event sent", server_name)
		}
	}
}

func (wls *Weblogic1212) ThreadStatusEvent() {
}

func (wls *Weblogic1212) SendErrorEvent(serverName string, metricType string, err string, body string) {
	error_event := beat.Event{
		Timestamp: time.Now(),
		Fields: common.MapStr{
			"err_server":       serverName,
			"err_metric_type":  metricType,
			"err_metric_error": err,
			"err_metric_body":  body,
		},
	}
	wls.bt.client.Publish(error_event)
	logp.Info("Error : %s", err)
}

func stringInSlice(a string, list []string) bool {
	for _, b := range list {
		if b == a {
			return true
		}
	}
	return false
}
