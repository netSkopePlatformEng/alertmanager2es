package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"github.com/graymeta/gmkit/backoff"
	"io/ioutil"
	"net/http"
	"strings"
	"time"

	elasticsearch "github.com/elastic/go-elasticsearch/v7"
	"github.com/elastic/go-elasticsearch/v7/esapi"
	"github.com/hashicorp/go-multierror"
	"github.com/pkg/errors"
	"github.com/prometheus/client_golang/prometheus"
	log "github.com/sirupsen/logrus"
)

const supportedWebhookVersion = "4"

type (
	AlertmanagerElasticsearchExporter struct {
		elasticSearchClient    *elasticsearch.Client
		elasticsearchIndexName string

		prometheus struct {
			alertsReceived   *prometheus.CounterVec
			alertsInvalid    *prometheus.CounterVec
			alertsSuccessful *prometheus.CounterVec
		}
	}

	AlertmanagerEntry struct {
		Alerts []struct {
			Annotations  map[string]string `json:"annotations"`
			EndsAt       time.Time         `json:"endsAt"`
			GeneratorURL string            `json:"generatorURL"`
			Labels       map[string]string `json:"labels"`
			StartsAt     time.Time         `json:"startsAt"`
			Status       string            `json:"status"`
		} `json:"alerts"`
		CommonAnnotations map[string]string `json:"commonAnnotations"`
		CommonLabels      map[string]string `json:"commonLabels"`
		ExternalURL       string            `json:"externalURL"`
		GroupLabels       map[string]string `json:"groupLabels"`
		Receiver          string            `json:"receiver"`
		Status            string            `json:"status"`
		Version           string            `json:"version"`
		GroupKey          string            `json:"groupKey"`

		// Timestamp records when the alert notification was received
		Timestamp string `json:"@timestamp"`
	}
)

func (e *AlertmanagerElasticsearchExporter) Init() {
	e.prometheus.alertsReceived = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "alertmanager2es_alerts_received",
			Help: "alertmanager2es received alerts",
		},
		[]string{},
	)
	prometheus.MustRegister(e.prometheus.alertsReceived)

	e.prometheus.alertsInvalid = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "alertmanager2es_alerts_invalid",
			Help: "alertmanager2es invalid alerts",
		},
		[]string{},
	)
	prometheus.MustRegister(e.prometheus.alertsInvalid)

	e.prometheus.alertsSuccessful = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "alertmanager2es_alerts_successful",
			Help: "alertmanager2es successful stored alerts",
		},
		[]string{},
	)
	prometheus.MustRegister(e.prometheus.alertsSuccessful)
}

func (e *AlertmanagerElasticsearchExporter) ConnectElasticsearch(cfg elasticsearch.Config, indexName string) {
	var err error
	e.elasticSearchClient, err = elasticsearch.NewClient(cfg)
	if err != nil {
		panic(err)
	}

	tries := 0
	for {
		_, err = e.elasticSearchClient.Info()
		if err != nil {
			tries++
			if tries >= 5 {
				panic(err)
			} else {
				log.Info("Failed to connect to ES, retry...")
				time.Sleep(5 * time.Second)
				continue
			}
		}

		break
	}

	e.elasticsearchIndexName = indexName
}

func (e *AlertmanagerElasticsearchExporter) buildIndexName(createTime time.Time) string {
	ret := e.elasticsearchIndexName

	ret = strings.Replace(ret, "%y", createTime.Format("2006"), -1)
	ret = strings.Replace(ret, "%m", createTime.Format("01"), -1)
	ret = strings.Replace(ret, "%d", createTime.Format("02"), -1)

	return ret
}

func (e *AlertmanagerElasticsearchExporter) HttpHandler(w http.ResponseWriter, r *http.Request) {
	e.prometheus.alertsReceived.WithLabelValues().Inc()

	if r.Body == nil {
		e.prometheus.alertsInvalid.WithLabelValues().Inc()
		err := errors.New("got empty request body")
		http.Error(w, err.Error(), http.StatusBadRequest)
		log.Error(errors.Wrap(err, "nil body"))
		return
	}

	b, err := ioutil.ReadAll(r.Body)
	if err != nil {
		e.prometheus.alertsInvalid.WithLabelValues().Inc()
		http.Error(w, err.Error(), http.StatusInternalServerError)
		log.Error(errors.Wrap(err, "read body"))
		return
	}
	defer r.Body.Close()

	var msg AlertmanagerEntry
	err = json.Unmarshal(b, &msg)
	if err != nil {
		e.prometheus.alertsInvalid.WithLabelValues().Inc()
		http.Error(w, err.Error(), http.StatusBadRequest)
		log.Error(errors.Wrap(err, "unmarshal body"))
		return
	}

	if msg.Version != supportedWebhookVersion {
		e.prometheus.alertsInvalid.WithLabelValues().Inc()
		err := fmt.Errorf("do not understand webhook version %q, only version %q is supported", msg.Version, supportedWebhookVersion)
		http.Error(w, err.Error(), http.StatusBadRequest)
		log.Error(err)
		return
	}

	now := time.Now()
	msg.Timestamp = now.Format(time.RFC3339)
	messages := msg.copy()

	back := backoff.New(
		backoff.InitBackoff(200*time.Millisecond),
		backoff.MaxBackoff(1*time.Minute),
		backoff.MaxCalls(4),
	)

	var merr error
	for _, myMsg := range messages {
		func() {
			myMsg.Timestamp = now
			b, _ := json.MarshalIndent(myMsg, "", "  ")
			log.Debugf(string(b))

			incidentJson, err := json.Marshal(myMsg)
			if err != nil {
				e.prometheus.alertsInvalid.WithLabelValues().Inc()
				merr = multierror.Append(merr, err)
				log.Error(errors.Wrapf(err, "marshal error"))
				return
			}

			req := esapi.IndexRequest{
				Index: e.buildIndexName(now),
				Body:  bytes.NewReader(incidentJson),
			}

			var res *esapi.Response
			err = back.Backoff(func() error {
				res, err = req.Do(context.Background(), e.elasticSearchClient)
				return err
			})
			if err != nil {
				e.prometheus.alertsInvalid.WithLabelValues().Inc()
				merr = multierror.Append(merr, err)
				log.Error(errors.Wrapf(err, "unable to insert document in elasticsearch"))
				return
			}
			defer res.Body.Close()
			log.Debugf("Elastic: %v", res)

			log.Debugf("received and stored alert: %v", msg.CommonLabels)
			e.prometheus.alertsSuccessful.WithLabelValues().Inc()
		}()
	}

	if merr != nil {
		http.Error(w, merr.Error(), http.StatusBadRequest)
		return
	}
}

func (e *AlertmanagerEntry) copy() []FlatAlert {
	var alertList []FlatAlert
	for _, alertEntry := range e.Alerts {
		var tempAlert FlatAlert
		tempAlert.Version = e.Version
		tempAlert.GroupLabels = e.GroupLabels
		tempAlert.Alert = Alert{
			Status:       alertEntry.Status,
			Labels:       alertEntry.Labels,
			Annotations:  alertEntry.Annotations,
			StartsAt:     alertEntry.StartsAt,
			EndsAt:       alertEntry.EndsAt,
			GeneratorURL: alertEntry.GeneratorURL,
		}
		tempAlert.Status = e.Status
		tempAlert.CommonLabels = e.CommonLabels
		tempAlert.CommonAnnotations = e.CommonAnnotations
		tempAlert.Receiver = e.Receiver
		tempAlert.ExternalURL = e.ExternalURL
		tempAlert.GroupKey = e.GroupKey
		alertList = append(alertList, tempAlert)
	}
	return alertList
}
