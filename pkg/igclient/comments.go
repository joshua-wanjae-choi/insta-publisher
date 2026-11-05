package igclient

import (
	"context"
	"fmt"
	"net/url"
)

type Comment struct {
	ID        string `json:"id"`
	Text      string `json:"text"`
	Timestamp string `json:"timestamp"`
	Username  string `json:"username"`
	LikeCount int    `json:"like_count"`
	Hidden    bool   `json:"hidden"`
}

type CommentsResponse struct {
	Data []Comment `json:"data"`
}

type PostCommentResponse struct {
	ID string `json:"id"`
}

type MediaItem struct {
	ID           string `json:"id"`
	Caption      string `json:"caption"`
	MediaType    string `json:"media_type"`
	Permalink    string `json:"permalink"`
	Timestamp    string `json:"timestamp"`
	CommentsCount int   `json:"comments_count"`
}

type RecentMediaResponse struct {
	Data []MediaItem `json:"data"`
}

// GetRecentMedia retrieves recent posts of the account
func (c *Client) GetRecentMedia(ctx context.Context, limit int) ([]MediaItem, error) {
	if limit <= 0 {
		limit = 10
	}
	endpoint := fmt.Sprintf("%s/media", c.accountID)
	query := url.Values{}
	query.Set("fields", "id,caption,media_type,permalink,timestamp,comments_count")
	query.Set("limit", fmt.Sprintf("%d", limit))

	var res RecentMediaResponse
	if err := c.get(ctx, endpoint, query, &res); err != nil {
		return nil, fmt.Errorf("get recent media: %w", err)
	}
	return res.Data, nil
}

// GetComments retrieves top-level comments for a given media object
func (c *Client) GetComments(ctx context.Context, mediaID string) ([]Comment, error) {
	endpoint := fmt.Sprintf("%s/comments", mediaID)
	query := url.Values{}
	query.Set("fields", "id,text,timestamp,username,like_count,hidden")

	var res CommentsResponse
	if err := c.get(ctx, endpoint, query, &res); err != nil {
		return nil, fmt.Errorf("get comments for media %s: %w", mediaID, err)
	}
	return res.Data, nil
}

// ReplyComment replies to an existing comment
func (c *Client) ReplyComment(ctx context.Context, commentID, replyText string) (string, error) {
	endpoint := fmt.Sprintf("%s/replies", commentID)
	data := url.Values{}
	data.Set("message", replyText)

	var res PostCommentResponse
	if err := c.post(ctx, endpoint, data, &res); err != nil {
		return "", fmt.Errorf("reply to comment %s: %w", commentID, err)
	}
	return res.ID, nil
}

// AddCommentToMedia adds a new comment to a media post
func (c *Client) AddCommentToMedia(ctx context.Context, mediaID, text string) (string, error) {
	endpoint := fmt.Sprintf("%s/comments", mediaID)
	data := url.Values{}
	data.Set("message", text)

	var res PostCommentResponse
	if err := c.post(ctx, endpoint, data, &res); err != nil {
		return "", fmt.Errorf("post comment to media %s: %w", mediaID, err)
	}
	return res.ID, nil
}
