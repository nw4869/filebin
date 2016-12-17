package model

import (
	"github.com/GeertJohan/go.rice"
	"github.com/nw4869/filebin/app/backend/fs"
	"github.com/nw4869/filebin/app/config"
	"github.com/nw4869/filebin/app/events"
	"github.com/nw4869/filebin/app/metrics"
	"log"
)

type Job struct {
	Bin      string
	Filename string
	Log      *log.Logger
	Cfg      *config.Configuration
}

type Context struct {
	TemplateBox *rice.Box
	StaticBox   *rice.Box
	Baseurl     string
	Log         *log.Logger
	WorkQueue   chan Job
	Backend     *fs.Backend
	Metrics     *metrics.Metrics
	Events      *events.Events
	RemoteAddr  string
}
