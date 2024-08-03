package notifications

import (
	"errors"
	"log/slog"
	"regexp"

	"github.com/bwmarrin/discordgo"
	"github.com/tez-capital/protocol-rewards/constants"
)

type DiscordNotificatorConfiguration struct {
	WebhookUrl   string `json:"webhook_url"`
	WebhookId    string `json:"webhook_id"`
	WebhookToken string `json:"webhook_token"`
}

type DiscordNotificator struct {
	session *discordgo.Session
	token   string
	id      string
}

const (
	// https://github.com/discordjs/discord.js/blob/aec44a0c93f620b22242f35e626d817e831fc8cb/packages/discord.js/src/util/Util.js#L517
	DISCORD_WEBHOOK_REGEX = `https?:\/\/(?:ptb\.|canary\.)?discord\.com\/api(?:\/v\d{1,2})?\/webhooks\/(\d{17,19})\/([\w-]{68})`
)

func InitDiscordNotificator(config *DiscordNotificatorConfiguration) (*DiscordNotificator, error) {
	id := config.WebhookId
	token := config.WebhookToken
	if config.WebhookUrl != "" {
		wr, err := regexp.Compile(DISCORD_WEBHOOK_REGEX)
		if err != nil {
			return nil, err
		}
		matched := wr.FindStringSubmatch(config.WebhookUrl)
		if len(matched) > 2 {
			id = matched[1]
			token = matched[2]
		} else {
			slog.Warn("failed to parse discord webhook")
		}
	}

	session, err := discordgo.New("")
	if err != nil {
		return nil, err
	}

	slog.Debug("discord notificator initialized")

	return &DiscordNotificator{
		session: session,
		id:      id,
		token:   token,
	}, nil
}

func ValidateDiscordConfiguration(config *DiscordNotificatorConfiguration) error {
	id := config.WebhookId
	token := config.WebhookToken
	if config.WebhookUrl != "" {
		wr, err := regexp.Compile(DISCORD_WEBHOOK_REGEX)
		if err != nil {
			return err
		}
		matched := wr.FindStringSubmatch(config.WebhookUrl)
		if len(matched) > 2 {
			id = matched[1]
			token = matched[2]
		} else {
			return errors.Join(constants.ErrInvalidNotificatorConfiguration, errors.New("failed to parse discord webhook"))
		}
	}
	if id == "" {
		if config.WebhookUrl != "" {
			return errors.Join(constants.ErrInvalidNotificatorConfiguration, errors.New("invalid discord webhook url - failed to parse id"))
		}
		return errors.Join(constants.ErrInvalidNotificatorConfiguration, errors.New("invalid discord webhook id"))
	}
	if token == "" {
		if config.WebhookUrl != "" {
			return errors.Join(constants.ErrInvalidNotificatorConfiguration, errors.New("invalid discord webhook url - failed to parse token"))
		}
		return errors.Join(constants.ErrInvalidNotificatorConfiguration, errors.New("invalid discord webhook token"))
	}
	return nil
}

func (dn *DiscordNotificator) notify(msg string) error {
	_, err := dn.session.WebhookExecute(dn.id, dn.token, true, &discordgo.WebhookParams{
		Content: msg,
	})
	return err
}

func Notify(notificator *DiscordNotificator, msg string) {
	slog.Debug("sending discord notification")

	if err := notificator.notify(msg); err != nil {
		slog.Warn("failed to send notification", "error", err)
	}
	slog.Debug("discord notification sent", "message", msg)
}
