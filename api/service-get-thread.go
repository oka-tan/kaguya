package api

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"time"

	"go.uber.org/zap"
)

func (s *Service) getThread(board string, threadNumber int64) ([]Post, error) {
	url := fmt.Sprintf("%s/%s/thread/%d.json", s.host, board, threadNumber)
	req, err := http.NewRequest(http.MethodGet, url, nil)

	if err != nil {
		return nil, fmt.Errorf("Error constructing thread request: %s", err)
	}

	for i := 0; ; i++ {
		resp, err := s.client.Do(req)

		if err != nil {
			if i < 3 {
				s.logger.Error("Error doing thread request", zap.Int64("thread-number", threadNumber), zap.String("board", board), zap.Error(err))
				time.Sleep(30 * time.Second)

				continue
			} else {
				return nil, fmt.Errorf("Error performing thread request: %s", err)
			}
		}

		if resp.StatusCode == 404 {
			resp.Body.Close()
			return nil, errors.New("Thread request returned 404")
		}

		if resp.StatusCode != 200 {
			resp.Body.Close()

			if i < 3 {
				s.logger.Error("Thread request received bad response", zap.String("board", board), zap.Int64("thread-number", threadNumber), zap.String("status", resp.Status))
				time.Sleep(30 * time.Second)

				continue
			} else {
				return nil, fmt.Errorf("Error processing thread response: Received status %s", resp.Status)
			}
		}

		var thread Thread

		jsonDecoder := json.NewDecoder(resp.Body)
		err = jsonDecoder.Decode(&thread)
		resp.Body.Close()

		if err != nil {
			if i < 3 {
				s.logger.Error("Error unmarshalling thread response body", zap.String("board", board), zap.Int64("thread-number", threadNumber), zap.Error(err))
				time.Sleep(30 * time.Second)

				continue
			} else {
				return nil, fmt.Errorf("Error unmarshalling thread response: %s", err)
			}
		}

		return thread.Posts, nil
	}
}
