package services

import (
	"context"
	"fmt"

	"github.com/DevKenny/slack-spotify/src/domain"
	"github.com/DevKenny/slack-spotify/src/repositories"
	"github.com/google/uuid"
	"github.com/slack-go/slack"
	"github.com/zmb3/spotify"
	"golang.org/x/oauth2"
)

type services struct {
	repositories         repositories.Repositories
	spotifyAuthenticator spotify.Authenticator
}

type Services interface {
	AddUser(ctx context.Context, user domain.User) error
	ChangeUserStatus(ctx context.Context) error
}

func NewServices(repositories repositories.Repositories, spotifyAuthenticator spotify.Authenticator) Services {
	return services{
		repositories,
		spotifyAuthenticator,
	}
}

func (s services) AddUser(ctx context.Context, user domain.User) error {
	user.ID = uuid.New().String()
	return s.repositories.CreateUser(ctx, user)
}

func (s services) ChangeUserStatus(ctx context.Context) error {
	users, err := s.repositories.SearchUsers(ctx)
	if err != nil {
		return err
	}

	for _, user := range users {
		go func(user domain.User) {
			println("User => ", user.ID)
			slackApi := slack.New(user.SlackAccessToken)

			spotifyToken := oauth2.Token{
				AccessToken:  user.SpotifyAccessToken,
				RefreshToken: user.SpotifyRefreshToken,
				Expiry:       user.SpotifyExpiry,
				TokenType:    user.SpotifyTokenType,
			}
			spotifyApi := s.spotifyAuthenticator.NewClient(&spotifyToken)

			player, err := spotifyApi.PlayerCurrentlyPlaying()
			if err != nil {
				fmt.Printf("Error spotify currently playing: %s\n", err)
				return
			}

			if player == nil || player.Item == nil {
				return
			}

			profile, err := slackApi.GetUserProfile(&slack.GetUserProfileParameters{UserID: user.SlackUserID})
			if err != nil {
				fmt.Printf("Error slack get user profile: %s\n", err)
				return
			}

			canUpdateStatus := player.Playing && (profile.StatusEmoji == ":spotify:" || profile.StatusEmoji == "")
			canClearStatus := !player.Playing && profile.StatusEmoji == ":spotify:"
			if !canUpdateStatus && !canClearStatus {
				return
			}

			if canUpdateStatus {
				songName := player.Item.Name
				println("Song name => ", songName)
				slackStatus := songName + " - " + player.Item.Artists[0].Name
				println("Slack Status => ", slackStatus)
				if len(slackStatus) > 100 {
					extraChars := len(slackStatus) - 100 + 3
					songName = player.Item.Name[:len(player.Item.Name)-extraChars]
					slackStatus = songName + "... - " + player.Item.Artists[0].Name
				}

				println("Slack User ID => ", user.SlackUserID)
				err = slackApi.SetUserCustomStatusWithUser(user.SlackUserID, slackStatus, ":spotify:", 0)
				if err != nil {
					fmt.Printf("Error slack set user custom status: %s\n", err)
				}
				return
			}

			if canClearStatus {
				err = slackApi.SetUserCustomStatusWithUser(user.SlackUserID, "", "", 0)
				if err != nil {
					fmt.Printf("Error slack set user custom status: %s\n", err)
				}
				return
			}
		}(user)
	}

	return nil
}
