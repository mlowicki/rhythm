package main

import (
	"crypto/tls"
	"net/http"

	"github.com/evalphobia/logrus_sentry"
	"github.com/getsentry/raven-go"
	"github.com/mlowicki/rhythm/conf"
	tlsutils "github.com/mlowicki/rhythm/tls"
	"github.com/onrik/logrus/filename"
	log "github.com/sirupsen/logrus"
)

func init() {
	log.AddHook(filename.NewHook())
}

func initLogging(c *conf.Logging) {
	switch c.Level {
	case conf.LoggingLevelDebug:
		log.SetLevel(log.DebugLevel)
	case conf.LoggingLevelInfo:
		log.SetLevel(log.InfoLevel)
	case conf.LoggingLevelWarn:
		log.SetLevel(log.WarnLevel)
	case conf.LoggingLevelError:
		log.SetLevel(log.ErrorLevel)
	default:
		log.Fatalf("Unknown logging level: %s", c.Level)
	}
	switch c.Backend {
	case conf.LoggingBackendSentry:
		err := initSentryLogging(&c.Sentry)
		if err != nil {
			log.Fatalf("Error initializing Sentry logging: %s", err)
		}
	case conf.LoggingBackendNone:
	default:
		log.Fatalf("Unknown logging backend: %s", c.Backend)
	}
	log.Infof("Logging backend: %s", c.Backend)
}

func initSentryLogging(c *conf.LoggingSentry) error {
	cli, err := raven.NewWithTags(c.DSN, c.Tags)
	if err != nil {
		return err
	}
	if c.RootCA != "" {
		pool, err := tlsutils.BuildCertPool(c.RootCA)
		if err != nil {
			return err
		}
		cli.Transport = &raven.HTTPTransport{
			Client: &http.Client{
				Transport: &http.Transport{
					TLSClientConfig: &tls.Config{RootCAs: pool},
				},
			},
		}
	}
	hook, err := logrus_sentry.NewWithClientSentryHook(cli, []log.Level{
		log.PanicLevel,
		log.FatalLevel,
		log.ErrorLevel,
		log.WarnLevel,
	})
	if err != nil {
		return err
	}
	hook.Timeout = 0 // Do not wait for a reply.
	log.AddHook(hook)
	return nil
}
