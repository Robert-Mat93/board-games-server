package main

type GameStarResponse struct {
	GameID string `json:"game_id"`
}

type GameEvent struct {
	GameID string `json:"game_id"`
	Event  string `json:"event"`
}

type User struct {
	Name string `json:"name"`
	ID   string `json:"id"`
}

type LogIn struct {
	UserID string `json:"user_id"`
}
