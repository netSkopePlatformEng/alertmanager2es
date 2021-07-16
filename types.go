package main

import "time"

type FlatAlert struct {
	Receiver          string            `json:"receiver"`
	Status            string            `json:"status"`
	Alert             Alert             `json:"alert"`
	GroupLabels       map[string]string `json:"groupLabels"`
	CommonLabels      map[string]string `json:"commonLabels"`
	CommonAnnotations map[string]string `json:"commonAnnotations"`
	ExternalURL       string            `json:"externalURL"`
	Version           string            `json:"version"`
	GroupKey          string            `json:"groupKey"`
}

type Alert struct {
	Status       string            `json:"status"`
	Labels       map[string]string `json:"labels"`
	Annotations  map[string]string `json:"annotations"`
	StartsAt     time.Time         `json:"startsAt"`
	EndsAt       time.Time         `json:"endsAt"`
	GeneratorURL string            `json:"generatorURL"`
	Fingerprint  string            `json:"fingerprint"`
}

/*
type Labels struct {
	Alertname string `json:"alertname"`
	Service   string `json:"service"`
	Severity  string `json:"severity"`
}

type Annotations struct {
	Summary string `json:"summary"`
}

type GroupLabels struct {
	Alertname string `json:"alertname"`
}

type CommonLabels struct {
	Alertname string `json:"alertname"`
	Service   string `json:"service"`
	Severity  string `json:"severity"`
}

type CommonAnnotations struct {
	Summary string `json:"summary"`
}
*/
