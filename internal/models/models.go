package models

type User struct {
	ID       int64  `db:"id" json:"id"`
	Email    string `db:"email" json:"email"`
	Password string `db:"password" json:"-"`
}

// RefreshToken stored in Postgres
// JWT blacklist or rate limit in Redis

type RefreshToken struct {
	ID     int64  `db:"id"`
	UserID int64  `db:"user_id"`
	Token  string `db:"token"`
}
