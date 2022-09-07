package api

import (
	"kaguya/config"
	"net/http"
	"sync"
	"time"

	"github.com/samber/lo"
	"go.uber.org/zap"
)

type catalogRequest struct {
	board              string
	returnChannel      chan []CatalogPage
	errorReturnChannel chan error
}

type archiveRequest struct {
	board              string
	returnChannel      chan []int64
	errorReturnChannel chan error
}

type threadRequest struct {
	board              string
	threadNumber       int64
	returnChannel      chan []Post
	errorReturnChannel chan error
}

//Service organizes and throttles API calls.
type Service struct {
	host   string
	client http.Client

	lastCatalogRequest      map[string]time.Time
	lastCatalogRequestMutex sync.Mutex

	lastArchiveRequest      map[string]time.Time
	lastArchiveRequestMutex sync.Mutex

	napTime time.Duration

	goroutines      int
	catalogRequests chan catalogRequest
	archiveRequests chan archiveRequest
	threadRequests  chan threadRequest

	logger *zap.Logger
}

//NewService creates a service
func NewService(
	apiConfig config.APIConfig,
	logger *zap.Logger,
) *Service {
	requestTimeout, _ := time.ParseDuration(apiConfig.RequestTimeout)

	client := http.Client{
		Timeout: requestTimeout,
	}

	napTime, err := time.ParseDuration(apiConfig.NapTime)
	if err != nil {
		napTime = 0 * time.Second
	}

	s := &Service{
		host:            apiConfig.Host,
		client:          client,
		goroutines:      apiConfig.Goroutines,
		napTime:         napTime,
		logger:          logger,
		catalogRequests: make(chan catalogRequest),
		archiveRequests: make(chan archiveRequest),
		threadRequests:  make(chan threadRequest),

		lastCatalogRequest: make(map[string]time.Time),
		lastArchiveRequest: make(map[string]time.Time),
	}

	for i := 0; i < s.goroutines; i++ {
		s.logger.Debug("Starting API goroutine", zap.Int("count", i))

		go func() {
			for {
				s.logger.Sync()

				select {
				case threadRequest := <-s.threadRequests:
					{
						thread, err := s.getThread(threadRequest.board, threadRequest.threadNumber)

						if err != nil {
							threadRequest.errorReturnChannel <- err
						} else {
							threadRequest.returnChannel <- thread
						}
					}
				case archiveRequest := <-s.archiveRequests:
					{
						archive, err := s.getArchive(archiveRequest.board)

						if err != nil {
							archiveRequest.errorReturnChannel <- err
						} else {
							archiveRequest.returnChannel <- archive
						}
					}
				case catalogRequest := <-s.catalogRequests:
					{
						catalog, err := s.getCatalog(catalogRequest.board)

						if err != nil {
							catalogRequest.errorReturnChannel <- err
						} else {
							catalogRequest.returnChannel <- catalog
						}
					}
				}
			}
		}()
	}

	return s
}

//GetStructuredCatalog returns the board catalog as a map[int64]CatalogThread
//where the keys are the thread numbers.
func (s *Service) GetStructuredCatalog(board string) (map[int64]CatalogThread, error) {
	returnChannel := make(chan []CatalogPage)
	errorReturnChannel := make(chan error)

	s.catalogRequests <- catalogRequest{
		board:              board,
		returnChannel:      returnChannel,
		errorReturnChannel: errorReturnChannel,
	}

	select {
	case catalogPages := <-returnChannel:
		{
			return lo.KeyBy(
				lo.Flatten(lo.Map(catalogPages, func(catalogPage CatalogPage, _ int) []CatalogThread {
					for i := range catalogPage.Threads {
						catalogPage.Threads[i].Page = catalogPage.Page
					}

					return catalogPage.Threads
				})),
				func(catalogThread CatalogThread) int64 { return catalogThread.No },
			), nil
		}
	case err := <-errorReturnChannel:
		{
			return nil, err
		}
	}
}

//GetRawCatalog returns the board catalog as an array.
func (s *Service) GetRawCatalog(board string) ([]CatalogThread, error) {
	returnChannel := make(chan []CatalogPage)
	errorReturnChannel := make(chan error)

	s.catalogRequests <- catalogRequest{
		board:              board,
		returnChannel:      returnChannel,
		errorReturnChannel: errorReturnChannel,
	}

	select {
	case catalogPages := <-returnChannel:
		{
			return lo.Flatten(lo.Map(
				catalogPages,
				func(catalogPage CatalogPage, _ int) []CatalogThread {
					for i := range catalogPage.Threads {
						catalogPage.Threads[i].Page = catalogPage.Page
					}

					return catalogPage.Threads
				},
			)), nil
		}
	case err := <-errorReturnChannel:
		{
			return nil, err
		}
	}
}

//GetStructuredArchive returns the archive as a Set (a map[int64]bool where the
//value is irrelevant)
func (s *Service) GetStructuredArchive(board string) (map[int64]bool, error) {
	returnChannel := make(chan []int64)
	errorReturnChannel := make(chan error)

	s.archiveRequests <- archiveRequest{
		board:              board,
		returnChannel:      returnChannel,
		errorReturnChannel: errorReturnChannel,
	}

	select {
	case archive := <-returnChannel:
		{
			structuredArchive := make(map[int64]bool)

			for _, threadNumber := range archive {
				structuredArchive[threadNumber] = true
			}

			return structuredArchive, nil
		}
	case err := <-errorReturnChannel:
		{
			return nil, err
		}
	}
}

//GetRawArchive returns the archive as an array
func (s *Service) GetRawArchive(board string) ([]int64, error) {
	returnChannel := make(chan []int64)
	errorReturnChannel := make(chan error)

	s.archiveRequests <- archiveRequest{
		board:              board,
		returnChannel:      returnChannel,
		errorReturnChannel: errorReturnChannel,
	}

	select {
	case archive := <-returnChannel:
		{
			return archive, nil
		}
	case err := <-errorReturnChannel:
		{
			return nil, err
		}
	}
}

//GetStructuredThread returns a thread as an OP and
//a map[int64]Post where they keys are post numbers.
func (s *Service) GetStructuredThread(board string, threadNumber int64) (Post, map[int64]Post, error) {
	returnChannel := make(chan []Post)
	errorReturnChannel := make(chan error)

	s.threadRequests <- threadRequest{
		board:              board,
		threadNumber:       threadNumber,
		returnChannel:      returnChannel,
		errorReturnChannel: errorReturnChannel,
	}

	select {
	case posts := <-returnChannel:
		{
			return posts[0], lo.KeyBy(posts[1:], func(p Post) int64 { return p.No }), nil
		}
	case err := <-errorReturnChannel:
		{
			return Post{}, nil, err
		}
	}
}

//GetRawThread returns a thread as an array
func (s *Service) GetRawThread(board string, threadNumber int64) ([]Post, error) {
	returnChannel := make(chan []Post)
	errorReturnChannel := make(chan error)

	s.threadRequests <- threadRequest{
		board:              board,
		threadNumber:       threadNumber,
		returnChannel:      returnChannel,
		errorReturnChannel: errorReturnChannel,
	}

	select {
	case posts := <-returnChannel:
		{
			return posts, nil
		}
	case err := <-errorReturnChannel:
		{
			return nil, err
		}
	}
}
