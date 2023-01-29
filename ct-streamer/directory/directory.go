package directory

import (
	"encoding/json"
	"net/http"
	"time"
)

const DirectoryUrl = "https://www.gstatic.com/ct/log_list/v3/log_list.json"

type Log struct {
	Description      string                 `json:"description"`
	LogID            string                 `json:"log_id"`
	Key              string                 `json:"key"`
	URL              string                 `json:"url"`
	Mmd              int                    `json:"mmd"`
	State            map[string]interface{} `json:"state,omitempty"`
	TemporalInterval struct {
		StartInclusive time.Time `json:"start_inclusive"`
		EndExclusive   time.Time `json:"end_exclusive"`
	} `json:"temporal_interval,omitempty"`
	LogType string `json:"log_type,omitempty"`
}
type LogDirectoryResponse struct {
	IsAllLogs        bool      `json:"is_all_logs"`
	Version          string    `json:"version"`
	LogListTimestamp time.Time `json:"log_list_timestamp"`
	Operators        []struct {
		Name  string   `json:"name"`
		Email []string `json:"email"`
		Logs  []Log    `json:"logs"`
	} `json:"operators"`
}

func GetCurrentLogs(httpClient *http.Client) ([]Log, error) {
	res, err := httpClient.Get(DirectoryUrl)
	if err != nil {
		return nil, err
	}
	defer res.Body.Close()

	directoryResponse := new(LogDirectoryResponse)
	err = json.NewDecoder(res.Body).Decode(directoryResponse)
	if err != nil {
		return nil, err
	}

	var filteredLogs []Log
	for _, o := range directoryResponse.Operators {
		for _, l := range o.Logs {
			if l.LogType == "test" {
				continue
			}

			x := false
			for k := range l.State {
				x = k == "usable"
				break
			}
			if !x {
				continue
			}

			filteredLogs = append(filteredLogs, l)
		}
	}

	return filteredLogs, nil
}
