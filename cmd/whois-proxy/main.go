package main

import (
	"context"
	"flag"
	"fmt"
	"io"
	"os"
	"os/signal"
	"syscall"

	"github.com/k0kubun/pp"
	"github.com/pkg/errors"
	graylog "github.com/shynie/logrus-graylog-hook/v3"
	"github.com/sirupsen/logrus"

	"gitlab.esta.spb.ru/arseny/whois-proxy/internal/config"
	whois_server "gitlab.esta.spb.ru/arseny/whois-proxy/internal/whois"
)

const (
	platformName       = "whois-proxy"
	configPathTemplate = "%s/config.yml"
)

// Общие переменные для удобства (чтоб не пробрасывать из функции в функции по указателям) логгер и конфигурация сервиса
var (
	cfg    *config.Config
	logger *logrus.Logger
)

func main() {
	// Инициализация cfg и logger
	if err := initService(); err != nil {
		logrus.WithError(err).Fatal("service init fail")
	}

	// Непосредственно старт сервиса
	if err := launchService(); err != nil {
		logrus.WithError(err).Fatal("service failed with error")
	}

	logger.Info("Finished Main")
}

func initService() error {
	var err error
	cfg, err = loadConfig()
	if err != nil {
		return errors.WithMessage(err, "cannot load configuration file")
	}

	_, _ = pp.Println(cfg)

	logger, err = newLogger(cfg.Graylog)
	if err != nil {
		return errors.WithMessage(err, "cannot init new logger")
	}

	logger.Info("Successfully Init: cfg and logger")

	return err
}

func loadConfig() (*config.Config, error) {
	wd, err := os.Getwd()
	if err != nil {
		return nil, errors.WithMessage(err, "failed to get working directory")
	}

	var pathToCfg string
	flag.StringVar(&pathToCfg, "config", fmt.Sprintf(configPathTemplate, wd), "Path to config file")
	flag.Parse()

	cfg, err := config.Load(pathToCfg)
	if err != nil {
		return nil, errors.WithMessage(err, "failed to load config file")
	}

	return &cfg, nil
}

func newLogger(cfg config.Graylog) (*logrus.Logger, error) {
	logger := logrus.StandardLogger()
	logger.SetReportCaller(true)
	logrus.SetFormatter(&logrus.TextFormatter{DisableColors: cfg.DisableColor})

	platform := cfg.Platform
	if platform == "" {
		platform = platformName
	}

	graylogAddr := fmt.Sprintf("%s:%d", cfg.Host, cfg.Port)
	graylogHook := graylog.NewGraylogHook(graylogAddr, map[string]interface{}{"platform": platform})
	graylogHook.Level = logrus.InfoLevel

	if cfg.EnableFileLog { // Включить логирование в файл
		nameLogFile := cfg.NameLogFile
		if nameLogFile == "" {
			nameLogFile = fmt.Sprintf("%s.log", platformName)
		}

		file, err := os.OpenFile(nameLogFile, os.O_WRONLY|os.O_APPEND|os.O_CREATE, 0644)
		if err != nil {
			logrus.WithField("graylogAddr", graylogAddr).Errorf("failed create log file %s", nameLogFile)
			return nil, fmt.Errorf("failed create log file %s", nameLogFile)
		}
		logger.Out = io.MultiWriter(file, os.Stdout)
	}

	logger.Info("Setting up graylog hook")
	if graylogHook.Writer() == nil {
		logrus.WithField("graylogAddr", graylogAddr).Error("failed setting up graylog hook")
		return nil, fmt.Errorf("failed setting up graylog hook")
	}
	logger.Hooks.Add(graylogHook)
	logrus.WithField("graylogAddr", graylogAddr).Info("done setting up graylog hook")

	if cfg.DebugLvl { // Отладочный режим - вывод Debug логов
		logger.SetLevel(logrus.DebugLevel)
	}

	return logger, nil
}

func launchService() error {
	logger.Info("Starting service")

	// start TCP whois proxy server
	whois, err := whois_server.NewWhoisProxyServer(&cfg.Service, logger)
	if err != nil {
		return errors.WithMessagef(err, "can't create new Whois Proxy Server")
	}

	err = whois.Start()
	if err != nil {
		return errors.WithMessage(err, "can't start whois server")
	}

	// Ниже обработка сигналов прерывания процесса (SIGINT)
	ctx, cancel := context.WithCancel(context.Background())
	go func() {
		handleSIGINT()
		cancel()
	}()

	<-ctx.Done()

	return nil
}

func handleSIGINT() {
	ch := make(chan os.Signal, 10)
	signal.Notify(ch, os.Interrupt, syscall.SIGTERM)

	for range ch {
		signal.Stop(ch)
		return
	}
}
