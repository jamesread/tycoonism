package main

import (
	"context"
	"flag"
	"fmt"
	"net"
	"net/http"
	"os"
	"os/signal"
	"path"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	"github.com/jamesread/httpauthshim"
	"github.com/jamesread/httpauthshim/providers/haslocal"
	"github.com/jamesread/httpauthshim/providers/hasoauth2"
	"github.com/jamesread/httpauthshim/sessions"
	"github.com/knadh/koanf/v2"
	"github.com/knadh/koanf/parsers/yaml"
	"github.com/knadh/koanf/providers/file"
	"github.com/sirupsen/logrus"
	"webfarm/internal/config"
	"webfarm/internal/server"
	"webfarm/internal/store"
)

func main() {
	hashPassword := flag.String("hashpassword", "", "hash a password for auth.localUsers (output is Argon2id hash for config)")
	flag.Parse()
	if *hashPassword != "" {
		hash, err := haslocal.CreateHash(*hashPassword)
		if err != nil {
			fmt.Fprintf(os.Stderr, "hashpassword: %v\n", err)
			os.Exit(1)
		}
		fmt.Println(hash)
		os.Exit(0)
	}

	log := logrus.New()
	log.SetLevel(logrus.InfoLevel)

	k := koanf.New(".")
	var configPath string
	if dir, err := os.UserHomeDir(); err == nil {
		configPath = filepath.Join(dir, ".config", "wf", "config.yaml")
	}
	configFileLoaded := false
	if configPath != "" {
		if _, err := os.Stat(configPath); err != nil {
			log.WithField("config", configPath).Info("config file not found, using defaults")
		} else {
			if err := k.Load(file.Provider(configPath), yaml.Parser()); err != nil {
				log.WithError(err).WithField("config", configPath).Warn("loading config file failed")
			} else {
				configFileLoaded = true
				log.WithField("config", configPath).Info("config file loaded")
			}
		}
	}
	cfg := config.Load(k)
	if configFileLoaded {
		config.LogLoadSummary(log, k, cfg)
	}
	if err := cfg.LoadGameData(); err != nil {
		log.WithError(err).Fatal("loading game data")
	}

	str := store.NewStore()
	if ps, err := store.LoadFromFile(cfg.StateFile); err != nil {
		log.WithError(err).Warn("loading state file, starting fresh")
	} else if ps != nil {
		str.Restore(ps)
		now := time.Now()
		n := str.RunCatchUpTicks(now, cfg)
		if n > 0 {
			log.WithField("missed_ticks", n).Info("caught up with missed ticks")
	} else {
		str.RunTick(now, cfg)
	}
	}
	var authCtx *auth.AuthShimContext
	var oauth2Handler *hasoauth2.OAuth2Handler
	if cfg.Auth != nil {
		sessionStorage := sessions.NewSessionStorage(sessions.NewYAMLPersistence())
		var err error
		authCtx, err = auth.NewAuthShimContext(cfg.Auth, sessionStorage)
		if err != nil {
			log.WithError(err).Fatal("initializing auth")
		}
		defer func() {
			if err := authCtx.Shutdown(); err != nil {
				log.WithError(err).Warn("auth shutdown")
			}
		}()
		if cfg.Auth.LocalUsers.Enabled {
			authCtx.AddProvider(haslocal.CheckUserFromLocalSession)
		}
		if len(cfg.Auth.OAuth2Providers) > 0 {
			oauth2Handler = hasoauth2.NewOAuth2Handler(cfg.Auth, authCtx.Sessions)
			authCtx.AddProvider(oauth2Handler.CheckUserFromOAuth2Cookie)
		}
	}
	var loginHandler http.Handler
	if authCtx != nil && cfg.Auth != nil && cfg.Auth.LocalUsers.Enabled {
		loginHandler = server.NewLocalLoginHandler(authCtx)
	}
	apiHandler := server.NewMux(str, cfg, log, authCtx, oauth2Handler, loginHandler)
	handler := apiHandler
	staticDir := cfg.StaticDir
	if staticDir == "" {
		for _, try := range []string{"frontend/dist", "dist"} {
			if _, err := os.Stat(try); err == nil {
				staticDir = try
				break
			}
		}
	}
	if staticDir != "" {
		absDir, err := filepath.Abs(staticDir)
		if err != nil {
			log.WithError(err).WithField("static_dir", staticDir).Fatal("resolving static dir")
		}
		if _, err := os.Stat(absDir); err != nil {
			log.WithError(err).WithField("static_dir", absDir).Fatal("static dir not found")
		}
		handler = staticWithFallback(absDir, apiHandler)
		log.WithField("static_dir", absDir).Info("serving frontend from static dir")
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go runSimulation(ctx, str, cfg, log)
	go runStateBackup(ctx, str, cfg, log)

	srv := &http.Server{
		Addr:    cfg.ListenAddr,
		Handler: handler,
	}
	go func() {
		_, port, _ := net.SplitHostPort(cfg.ListenAddr)
		if port == "" {
			port = cfg.ListenAddr
		}
		log.WithField("addr", cfg.ListenAddr).WithField("port", port).Info("listening")
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.WithError(err).Fatal("server exited")
		}
	}()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit
	log.Info("shutting down")
	cancel()
	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer shutdownCancel()
	if err := srv.Shutdown(shutdownCtx); err != nil {
		log.WithError(err).Warn("shutdown error")
	}
	if err := str.SaveToFile(cfg.StateFile); err != nil {
		log.WithError(err).Warn("saving state on shutdown")
	}
}

func staticWithFallback(root string, api http.Handler) http.Handler {
	fs := http.Dir(root)
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		p := path.Clean(r.URL.Path)
		if p == "/oauth" || strings.HasPrefix(p, "/oauth/") || p == "/login" || strings.HasPrefix(p, "/game.v1") {
			api.ServeHTTP(w, r)
			return
		}
		name := p
		if name == "" || name == "/" {
			name = "index.html"
		} else {
			name = strings.TrimPrefix(name, "/")
		}
		f, err := fs.Open(name)
		if err != nil {
			if os.IsNotExist(err) && !strings.Contains(name, ".") {
				f, err = fs.Open("index.html")
				if err == nil {
					name = "index.html"
				}
			}
			if err != nil {
				if os.IsNotExist(err) {
					http.NotFound(w, r)
					return
				}
				http.Error(w, err.Error(), http.StatusInternalServerError)
				return
			}
		}
		defer f.Close()
		stat, _ := f.Stat()
		if stat != nil && stat.IsDir() {
			http.NotFound(w, r)
			return
		}
		http.ServeContent(w, r, path.Base(name), stat.ModTime(), f)
	})
}

func runSimulation(ctx context.Context, str *store.Store, cfg *config.Config, log *logrus.Logger) {
	for {
		select {
		case <-ctx.Done():
			return
		default:
		}
		_, _, nextTickAt := str.World().State()
		if time.Now().Before(nextTickAt) {
			select {
			case <-ctx.Done():
				return
			case <-time.After(time.Until(nextTickAt)):
			}
		}
		now := time.Now()
		str.RunTick(now, cfg)
		server.RunNPCTick(str, cfg)
	}
}

func runStateBackup(ctx context.Context, str *store.Store, cfg *config.Config, log *logrus.Logger) {
	ticker := time.NewTicker(cfg.StateBackupInterval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			if err := str.SaveToFile(cfg.StateFile); err != nil {
				log.WithError(err).Warn("state backup failed")
			}
		}
	}
}
