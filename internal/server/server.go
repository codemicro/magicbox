package server

import (
	"bytes"
	"crypto/subtle"
	"errors"
	"fmt"
	"git.tdpain.net/codemicro/magicbox/internal/config"
	"github.com/allegro/bigcache"
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/credentials"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/s3"
	"github.com/eko/gocache/lib/v4/cache"
	bigcacheStore "github.com/eko/gocache/store/bigcache/v4"
	"log/slog"
	"net/http"
	"time"
)

func ListenAndServe() error {
	var (
		conf = config.Get()
		s    = new(server)
	)

	{ // Init cache
		cacheEngine, err := bigcache.NewBigCache(bigcache.Config{
			Shards:             1024,
			LifeWindow:         time.Hour * 24 * 30,
			CleanWindow:        time.Hour,
			MaxEntriesInWindow: 1000 * 10 * 60,
			MaxEntrySize:       500,
			Verbose:            true,
			HardMaxCacheSize:   conf.MaxCacheMB,
		})
		if err != nil {
			return fmt.Errorf("initialise bigcache: %w", err)
		}

		s.cache = cache.New[[]byte](bigcacheStore.NewBigcache(cacheEngine))
	}

	{ // Init S3 client
		awsConfig := &aws.Config{
			Credentials:      credentials.NewStaticCredentials(conf.S3CredentialID, conf.S3CredentialSecret, ""),
			Endpoint:         aws.String(conf.S3Endpoint),
			Region:           aws.String(conf.S3Region),
			S3ForcePathStyle: aws.Bool(conf.S3ForcePathStyle),
		}

		awsSession, err := session.NewSession(awsConfig)
		if err != nil {
			return fmt.Errorf("create S3 session: %w", err)
		}

		s.s3Client = s3.New(awsSession)
	}

	{ // Setup admin pages
		s.adminMux = http.NewServeMux()
		s.adminMux.Handle("GET /stats", handlerFuncWithError(s.cacheStatsHandler))
		s.adminMux.Handle("PUT /invalidate/{selector}", handlerFuncWithError(s.cacheInvalidationHandler))
	}

	// Let's do this B-)

	errChan := make(chan error)

	if conf.AdminEnabled {
		if conf.AdminToken == "" {
			slog.Warn("Admin HTTP server enabled without an admin token set")
		}
		slog.Info("Admin HTTP server alive", "address", conf.AdminHTTPAddress)

		go func() {
			err := http.ListenAndServe(conf.AdminHTTPAddress, http.HandlerFunc(func(rw http.ResponseWriter, req *http.Request) {
				if tok := conf.AdminToken; tok == "" ||
					subtle.ConstantTimeCompare([]byte("Bearer "+tok), []byte(req.Header.Get("Authorization"))) == 1 {
					s.adminMux.ServeHTTP(rw, req)
					return
				}
				rw.WriteHeader(http.StatusForbidden)
				return
			}))
			if err != nil {
				errChan <- err
			}
		}()
	}

	slog.Info("HTTP server alive", "address", conf.ServerHTTPAddress)
	go func() {
		err := http.ListenAndServe(conf.ServerHTTPAddress, handlerFuncWithError(s.rootHandler))
		if err != nil {
			errChan <- err
		}
	}()

	return <-errChan
}

func handlerFuncWithError(f func(http.ResponseWriter, *http.Request) error) http.HandlerFunc {
	return func(rw http.ResponseWriter, req *http.Request) {
		if err := f(rw, req); err != nil {
			slog.Error("unhandled handler error", "error", err, "request", req)
			rw.Header().Set("Content-Type", "text/plain")
			rw.WriteHeader(http.StatusInternalServerError)
		}
	}
}

type server struct {
	cache    *cache.Cache[[]byte]
	s3Client *s3.S3
	adminMux *http.ServeMux

	// Eventual consistency?? That's what this being unguarded by a mutex is right???
	hitMissCounter struct {
		Hits   uint64
		Misses uint64
	}
}

func (s *server) incHits() {
	s.hitMissCounter.Hits += 1
}

func (s *server) incMisses() {
	s.hitMissCounter.Misses += 1
}

type cachedFile struct {
	Body        []byte
	ContentType string
}

func (c *cachedFile) Bytes() []byte {
	return append(append([]byte(c.ContentType), byte('\000')), c.Body...)
}

func (c *cachedFile) FromBytes(x []byte) error {
	firstNullByte := bytes.IndexRune(x, '\000')
	if firstNullByte == -1 {
		return errors.New("badly formatted cachedFile")
	}
	c.ContentType = string(x[:firstNullByte])
	if len(x) > firstNullByte {
		c.Body = x[firstNullByte+1:]
	}
	return nil
}
