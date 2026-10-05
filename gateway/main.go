package main

import (
	"bufio"
	"context"
	"errors"
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"golang.org/x/crypto/bcrypt"
	"golang.org/x/term"
)

const bcryptCost = 12

func main() {
	log.SetFlags(log.LstdFlags | log.LUTC)

	if len(os.Args) > 1 && os.Args[1] == "hash-password" {
		if err := hashPassword(); err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
		return
	}

	configPath := flag.String("config", "/config/config.yaml", "설정 파일 경로")
	customDir := flag.String("custom", "/config/custom", "기본 웹 파일을 덮어쓸 디렉터리 (login.html 등)")
	listen := flag.String("listen", "", "설정 파일의 listen 값을 덮어씀 (컨테이너 안에서는 0.0.0.0:8086)")
	flag.Parse()

	cfg, err := LoadConfig(*configPath)
	if err != nil {
		log.Fatal(err)
	}
	if *listen != "" {
		cfg.Listen = *listen
	}
	srv := NewServer(cfg, *customDir)

	hup := make(chan os.Signal, 1)
	signal.Notify(hup, syscall.SIGHUP)
	go func() {
		for range hup {
			next, err := LoadConfig(*configPath)
			if err != nil {
				log.Printf("config reload failed, keeping previous config: %v", err)
				continue
			}
			srv.Reload(next)
			log.Printf("config reloaded")
		}
	}()

	go func() {
		for range time.Tick(10 * time.Minute) {
			srv.sessions.Sweep()
		}
	}()

	httpSrv := &http.Server{
		Addr:              cfg.Listen,
		Handler:           srv.Handler(),
		ReadHeaderTimeout: 10 * time.Second,
		ReadTimeout:       30 * time.Second,
		WriteTimeout:      30 * time.Second,
	}

	stop := make(chan os.Signal, 1)
	signal.Notify(stop, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		<-stop
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		httpSrv.Shutdown(ctx)
	}()

	log.Printf("nas-gateway listening on %s", cfg.Listen)
	if err := httpSrv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
		log.Fatal(err)
	}
}

func hashPassword() error {
	var pw []byte
	if term.IsTerminal(int(os.Stdin.Fd())) {
		fmt.Fprint(os.Stderr, "Password: ")
		p1, err := term.ReadPassword(int(os.Stdin.Fd()))
		fmt.Fprintln(os.Stderr)
		if err != nil {
			return err
		}
		fmt.Fprint(os.Stderr, "Again: ")
		p2, err := term.ReadPassword(int(os.Stdin.Fd()))
		fmt.Fprintln(os.Stderr)
		if err != nil {
			return err
		}
		if string(p1) != string(p2) {
			return errors.New("passwords do not match")
		}
		pw = p1
	} else {
		line, err := bufio.NewReader(os.Stdin).ReadString('\n')
		if err != nil && line == "" {
			return err
		}
		pw = []byte(strings.TrimRight(line, "\r\n"))
	}
	if len(pw) < 8 {
		return errors.New("password must be at least 8 characters")
	}
	if len(pw) > 72 {
		return errors.New("password must be at most 72 bytes (bcrypt limit)")
	}
	h, err := bcrypt.GenerateFromPassword(pw, bcryptCost)
	if err != nil {
		return err
	}
	fmt.Println(string(h))
	return nil
}
