package services

import (
	"context"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"sync"
	"time"

	"github.com/fsnotify/fsnotify"
	"github.com/pkg/errors"
	log "github.com/sirupsen/logrus"
	"github.com/urfave/cli"
)

type Waiter struct {
	path   string
	re     *regexp.Regexp
	locks  sync.Map
	doneCh chan error
	w      *fsnotify.Watcher
	closed bool
}

func NewWaiter(c *cli.Context, re *regexp.Regexp) *Waiter {
	return &Waiter{
		path:   c.String(OutputFlag),
		re:     re,
		doneCh: make(chan error),
	}
}

func (s *Waiter) Serve() error {
	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		return errors.Wrap(err, "Failed to init watcher")
	}
	s.w = watcher
	go func() {
		for {
			select {
			case event, ok := <-watcher.Events:
				if !ok {
					return
				}
				name := filepath.Base(event.Name)
				if s.re.MatchString(name) {
					// log.WithField("name", name).WithField("op", event.Op).Info("Got watcher event")
					l, ok := s.locks.Load(name)
					if ok {
						go func() {
							<-time.After(500 * time.Millisecond)
							log.WithField("name", name).Info("Release lock")
							l.(*AccessLock).Unlock()
						}()
					}
				}
			case err, ok := <-watcher.Errors:
				if !ok {
					return
				}
				if err != nil {
					log.WithError(err).Error("Got watcher error")
				}
			}
		}
	}()
	err = watcher.Add(s.path)
	if err != nil {
		return errors.Wrap(err, "Failed add path for watcher")
	}
	log.Info("Starting Waiter")
	return <-s.doneCh
}

func (s *Waiter) Wait(ctx context.Context, path string) chan error {
	errCh := make(chan error)
	go func() {
		if !s.re.MatchString(path) || s.closed {
			errCh <- nil
		} else if _, err := os.Stat(s.path + path); os.IsNotExist(err) {
			log.WithField("name", s.path+path).Info("Add request lock")
			al, _ := s.locks.LoadOrStore(filepath.Base(path), NewAccessLock())
			select {
			case <-s.doneCh:
			case <-al.(*AccessLock).Unlocked():
				errCh <- nil
				break
			case <-ctx.Done():
				errCh <- ctx.Err()
				break
			}
		} else {
			errCh <- nil
		}
	}()
	return errCh
}

func (s *Waiter) Close() {
	if s.closed {
		return
	}
	s.closed = true
	close(s.doneCh)
	if s.w != nil {
		s.w.Close()
	}
}

func (s *Waiter) Handle(h Handleable) {
	h.Handle(func(h http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			ctx := r.Context()
			select {
			case <-s.Wait(r.Context(), r.URL.Path):
			case <-ctx.Done():
				if ctx.Err() != nil {
					w.WriteHeader(http.StatusInternalServerError)
					return
				}
				break
			}
			h.ServeHTTP(w, r)
		})
	})
}
