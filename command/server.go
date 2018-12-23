package command

import (
	"crypto/tls"
	"flag"
	"net/http"
	"strings"
	"time"

	"github.com/evalphobia/logrus_sentry"
	"github.com/getsentry/raven-go"
	"github.com/mlowicki/rhythm/api"
	"github.com/mlowicki/rhythm/conf"
	"github.com/mlowicki/rhythm/coordinator"
	"github.com/mlowicki/rhythm/mesos"
	"github.com/mlowicki/rhythm/secrets"
	"github.com/mlowicki/rhythm/storage"
	tlsutils "github.com/mlowicki/rhythm/tls"
	"github.com/prometheus/client_golang/prometheus"
	log "github.com/sirupsen/logrus"
	"github.com/tevino/abool"
)

type ServerCommand struct {
	*BaseCommand
	Version     string
	confPath    string
	testLogging bool
}

func (c *ServerCommand) Run(args []string) int {
	var infoGauge = prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Name: "rhythm_info",
		Help: "Information about rhythm instance.",
	}, []string{"version"})
	prometheus.MustRegister(infoGauge)
	infoGauge.WithLabelValues(c.Version).Set(1)

	fs := c.Flags()
	fs.Parse(args)
	conf, err := conf.New(c.confPath)
	if err != nil {
		log.Fatalf("Error getting configuration: %s", err)
	}
	initLogging(&conf.Logging)
	if c.testLogging {
		log.Error("test")
		log.Info("Sending test event. Wait 10s...")
		<-time.After(10 * time.Second)
		return 0
	}
	leader := abool.New()
	var leaderGauge = prometheus.NewGaugeFunc(prometheus.GaugeOpts{
		Name: "leader",
		Help: "Indicates if instance is elected as leader.",
	}, func() float64 {
		if leader.IsSet() {
			return 1
		}
		return 0
	})
	prometheus.MustRegister(leaderGauge)
	stor := storage.New(&conf.Storage)
	coord := coordinator.New(&conf.Coordinator)
	api.New(&conf.API, stor, api.State{func() bool { return leader.IsSet() }, c.Version})
	secr := secrets.New(&conf.Secrets)
	for {
		log.Info("Waiting until Mesos scheduler leader")
		ctx := coord.WaitUntilLeader()
		leader.Set()
		err = mesos.Run(ctx, conf, stor, secr)
		leader.UnSet()
		if err != nil {
			log.Errorf("Controller error: %s", err)
			<-time.After(time.Second)
		}
	}
}

func (c *ServerCommand) Help() string {
	help := `
Usage: rhythm server [options]

  This command starts a Rhythm server.

  Start a server with configuration file:

      $ rhythm server -config=/etc/rhythm/config.json

` + c.Flags().help()
	return strings.TrimSpace(help)
}

func (c *ServerCommand) Flags() *flagSet {
	fs := flag.NewFlagSet("server", flag.ContinueOnError)
	fs.Usage = func() { c.Printf(c.Help()) }
	fs.StringVar(&c.confPath, "config", "config.json", "Path to configuration file")
	fs.BoolVar(&c.testLogging, "testlogging", false, "log sample error and exit")
	return &flagSet{fs}
}

func (c *ServerCommand) Synopsis() string {
	return "Start a Rhythm server"
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
	if c.CACert != "" {
		pool, err := tlsutils.BuildCertPool(c.CACert)
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
