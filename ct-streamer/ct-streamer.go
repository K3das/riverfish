package ctstreamer

import (
	"context"
	"crypto/sha256"
	"net/http"
	_ "net/http/pprof"
	"sync"
	"time"

	"github.com/K3das/ct-streamer/clientwrapper"
	"github.com/K3das/ct-streamer/directory"
	ct "github.com/google/certificate-transparency-go"
	"github.com/google/certificate-transparency-go/jsonclient"
	"github.com/google/certificate-transparency-go/scanner"
	"github.com/google/certificate-transparency-go/x509"
	"github.com/jellydator/ttlcache/v3"
	"golang.org/x/sync/errgroup"
	"k8s.io/klog/v2"
)

type ResultCertificate struct {
	Index int64
	x509.Certificate
}

type ResultLogs struct {
	Certificates []ResultCertificate
	LogURL       string
}

type CTStreamer struct {
	Logs       []directory.Log
	httpClient *http.Client

	// Run
	callback (func(ResultLogs) error)
	g        errgroup.Group

	// Existing cache
	existingCache *ttlcache.Cache[[32]byte, int]

	// Stop!
	cancel context.CancelFunc
	mu     sync.Mutex
}

func (s *CTStreamer) worker(ctx context.Context, logUrl string) error {
	client, err := clientwrapper.NewClientWrapper(logUrl, s.httpClient, jsonclient.Options{})
	if err != nil {
		return err
	}

	fetcher := scanner.NewFetcher(client, &scanner.FetcherOptions{
		BatchSize:     client.BatchSize,
		ParallelFetch: 1,
		StartIndex:    client.LastTreeSize,
		EndIndex:      0,
		Continuous:    true,
	})
	return fetcher.Run(ctx, func(eb scanner.EntryBatch) {
		filtered := []ResultCertificate{}
		for i, le := range eb.Entries {
			index := eb.Start + int64(i)
			rawLogEntry, err := ct.RawLogEntryFromLeaf(index, &le)
			if err != nil {
				klog.Errorf("[%s] failed to build raw log entry (%d): %s", client.BaseURI(), index, err.Error())
			}

			logEntry, err := rawLogEntry.ToLogEntry()
			if err != nil {
				klog.Errorf("[%s] failed to build log entry (%d): %s", client.BaseURI(), index, err.Error())
			}

			if logEntry.X509Cert == nil {
				continue
			}

			hash := sha256.Sum256(logEntry.X509Cert.Raw)

			item := s.existingCache.Get(hash)
			if item == nil {
				s.existingCache.Set(hash, 1, ttlcache.DefaultTTL)
			} else {
				s.existingCache.Set(hash, item.Value()+1, ttlcache.DefaultTTL)
				continue
			}
			filtered = append(filtered, ResultCertificate{
				eb.Start + int64(i),
				*logEntry.X509Cert,
			})
		}
		s.g.Go(func() error {
			return s.callback(ResultLogs{
				Certificates: filtered,
				LogURL:       logUrl,
			})
		})
	})
}

// Create a new CT Streamer with an HTTP client
func NewStreamer(httpClient *http.Client) (*CTStreamer, error) {
	var err error
	streamer := &CTStreamer{}

	streamer.Logs, err = directory.GetCurrentLogs(httpClient)
	if err != nil {
		return nil, err
	}
	streamer.httpClient = httpClient

	streamer.cancel = func() {}
	streamer.callback = func(_ ResultLogs) error { return nil }

	streamer.existingCache = ttlcache.New(
		ttlcache.WithTTL[[32]byte, int](time.Minute),
	)

	return streamer, nil
}

// Run the CT Streamer
//
// (Blocking)
func (s *CTStreamer) Run(cctx context.Context, callback func(ResultLogs) error) error {
	ctx, cancel := context.WithCancel(cctx)
	defer cancel()

	s.mu.Lock()
	s.cancel = cancel
	s.mu.Unlock()

	go func() {
		for {
			select {
			case <-time.After(time.Minute):
			case <-ctx.Done():
				s.existingCache.DeleteAll()
				return
			}
			klog.Info("Clearing Cache")
			s.existingCache.DeleteExpired()
		}
	}()

	// defer close(s.output)
	for _, ctLog := range s.Logs {
		s.g.Go(func() error {
			return s.worker(ctx, ctLog.URL)
		})
		time.Sleep(50 * time.Millisecond)
	}

	s.callback = callback

	return s.g.Wait()
}

func (s *CTStreamer) Cancel() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.cancel()
}

func main() {
	go func() {
		klog.Error(http.ListenAndServe(":3621", nil))
	}()

	httpClient := http.Client{
		Timeout: time.Second * 5,
	}

	streamer, err := NewStreamer(&httpClient)
	if err != nil {
		klog.Fatal(err)
	}

	count := 0

	go func() {
		for {
			time.Sleep(time.Second)
			klog.Infof("Seen %d certificates", count)
		}
	}()
	if err := streamer.Run(context.Background(), func(l ResultLogs) error {
		count += len(l.Certificates)
		return nil
	}); err != nil {
		klog.Fatal(err)
	}
}
