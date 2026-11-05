package igclient

import (
	"context"
	"fmt"
	"net/url"
	"strings"
	"time"
)

type MediaContainerResponse struct {
	ID string `json:"id"`
}

type MediaStatusResponse struct {
	ID         string `json:"id"`
	StatusCode string `json:"status_code"` // "EXPIRED", "ERROR", "FINISHED", "IN_PROGRESS", "PUBLISHED"
}

type PublishMediaResponse struct {
	ID string `json:"id"`
}

// CreatePhotoContainer creates an Instagram media container for a single image (4:5 or 1:1)
// isCarouselItem should be true if this container is to be included in a carousel post (caption must be omitted).
func (c *Client) CreatePhotoContainer(ctx context.Context, imageURL, caption string, isCarouselItem bool) (string, error) {
	endpoint := fmt.Sprintf("%s/media", c.accountID)
	data := url.Values{}
	data.Set("image_url", imageURL)
	if isCarouselItem {
		data.Set("is_carousel_item", "true")
	} else if caption != "" {
		data.Set("caption", caption)
	}

	var res MediaContainerResponse
	if err := c.post(ctx, endpoint, data, &res); err != nil {
		return "", fmt.Errorf("create photo container: %w", err)
	}
	return res.ID, nil
}

// CreateCarouselContainer creates a carousel container referencing multiple item container IDs
func (c *Client) CreateCarouselContainer(ctx context.Context, childrenIDs []string, caption string) (string, error) {
	endpoint := fmt.Sprintf("%s/media", c.accountID)
	data := url.Values{}
	data.Set("media_type", "CAROUSEL")
	data.Set("children", strings.Join(childrenIDs, ","))
	if caption != "" {
		data.Set("caption", caption)
	}

	var res MediaContainerResponse
	if err := c.post(ctx, endpoint, data, &res); err != nil {
		return "", fmt.Errorf("create carousel container: %w", err)
	}
	return res.ID, nil
}

// UploadAndPublishCarousel creates containers for each image, creates carousel container, and publishes
func (c *Client) UploadAndPublishCarousel(ctx context.Context, imageURLs []string, caption string) (string, error) {
	if len(imageURLs) == 0 {
		return "", fmt.Errorf("at least one image required")
	}
	if len(imageURLs) == 1 {
		return c.UploadAndPublishPhoto(ctx, imageURLs[0], caption)
	}

	var childrenIDs []string
	for idx, imgURL := range imageURLs {
		cid, err := c.CreatePhotoContainer(ctx, imgURL, "", true)
		if err != nil {
			return "", fmt.Errorf("create child %d failed: %w", idx, err)
		}
		if err := c.WaitForContainerReady(ctx, cid, 1*time.Minute); err != nil {
			return "", fmt.Errorf("wait child %d failed: %w", idx, err)
		}
		childrenIDs = append(childrenIDs, cid)
	}

	carouselID, err := c.CreateCarouselContainer(ctx, childrenIDs, caption)
	if err != nil {
		return "", fmt.Errorf("create carousel container: %w", err)
	}

	if err := c.WaitForContainerReady(ctx, carouselID, 1*time.Minute); err != nil {
		return "", fmt.Errorf("wait carousel container: %w", err)
	}

	return c.PublishMedia(ctx, carouselID)
}

// WaitForContainerReady checks container processing status
func (c *Client) WaitForContainerReady(ctx context.Context, containerID string, maxWait time.Duration) error {
	deadline := time.Now().Add(maxWait)
	endpoint := containerID
	query := url.Values{}
	query.Set("fields", "status_code")

	for time.Now().Before(deadline) {
		var res MediaStatusResponse
		if err := c.get(ctx, endpoint, query, &res); err != nil {
			return fmt.Errorf("check container status: %w", err)
		}

		switch res.StatusCode {
		case "FINISHED":
			return nil
		case "ERROR", "EXPIRED":
			return fmt.Errorf("media container ended with status: %s", res.StatusCode)
		case "IN_PROGRESS":
			// 대기 후 재시도
		default:
			// 이미지는 별도 status_code 없이 바로 준비되는 경우가 많음
			if res.StatusCode == "" {
				return nil
			}
		}

		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(2 * time.Second):
		}
	}

	return fmt.Errorf("timed out waiting for container %s", containerID)
}

// PublishMedia publishes a created container to Instagram feed
func (c *Client) PublishMedia(ctx context.Context, creationID string) (string, error) {
	endpoint := fmt.Sprintf("%s/media_publish", c.accountID)
	data := url.Values{}
	data.Set("creation_id", creationID)

	var res PublishMediaResponse
	if err := c.post(ctx, endpoint, data, &res); err != nil {
		return "", fmt.Errorf("publish media: %w", err)
	}
	return res.ID, nil
}

// UploadAndPublishPhoto creates photo container, waits for ready, and publishes to feed
func (c *Client) UploadAndPublishPhoto(ctx context.Context, imageURL, caption string) (string, error) {
	containerID, err := c.CreatePhotoContainer(ctx, imageURL, caption, false)
	if err != nil {
		return "", err
	}

	if err := c.WaitForContainerReady(ctx, containerID, 1*time.Minute); err != nil {
		return "", err
	}

	mediaID, err := c.PublishMedia(ctx, containerID)
	if err != nil {
		return "", err
	}

	return mediaID, nil
}
