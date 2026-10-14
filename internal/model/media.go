package model

// MediaKind determines the native Telegram media method used for an RSS attachment.
type MediaKind string

const (
	MediaPhoto     MediaKind = "photo"
	MediaVideo     MediaKind = "video"
	MediaAnimation MediaKind = "animation"
	MediaAudio     MediaKind = "audio"
)

// Media is only carried during RSS delivery, not stored in the database.
type Media struct {
	URL  string
	Kind MediaKind
}
