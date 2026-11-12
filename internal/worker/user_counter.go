// Package worker holds background jobs that run next to the HTTP server.
package worker

import (
	"context"
	"log"
	"time"

	"github.com/attachaiB/backend-challenge/internal/service"
)

// RunUserCounter logs the total number of users every interval.
// It returns when ctx is cancelled, so main can wait for it on shutdown.
func RunUserCounter(ctx context.Context, userService service.UserService, interval time.Duration) {
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			log.Println("user counter stopped")
			return
		case <-ticker.C:
			countCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
			total, err := userService.Count(countCtx)
			cancel()

			if err != nil {
				log.Printf("user counter: %v", err)
				continue
			}

			log.Printf("total users: %d", total)
		}
	}
}
