package services

import (
	"bytes"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/ensoul-labs/ensoul-server/config"
	"github.com/ensoul-labs/ensoul-server/database"
	"github.com/ensoul-labs/ensoul-server/models"
	"github.com/ensoul-labs/ensoul-server/util"
	"github.com/google/uuid"
)

const lemonSqueezyAPI = "https://api.lemonsqueezy.com/v1"

// CreateCheckoutURL creates a LemonSqueezy checkout session for a user.
func CreateCheckoutURL(userID uuid.UUID, email string) (string, error) {
	apiKey := config.Cfg.LemonSqueezyAPIKey
	storeID := config.Cfg.LemonSqueezyStoreID
	variantID := config.Cfg.LemonSqueezyVariantID

	if apiKey == "" || storeID == "" || variantID == "" {
		return "", fmt.Errorf("LemonSqueezy not configured")
	}

	payload := map[string]interface{}{
		"data": map[string]interface{}{
			"type": "checkouts",
			"attributes": map[string]interface{}{
				"checkout_data": map[string]interface{}{
					"email":  email,
					"custom": map[string]string{"user_id": userID.String()},
				},
			},
			"relationships": map[string]interface{}{
				"store": map[string]interface{}{
					"data": map[string]interface{}{"type": "stores", "id": storeID},
				},
				"variant": map[string]interface{}{
					"data": map[string]interface{}{"type": "variants", "id": variantID},
				},
			},
		},
	}

	body, _ := json.Marshal(payload)
	req, err := http.NewRequest("POST", lemonSqueezyAPI+"/checkouts", bytes.NewReader(body))
	if err != nil {
		return "", err
	}
	req.Header.Set("Authorization", "Bearer "+apiKey)
	req.Header.Set("Content-Type", "application/vnd.api+json")
	req.Header.Set("Accept", "application/vnd.api+json")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("LemonSqueezy API error: %v", err)
	}
	defer resp.Body.Close()

	respBody, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != 201 {
		return "", fmt.Errorf("LemonSqueezy API returned %d: %s", resp.StatusCode, string(respBody))
	}

	var result struct {
		Data struct {
			Attributes struct {
				URL string `json:"url"`
			} `json:"attributes"`
		} `json:"data"`
	}
	if err := json.Unmarshal(respBody, &result); err != nil {
		return "", fmt.Errorf("failed to parse checkout response: %v", err)
	}

	return result.Data.Attributes.URL, nil
}

// VerifyWebhookSignature verifies LemonSqueezy webhook signature.
func VerifyWebhookSignature(payload []byte, signature string) bool {
	secret := config.Cfg.LemonSqueezyWebhookSecret
	if secret == "" {
		return false
	}
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write(payload)
	expected := hex.EncodeToString(mac.Sum(nil))
	return hmac.Equal([]byte(expected), []byte(signature))
}

// LemonSqueezyWebhookEvent represents the incoming webhook payload.
type LemonSqueezyWebhookEvent struct {
	Meta struct {
		EventName  string          `json:"event_name"`
		CustomData json.RawMessage `json:"custom_data"`
	} `json:"meta"`
	Data struct {
		ID         string `json:"id"`
		Attributes struct {
			Status        string    `json:"status"`
			RenewsAt      string    `json:"renews_at"`
			EndsAt        string    `json:"ends_at"`
			CreatedAt     string    `json:"created_at"`
			CustomerID    int       `json:"customer_id"`
			VariantID     int       `json:"variant_id"`
			UserEmail     string    `json:"user_email"`
			UpdatedAt     time.Time `json:"updated_at"`
		} `json:"attributes"`
	} `json:"data"`
}

// HandleSubscriptionWebhook processes LemonSqueezy subscription events.
func HandleSubscriptionWebhook(payload []byte) error {
	var event LemonSqueezyWebhookEvent
	if err := json.Unmarshal(payload, &event); err != nil {
		return fmt.Errorf("invalid webhook payload: %v", err)
	}

	// Extract user_id from custom data
	var customData struct {
		UserID string `json:"user_id"`
	}
	if err := json.Unmarshal(event.Meta.CustomData, &customData); err != nil || customData.UserID == "" {
		util.Log.Warn("[billing] Webhook missing user_id in custom_data, event=%s", event.Meta.EventName)
		return fmt.Errorf("missing user_id in custom_data")
	}

	userID, err := uuid.Parse(customData.UserID)
	if err != nil {
		return fmt.Errorf("invalid user_id: %s", customData.UserID)
	}

	var user models.User
	if err := database.DB.First(&user, "id = ?", userID).Error; err != nil {
		return fmt.Errorf("user not found: %s", userID)
	}

	eventName := event.Meta.EventName
	subStatus := event.Data.Attributes.Status
	subID := event.Data.ID

	util.Log.Info("[billing] Webhook: event=%s status=%s user=%s sub=%s", eventName, subStatus, userID, subID)

	switch eventName {
	case "subscription_created", "subscription_updated", "subscription_resumed":
		if subStatus == "active" || subStatus == "on_trial" {
			// Activate/renew Pro
			renewsAt := parseTime(event.Data.Attributes.RenewsAt)
			if renewsAt == nil {
				// Fallback: 30 days from now
				t := time.Now().AddDate(0, 1, 0)
				renewsAt = &t
			}
			updates := map[string]interface{}{
				"pro_expires_at":        renewsAt,
				"lemon_subscription_id": subID,
			}
			// Reset credits on new subscription / renewal
			if eventName == "subscription_created" {
				updates["credits"] = 5000
				updates["credits_reset"] = time.Now().Truncate(24 * time.Hour).AddDate(0, 1, 0)
			}
			database.DB.Model(&user).Updates(updates)
			util.Log.Info("[billing] Pro activated for user %s until %s", userID, renewsAt.Format(time.RFC3339))
		}

	case "subscription_cancelled", "subscription_expired":
		// Don't immediately revoke — let them use until pro_expires_at.
		// Just log it. When pro_expires_at passes, IsPro() returns false automatically.
		database.DB.Model(&user).Update("lemon_subscription_id", "")
		util.Log.Info("[billing] Subscription %s for user %s (status: %s)", eventName, userID, subStatus)

	case "subscription_payment_success":
		// Monthly renewal — reset credits
		renewsAt := parseTime(event.Data.Attributes.RenewsAt)
		if renewsAt != nil {
			database.DB.Model(&user).Updates(map[string]interface{}{
				"pro_expires_at": renewsAt,
				"credits":        5000,
				"credits_reset":  time.Now().Truncate(24 * time.Hour).AddDate(0, 1, 0),
			})
			util.Log.Info("[billing] Pro renewed for user %s, credits reset to 5000", userID)
		}

	case "subscription_payment_failed":
		util.Log.Warn("[billing] Payment failed for user %s, subscription %s", userID, subID)
	}

	return nil
}

// GetBillingStatus returns the current billing status for a user.
func GetBillingStatus(userID uuid.UUID) (map[string]interface{}, error) {
	var user models.User
	if err := database.DB.First(&user, "id = ?", userID).Error; err != nil {
		return nil, fmt.Errorf("user not found")
	}

	result := map[string]interface{}{
		"is_pro":         user.IsPro(),
		"credits":        user.Credits,
		"credits_reset":  user.CreditsReset,
		"plan":           "free",
	}

	if user.IsPro() {
		result["plan"] = "pro"
		result["pro_expires_at"] = user.ProExpiresAt
	}

	return result, nil
}

func parseTime(s string) *time.Time {
	if s == "" {
		return nil
	}
	t, err := time.Parse(time.RFC3339, s)
	if err != nil {
		return nil
	}
	return &t
}
