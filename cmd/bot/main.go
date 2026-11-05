package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"
	"time"

	"insta/pkg/config"
	"insta/pkg/igclient"
)

func main() {
	mode := flag.String("mode", "status", "Mode to run: status, post-one, test-comment, or daemon")
	imageURL := flag.String("image", "", "Image public URL (for post-one mode)")
	caption := flag.String("caption", "", "Post caption (for post-one mode)")
	mediaID := flag.String("media-id", "", "Media ID (for test-comment mode)")
	commentText := flag.String("text", "", "Comment text (for test-comment mode)")
	flag.Parse()

	cfg, err := config.Load(".env")
	if err != nil {
		log.Printf("Notice: could not load .env file directly (%v), using system env", err)
	}

	if cfg.InstagramAccountID == "" || cfg.AccessToken == "" {
		log.Println("================================================================")
		log.Println("⚠️  INSTAGRAM_ACCOUNT_ID or GRAPH_API_ACCESS_TOKEN is missing!")
		log.Println("   Please copy .env.example to .env and configure your credentials.")
		log.Println("================================================================")
		os.Exit(1)
	}

	client := igclient.New(cfg.InstagramAccountID, cfg.AccessToken, cfg.APIVersion)
	ctx := context.Background()

	switch *mode {
	case "status":
		checkAccountStatus(ctx, client)
	case "post-one":
		if *imageURL == "" {
			log.Fatalf("Error: --image is required for post-one mode")
		}
		publishSingle(ctx, client, *imageURL, *caption)
	case "test-comment":
		if *mediaID == "" || *commentText == "" {
			log.Fatalf("Error: --media-id and --text are required for test-comment mode")
		}
		addComment(ctx, client, *mediaID, *commentText)
	case "daemon":
		runDaemon(ctx, client, cfg)
	default:
		log.Fatalf("Unknown mode: %s. Available modes: status, post-one, test-comment, daemon", *mode)
	}
}

func checkAccountStatus(ctx context.Context, client *igclient.Client) {
	log.Println("Checking Instagram account connection...")
	info, err := client.GetAccountInfo(ctx)
	if err != nil {
		log.Fatalf("❌ Failed to get account info: %v", err)
	}

	fmt.Println("----------------------------------------")
	fmt.Printf("✅ Connected Successfully!\n")
	fmt.Printf("ID       : %v\n", info["id"])
	fmt.Printf("Username : @%v\n", info["username"])
	fmt.Printf("Name     : %v\n", info["name"])
	fmt.Println("----------------------------------------")

	recent, err := client.GetRecentMedia(ctx, 3)
	if err != nil {
		log.Printf("⚠️ Could not fetch recent media: %v", err)
		return
	}
	fmt.Printf("📸 Recent Posts (last %d):\n", len(recent))
	for idx, item := range recent {
		fmt.Printf("  [%d] ID: %s | Comments: %d | Link: %s\n", idx+1, item.ID, item.CommentsCount, item.Permalink)
	}
}

func publishSingle(ctx context.Context, client *igclient.Client, imageURL, caption string) {
	log.Printf("Publishing image: %s\n", imageURL)
	id, err := client.UploadAndPublishPhoto(ctx, imageURL, caption)
	if err != nil {
		log.Fatalf("❌ Failed to publish: %v", err)
	}
	log.Printf("🎉 Successfully published! Media ID: %s\n", id)
}

func addComment(ctx context.Context, client *igclient.Client, mediaID, text string) {
	log.Printf("Posting comment to media %s...\n", mediaID)
	id, err := client.AddCommentToMedia(ctx, mediaID, text)
	if err != nil {
		log.Fatalf("❌ Failed to post comment: %v", err)
	}
	log.Printf("💬 Comment posted! Comment ID: %s\n", id)
}

func runDaemon(ctx context.Context, client *igclient.Client, cfg *config.Config) {
	log.Printf("🤖 Instagram Automation Daemon started.")
	log.Printf("⏰ Daily post scheduled at: %s", cfg.PostScheduleTime)
	log.Printf("💬 Comment check interval: every %d minutes", cfg.CommentCheckIntervalMins)

	postTicker := time.NewTicker(1 * time.Minute)
	defer postTicker.Stop()

	commentTicker := time.NewTicker(time.Duration(cfg.CommentCheckIntervalMins) * time.Minute)
	defer commentTicker.Stop()

	lastPostDate := ""

	for {
		select {
		case now := <-postTicker.C:
			currentTimeStr := now.Format("15:04")
			todayDate := now.Format("2006-01-02")

			if currentTimeStr == cfg.PostScheduleTime && lastPostDate != todayDate {
				log.Println("🚀 Triggering daily scheduled post...")
				if err := processQueue(ctx, client); err != nil {
					log.Printf("⚠️ Error processing queue: %v", err)
				} else {
					lastPostDate = todayDate
				}
			}
		case <-commentTicker.C:
			log.Println("🔍 Checking new comments...")
			monitorComments(ctx, client)
		case <-ctx.Done():
			return
		}
	}
}

func processQueue(ctx context.Context, client *igclient.Client) error {
	// Look into queue folder for .json or .txt + image
	files, err := os.ReadDir("queue")
	if err != nil {
		return err
	}

	for _, f := range files {
		if f.IsDir() {
			continue
		}
		if strings.HasSuffix(f.Name(), ".url") {
			filePath := filepath.Join("queue", f.Name())
			data, err := os.ReadFile(filePath)
			if err != nil {
				continue
			}
			parts := strings.SplitN(string(data), "\n", 2)
			imgURL := strings.TrimSpace(parts[0])
			caption := ""
			if len(parts) > 1 {
				caption = strings.TrimSpace(parts[1])
			}

			log.Printf("Found item to post: %s", f.Name())
			mediaID, err := client.UploadAndPublishPhoto(ctx, imgURL, caption)
			if err != nil {
				return fmt.Errorf("failed to publish %s: %w", f.Name(), err)
			}

			log.Printf("✅ Published successfully! Media ID: %s", mediaID)
			_ = os.Rename(filePath, filepath.Join("processed", f.Name()))
			return nil
		}
	}
	log.Println("ℹ️ Queue is empty. No post published today.")
	return nil
}

func monitorComments(ctx context.Context, client *igclient.Client) {
	recent, err := client.GetRecentMedia(ctx, 5)
	if err != nil {
		log.Printf("Failed to get recent posts for comments: %v", err)
		return
	}

	for _, item := range recent {
		if item.CommentsCount == 0 {
			continue
		}
		comments, err := client.GetComments(ctx, item.ID)
		if err != nil {
			log.Printf("Failed to get comments for %s: %v", item.ID, err)
			continue
		}
		log.Printf("Post [%s] has %d comments", item.ID, len(comments))
		for _, c := range comments {
			log.Printf("  ↳ @%s: %s (Time: %s)", c.Username, c.Text, c.Timestamp)
		}
	}
}
