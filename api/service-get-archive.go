package api

import (
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"go.uber.org/zap"
)

func (s *Service) getArchive(board string) ([]int64, error) {
	defer s.logger.Sync()

	url := fmt.Sprintf("%s/%s/archive.json", s.host, board)
	req, err := http.NewRequest(http.MethodGet, url, nil)

	if err != nil {
		return nil, fmt.Errorf("Error constructing archive request: %s", err)
	}

	s.lastArchiveRequestMutex.Lock()
	lastArchiveRequest, lastArchiveRequestExists := s.lastArchiveRequest[board]

	if lastArchiveRequestExists {
		req.Header["If-Modified-Since"] = []string{
			fmt.Sprintf("%s GMT", lastArchiveRequest.Format("Mon, 02 Jan 2006 15:04:05")),
		}
	}

	s.lastArchiveRequest[board] = time.Now().UTC()
	s.lastArchiveRequestMutex.Unlock()

	for i := 0; ; i++ {
		resp, err := s.client.Do(req)

		if err != nil {
			if i < 3 {
				s.logger.Error("Error doing archive request", zap.String("board", board), zap.Error(err))
				time.Sleep(30 * time.Second)

				continue
			} else {
				return nil, fmt.Errorf("Error doing archive request: %s", err)
			}
		}

		//Archive hasn't been modified since the time specified in If-Modified-Since
		if resp.StatusCode == 304 {
			resp.Body.Close()
			return nil, nil
		}

		if resp.StatusCode == 404 {
			resp.Body.Close()
			s.logger.Fatal("Your configuration seems fucked, mark this board as not having an archive.", zap.String("board", board))
		}

		if resp.StatusCode != 200 {
			resp.Body.Close()

			if i < 3 {
				s.logger.Error("Bad archive request status code", zap.String("board", board), zap.String("status", resp.Status))
				time.Sleep(30 * time.Second)

				continue
			} else {
				return nil, fmt.Errorf("Bad archive request status code: %s", resp.Status)
			}
		}

		var archive []int64

		jsonDecoder := json.NewDecoder(resp.Body)
		err = jsonDecoder.Decode(&archive)
		resp.Body.Close()

		if err != nil {
			if i < 3 {
				s.logger.Error("Error unmarshalling archive response body", zap.String("board", board), zap.Error(err))
				time.Sleep(30 * time.Second)

				continue
			} else {
				return nil, fmt.Errorf("Error unmarshalling archive response body: %s", err)
			}
		}

		return archive, nil
	}
}
