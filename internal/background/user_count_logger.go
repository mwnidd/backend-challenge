package background

import (
	"context"
	"log"
	"time"

	"github.com/7-solutions/backend-challenge/internal/application"
)

func StartUserCountLogger(ctx context.Context, users *application.UserService, interval time.Duration) {
	ticker := time.NewTicker(interval)
	go func() {
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				log.Println("user count logger stopped")
				return
			case <-ticker.C:
				count, err := users.Count(ctx)
				if err != nil {
					log.Printf("failed to count users: %v", err)
					continue
				}
				log.Printf("total users: %d", count)
			}
		}
	}()
}
