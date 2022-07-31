package api

import (
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"go.uber.org/zap"
)

func (s *Service) getCatalog(board string) ([]CatalogPage, error) {

	url := fmt.Sprintf("%s/%s/catalog.json", s.host, board)
	req, err := http.NewRequest(http.MethodGet, url, nil)

	if err != nil {
		return nil, fmt.Errorf("Error constructing catalog request: %s", err)
	}

	s.lastCatalogRequestMutex.Lock()
	lastCatalogRequest, lastCatalogRequestExists := s.lastCatalogRequest[board]

	if lastCatalogRequestExists {
		req.Header["If-Modified-Since"] = []string{
			fmt.Sprintf("%s GMT", lastCatalogRequest.Format("Mon, 02 Jan 2006 15:04:05")),
		}
	}

	s.lastCatalogRequest[board] = time.Now().UTC()
	s.lastCatalogRequestMutex.Unlock()

	for i := 0; ; i++ {
		resp, err := s.client.Do(req)

		if err != nil {
			if i < 3 {
				s.logger.Error("Error doing catalog request", zap.String("board", board), zap.Error(err))
				time.Sleep(30 * time.Second)

				continue
			} else {
				return nil, fmt.Errorf("Error doing catalog request: %s", err)
			}
		}

		//Archive hasn't been modified since the time specified in If-Modified-Since
		if resp.StatusCode == 304 {
			resp.Body.Close()
			return nil, nil
		}

		if resp.StatusCode != 200 {
			resp.Body.Close()

			if i < 3 {
				s.logger.Error("Error doing catalog request", zap.String("board", board), zap.String("status", resp.Status))
				time.Sleep(30 * time.Second)

				continue
			} else {
				return nil, fmt.Errorf("Error doing catalog request: Status code is %s", resp.Status)
			}
		}

		catalogPages := make([]CatalogPage, 0, 150)

		jsonDecoder := json.NewDecoder(resp.Body)
		err = jsonDecoder.Decode(&catalogPages)
		resp.Body.Close()

		if err != nil {
			if i < 3 {
				s.logger.Error("Error unmarshalling catalog", zap.String("board", board), zap.Error(err))
				time.Sleep(30 * time.Second)

				continue
			} else {
				return nil, fmt.Errorf("Error unmarshalling archive response body: %s", err)
			}
		}

		return catalogPages, nil
	}
}
