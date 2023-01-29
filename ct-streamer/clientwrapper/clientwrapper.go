package clientwrapper

import (
	"context"
	"net/http"

	ct "github.com/google/certificate-transparency-go"
	"github.com/google/certificate-transparency-go/client"
	"github.com/google/certificate-transparency-go/jsonclient"
	"k8s.io/klog/v2"
)

type ClientWrapper struct {
	BatchSize    int
	LastTreeSize int64
	*client.LogClient
}

func NewClientWrapper(url string, httpClient *http.Client, j jsonclient.Options) (*ClientWrapper, error) {
	client, err := client.New(url, httpClient, j)
	if err != nil {
		return nil, err
	}
	wrappedClient := &ClientWrapper{
		LogClient:    client,
		BatchSize:    0,
		LastTreeSize: 0,
	}

	logs, err := wrappedClient.FetchLogs(context.Background(), 0, 512)
	if err != nil {
		return nil, err
	}
	wrappedClient.BatchSize = len(logs)

	res, err := wrappedClient.GetSTH(context.Background())
	if err != nil {
		return nil, err
	}
	wrappedClient.LastTreeSize = int64(res.TreeSize)

	return wrappedClient, nil
}

func (client *ClientWrapper) FetchLogs(ctx context.Context, start, end int64) ([]ct.LogEntry, error) {
	rawEntries, err := client.GetRawEntries(ctx, start, end)

	if err != nil {
		return nil, err
	}

	entries := make([]ct.LogEntry, len(rawEntries.Entries))
	for i, entry := range rawEntries.Entries {
		index := int64(i) + start
		rawLogEntry, err := ct.LogEntryFromLeaf(index, &entry)
		if err != nil {
			klog.Infof("[%s] error parsing: %e", client.BaseURI(), err)
		}
		entries[i] = *rawLogEntry
	}

	return entries, nil
}
