package manager

import (
	"context"
	"database/sql"
	"kaguya/api"
	"kaguya/config"
	"kaguya/db"
	"kaguya/media"
	"kaguya/thumbnail"
	"kaguya/utils"
	"sync"
	"time"

	"github.com/samber/lo"
	"github.com/uptrace/bun"
	"go.uber.org/zap"
)

const insertBatchSize = 50
const archiveBatchSize = 20

//BoardManager manages a board
type BoardManager struct {
	apiService       *api.Service
	pg               *bun.DB
	mediaService     *media.Service
	thumbnailService *thumbnail.Service
	threadCache      map[int64]cachedThread
	longNapTime      time.Duration
	media            bool
	thumbnails       bool
	skipArchive      bool
	board            string
	logger           *zap.Logger
}

//NewBoardManager creates and returns a board manager
func NewBoardManager(
	pg *bun.DB,
	mediaService *media.Service,
	thumbnailService *thumbnail.Service,
	boardConfig config.BoardConfig,
	apiService *api.Service,
	logger *zap.Logger,
) BoardManager {
	longNapTime, err := time.ParseDuration(boardConfig.LongNapTime)
	if err != nil {
		longNapTime = 50 * time.Second
	}

	return BoardManager{
		apiService:       apiService,
		pg:               pg,
		mediaService:     mediaService,
		thumbnailService: thumbnailService,
		threadCache:      make(map[int64]cachedThread),
		longNapTime:      longNapTime,
		media:            boardConfig.Media,
		thumbnails:       boardConfig.Thumbnails,
		board:            boardConfig.Name,
		skipArchive:      boardConfig.SkipArchive,
		logger:           logger,
	}
}

//LoadArchivePosts crawls through the archive
//and downloads everything in it
func (b *BoardManager) LoadArchivePosts() error {
	//Inserting all of the archived threads
	//can take a bit of time on boards like
	///v/ where there are usually 3000 archived threads,
	//so new threads may be archived during the process
	//
	//Hence, we do enough passes for there to be an empty run and call it a day
	//
	//Threads can be still archived during the
	//extremely brief time between the last pass and
	//querying the catalog endpoint
	//
	//I don't particularly care
	insertedArchiveThreads := make(map[int64]bool)
	var mutex sync.Mutex
	defer b.logger.Sync()

	for i := 0; true; i++ {
		b.logger.Info("Loading archived threads", zap.String("board", b.board), zap.Int("count", i))

		threadsInsertedInThisLoop := 0
		archive, err := b.apiService.GetRawArchive(b.board)

		if err != nil {
			return err
		}

		for _, archiveChunk := range lo.Chunk(archive, archiveBatchSize) {
			dbPosts := make([]db.Post, 0, 10)
			var wg sync.WaitGroup

			mutex.Lock()
			for i, no := range archiveChunk {
				_, threadAlreadyInserted := insertedArchiveThreads[no]

				if threadAlreadyInserted {
					continue
				}

				wg.Add(1)
				go func(i int, no int64) {
					defer wg.Done()

					posts, err := b.apiService.GetRawThread(b.board, no)

					if err != nil {
						b.logger.Error("Error loading archive thread", zap.String("board", b.board), zap.Int64("thread-number", no), zap.Error(err))
					}

					mutex.Lock()
					dbPosts = append(dbPosts, lo.Map(posts, func(p api.Post, _ int) db.Post { return db.ToPostModel(b.board, p) })...)
					threadsInsertedInThisLoop++
					insertedArchiveThreads[no] = true
					mutex.Unlock()

				}(i, no)
			}
			mutex.Unlock()
			wg.Wait()

			threadNumbers := lo.FilterMap(dbPosts, func(p db.Post, _ int) (int64, bool) {
				if p.Op {
					return p.PostNumber, true
				}
				return 0, false
			})

			utils.PatchLastModifiedCreatedAt(dbPosts, time.Now())

			tx, err := b.pg.BeginTx(context.Background(), &sql.TxOptions{})

			if err != nil {
				return err
			}

			for _, batch := range lo.Chunk(dbPosts, insertBatchSize) {
				_, err := tx.NewInsert().
					Model(&batch).
					On("CONFLICT(board, post_number) DO UPDATE SET comment = EXCLUDED.comment, last_modified = EXCLUDED.last_modified, time_media_deleted = COALESCE(post.time_media_deleted, EXCLUDED.time_media_deleted), sticky = CASE post.op WHEN TRUE THEN post.sticky IS TRUE OR EXCLUDED.sticky IS TRUE ELSE NULL END, media_deleted = EXCLUDED.media_deleted, posters = EXCLUDED.posters, closed = CASE post.op WHEN TRUE THEN post.closed IS TRUE OR EXCLUDED.closed IS TRUE ELSE NULL END").
					Returning("NULL").
					Exec(context.Background())

				if err != nil {
					return err
				}
			}

			if len(threadNumbers) > 0 {
				_, err := tx.NewUpdate().
					Model(&db.Post{}).
					Set("replies = (SELECT COUNT(*) - 1 FROM post post_subquery WHERE post_subquery.board = ? AND post_subquery.thread_number = post.post_number)", b.board).
					Where("op").
					Where("board = ?", b.board).
					Where("post_number IN (?)", bun.In(threadNumbers)).
					Returning("NULL").
					Exec(context.Background())

				if err != nil {
					return err
				}
			}

			if err := tx.Commit(); err != nil {
				return err
			}

			if b.media {
				b.logger.Debug("Loading media", zap.String("board", b.board))
				b.mediaService.Enqueue(dbPosts)
			}

			if b.thumbnails {
				b.logger.Debug("Loading thumbnails", zap.String("board", b.board))
				b.thumbnailService.Enqueue(dbPosts)
			}
		}

		if threadsInsertedInThisLoop == 0 {
			break
		}

		b.logger.Sync()
	}

	return nil
}

//Init itiates a board manager
func (b *BoardManager) Init() error {
	defer b.logger.Sync()

	b.logger.Info("Init'ing board manager", zap.String("board", b.board))

	if !b.skipArchive {
		if err := b.LoadArchivePosts(); err != nil {
			return err
		}
	}

	var mutex sync.Mutex
	dbPosts := make([]db.Post, 0, 10)

	catalog, err := b.apiService.GetRawCatalog(b.board)

	if err != nil {
		return err
	}

	var wg sync.WaitGroup

	wg.Add(len(catalog))
	for _, catalogThread := range catalog {
		go func(catalogThread api.CatalogThread) {
			defer wg.Done()

			posts, err := b.apiService.GetRawThread(b.board, catalogThread.No)

			if err != nil {
				b.logger.Error("Error looking up catalog thread", zap.String("board", b.board), zap.Int64("thread-number", catalogThread.No), zap.Error(err))
				return
			}

			mutex.Lock()
			dbPosts = append(dbPosts, lo.Map(posts, func(p api.Post, _ int) db.Post { return db.ToPostModel(b.board, p) })...)
			b.threadCache[catalogThread.No] = cachedThread{
				lastModified: catalogThread.LastModified,
				posts:        toCachedPosts(posts),
			}
			mutex.Unlock()

		}(catalogThread)
	}

	wg.Wait()

	threadNumbers := lo.FilterMap(dbPosts, func(p db.Post, _ int) (int64, bool) {
		if p.Op {
			return p.PostNumber, true
		}
		return 0, false
	})

	utils.PatchLastModifiedCreatedAt(dbPosts, time.Now())

	tx, err := b.pg.BeginTx(context.Background(), &sql.TxOptions{})

	if err != nil {
		return err
	}

	for _, batch := range lo.Chunk(dbPosts, insertBatchSize) {
		_, err := tx.NewInsert().
			Model(&batch).
			On("CONFLICT(board, post_number) DO UPDATE SET comment = EXCLUDED.comment, last_modified = EXCLUDED.last_modified, time_media_deleted = COALESCE(post.time_media_deleted, EXCLUDED.time_media_deleted), sticky = CASE post.op WHEN TRUE THEN post.sticky IS TRUE OR EXCLUDED.sticky IS TRUE ELSE NULL END, media_deleted = EXCLUDED.media_deleted, posters = EXCLUDED.posters, closed = CASE post.op WHEN TRUE THEN post.closed IS TRUE OR EXCLUDED.closed IS TRUE ELSE NULL END").
			Returning("NULL").
			Exec(context.Background())

		if err != nil {
			return err
		}
	}

	if len(threadNumbers) > 0 {
		_, err := tx.NewUpdate().
			Model(&db.Post{}).
			Set("replies = (SELECT COUNT(*) - 1 FROM post post_subquery WHERE post_subquery.board = ? AND post_subquery.thread_number = post.post_number)", b.board).
			Where("op").
			Where("board = ?", b.board).
			Where("post_number IN (?)", bun.In(threadNumbers)).
			Returning("NULL").
			Exec(context.Background())

		if err != nil {
			return err
		}
	}

	if err := tx.Commit(); err != nil {
		return err
	}

	if b.media {
		b.logger.Debug("Loading media", zap.String("board", b.board))
		b.mediaService.Enqueue(dbPosts)
	}

	if b.thumbnails {
		b.logger.Debug("Loading thumbnails", zap.String("board", b.board))
		b.thumbnailService.Enqueue(dbPosts)
	}

	return nil
}

//Run is the main loop for a board manager.
//A board manager should be init'd beforehand
func (b *BoardManager) Run() {
	for {
		time.Sleep(b.longNapTime)

		b.logger.Info("Starting loop", zap.String("board", b.board))

		archive, err := b.apiService.GetStructuredArchive(b.board)

		if err != nil {
			b.logger.Error("Error looking up archive", zap.String("board", b.board), zap.Error(err))
			continue
		}

		catalog, err := b.apiService.GetStructuredCatalog(b.board)

		if err != nil {
			b.logger.Error("Error looking up catalog", zap.String("board", b.board), zap.Error(err))
			continue
		}

		if catalog == nil {
			b.logger.Debug("Catalog is empty, skipping", zap.String("board", b.board))
			continue
		}

		time.Sleep(5 * time.Second)

		var mutex sync.Mutex
		var wg sync.WaitGroup

		newPosts := make([]db.Post, 0, 10)
		newOps := make([]int64, 0, 10)
		updatedOps := make([]updatedOp, 0, 10)
		updatedPosts := make([]updatedPost, 0, 10)
		deletedPosts := make([]int64, 0, 10)

		//We release this mutex *after* dispatching all goroutines.
		mutex.Lock()
		for threadNumber, cThread := range b.threadCache {
			_, threadArchived := archive[threadNumber]
			catalogThread, threadInCatalog := catalog[threadNumber]

			if threadArchived {
				delete(b.threadCache, threadNumber)
				//A thread might be in both the archive and the catalog
				//due to 4chan cache
				if threadInCatalog {
					delete(catalog, threadNumber)
				}

				wg.Add(1)
				go func(threadNumber int64) {
					defer wg.Done()

					rawOp, rawPosts, err := b.apiService.GetStructuredThread(b.board, threadNumber)

					if err != nil {
						b.logger.Error("Error looking up archived thread", zap.String("board", b.board), zap.Int64("thread-number", threadNumber), zap.Error(err))
						return
					}

					mutex.Lock()
					updatedOps = append(updatedOps, toUpdatedOp(&rawOp))

					for postNumber, cachedPost := range cThread.posts {
						updatedPost, postNotDeleted := rawPosts[postNumber]
						delete(rawPosts, postNumber)

						if !postNotDeleted {
							b.logger.Debug("Marking post as deleted", zap.String("board", b.board), zap.Int64("post-number", postNumber))
							deletedPosts = append(deletedPosts, postNumber)
						} else if postModified(&updatedPost, &cachedPost) {
							updatedPosts = append(updatedPosts, toUpdatedPost(&updatedPost))
						}
					}

					for _, rawPost := range rawPosts {
						newPosts = append(newPosts, db.ToPostModel(b.board, rawPost))
					}

					mutex.Unlock()
				}(threadNumber)
			} else if threadInCatalog {
				delete(catalog, threadNumber)

				if catalogThread.LastModified != cThread.lastModified {
					b.logger.Debug("Updating thread", zap.String("board", b.board), zap.Int64("thread-number", threadNumber))
					wg.Add(1)
					go func(threadNumber int64, cThread cachedThread) {
						defer wg.Done()

						rawOp, rawPosts, err := b.apiService.GetStructuredThread(b.board, threadNumber)

						if err != nil {
							b.logger.Error("Error looking up catalog thread", zap.String("board", b.board), zap.Int64("thread-number", threadNumber), zap.Error(err))
							return
						}

						mutex.Lock()
						updatedOps = append(updatedOps, toUpdatedOp(&rawOp))

						for postNumber, cachedPost := range cThread.posts {
							updatedPost, postNotDeleted := rawPosts[postNumber]
							delete(rawPosts, postNumber)

							if !postNotDeleted {
								b.logger.Debug("Marking post as deleted", zap.String("board", b.board), zap.Int64("post-number", postNumber))
								deletedPosts = append(deletedPosts, postNumber)
							} else if postModified(&updatedPost, &cachedPost) {
								cThread.posts[postNumber] = toCachedPost(&updatedPost)
								updatedPosts = append(updatedPosts, toUpdatedPost(&updatedPost))
							}
						}

						for _, rawPost := range rawPosts {
							cThread.posts[rawPost.No] = toCachedPost(&rawPost)
							newPosts = append(newPosts, db.ToPostModel(b.board, rawPost))
						}

						cThread.lastModified = catalogThread.LastModified
						b.threadCache[threadNumber] = cThread

						mutex.Unlock()
					}(threadNumber, cThread)
				}
			} else {
				delete(b.threadCache, threadNumber)
				b.logger.Debug("Marking thread as deleted", zap.String("board", b.board), zap.Int64("thread-number", threadNumber))
				deletedPosts = append(deletedPosts, threadNumber)
			}
		}

		wg.Add(len(catalog))
		for threadNumber, catalogThread := range catalog {
			go func(threadNumber int64, catalogThread api.CatalogThread) {
				defer wg.Done()
				rawPosts, err := b.apiService.GetRawThread(b.board, threadNumber)

				if err != nil {
					b.logger.Error("Error looking up new thread", zap.String("board", b.board), zap.Int64("thread-number", threadNumber), zap.Error(err))
					return
				}

				mutex.Lock()

				newOps = append(newOps, threadNumber)
				cachedPosts := toCachedPosts(rawPosts[1:])

				for _, rawPost := range rawPosts {
					newPosts = append(newPosts, db.ToPostModel(b.board, rawPost))
				}

				b.threadCache[threadNumber] = cachedThread{
					lastModified: catalogThread.LastModified,
					posts:        cachedPosts,
				}

				mutex.Unlock()
			}(threadNumber, catalogThread)
		}

		mutex.Unlock()
		wg.Wait()

		now := time.Now()
		utils.PatchLastModifiedCreatedAt(newPosts, now)

		tx, err := b.pg.BeginTx(context.Background(), &sql.TxOptions{})

		if err != nil {
			panic(err)
		}

		for _, newPostsBatch := range lo.Chunk(newPosts, insertBatchSize) {
			_, err := tx.NewInsert().
				Model(&newPostsBatch).
				On("CONFLICT DO NOTHING").
				Returning("NULL").
				Exec(context.Background())

			if err != nil {
				b.logger.Fatal("Error inserting new posts into database", zap.String("board", b.board), zap.Error(err))
			}
		}

		if len(newOps) > 0 {
			_, err := tx.NewUpdate().
				Model(&db.Post{}).
				Set("replies = (SELECT COUNT(*) - 1 FROM post post_subquery WHERE post_subquery.board = post.board AND post_subquery.thread_number = post.post_number)").
				Set("last_modified = ?", now).
				Where("post.op").
				Where("post.board = ?", b.board).
				Where("post.post_number IN (?)", bun.In(newOps)).
				Returning("NULL").
				Exec(context.Background())

			if err != nil {
				b.logger.Fatal("Error updating new OPs", zap.String("board", b.board), zap.Error(err))
			}
		}

		if len(deletedPosts) > 0 {
			_, err := tx.NewUpdate().
				Model(&db.Post{}).
				Set("deleted = TRUE").
				Set("last_modified = ?", now).
				Where("board = ?", b.board).
				Where("post_number IN (?)", bun.In(deletedPosts)).
				Returning("NULL").
				Exec(context.Background())

			if err != nil {
				b.logger.Fatal("Error marking deleted posts", zap.String("board", b.board), zap.Error(err))
			}
		}

		if len(updatedPosts) > 0 {
			_, err := tx.NewUpdate().
				With("_data", tx.NewValues(&updatedPosts)).
				Model(&db.Post{}).
				TableExpr("_data").
				Set("comment = _data.comment").
				Set("media_deleted = CASE _data.media_deleted WHEN TRUE THEN TRUE ELSE CASE post.has_media WHEN TRUE THEN FALSE ELSE NULL END END").
				Set("time_media_deleted = COALESCE(post.time_media_deleted, ?)", now).
				Set("last_modified = ?", now).
				Where("post.board = ?", b.board).
				Where("post.post_number = _data.post_number").
				Returning("NULL").
				Exec(context.Background())

			if err != nil {
				b.logger.Fatal("Error marking deleted posts", zap.String("board", b.board), zap.Error(err))
			}
		}

		if len(updatedOps) > 0 {
			_, err := tx.NewUpdate().
				With("_data", tx.NewValues(&updatedOps)).
				Model(&db.Post{}).
				TableExpr("_data").
				Set("comment = _data.comment").
				Set("media_deleted = CASE _data.media_deleted WHEN TRUE THEN TRUE ELSE CASE post.has_media WHEN TRUE THEN FALSE ELSE NULL END END").
				Set("time_media_deleted = COALESCE(post.time_media_deleted, ?)", now).
				Set("last_modified = ?", now).
				Set("closed = _data.closed").
				Set("sticky = post.sticky OR _data.sticky").
				Where("post.board = ?", b.board).
				Where("post.post_number = _data.post_number").
				Returning("NULL").
				Exec(context.Background())

			if err != nil {
				b.logger.Fatal("Error updating OPs", zap.String("board", b.board), zap.Error(err))
			}
		}

		if err := tx.Commit(); err != nil {
			b.logger.Fatal("Error commiting transaction", zap.String("board", b.board), zap.Error(err))
		}

		if b.media {
			b.logger.Debug("Loading media", zap.String("board", b.board))
			b.mediaService.Enqueue(newPosts)
		}

		if b.thumbnails {
			b.logger.Debug("Loading thumbnails", zap.String("board", b.board))
			b.thumbnailService.Enqueue(newPosts)
		}

		b.logger.Sync()
	}
}
