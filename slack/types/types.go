package types

// Channel contains information about a team channel.
type Channel struct {
	// ID is the uuid for this channel
	ID string `json:"id"`
	// Name of the the channel without leading hash sign.
	Name string `json:"name"`
	// IsChannel is true if the object is a channel (always set for channels)
	IsChannel bool `json:"is_channel"`
	// Created is the unix timestamp when the channel was created
	Created int64 `json:"created"`
	// Creator is the user ID of the creator of the channel
	Creator string `json:"creator"`
	// IsArchived is true if the channel is archived
	IsArchived bool `json:"is_archived"`
	// IsGeneral is true if the channel is the "general" channel that includes
	// all regular team members. In most teams this is called `#general` but
	// some teams have renamed it.
	IsGeneral bool `json:"is_general"`
	// Members is a list of user IDs for all uers in this channel. This includes
	// any disabled accounts that were in this channel when they were disabled.
	Members []string `json:"members"`
	// Topic is optional current topic of discussion on the channel
	Topic Property `json:"topic,omitempty"`
	// Purpose is optional "mission statement" for the channel
	Purpose Property `json:"purpose,omitempty"`
	// IsMember is true if the calling member is part of the channel
	IsMember bool `json:"is_member"`
	// LastRead is an optional timestamp for the last message the calling
	// member has read in this channel
	LastRead string `json:"last_read,omitempty"`
	// Latest is the last message posted to the channel
	//Latest Message `json:"latest,omitempty"`

	// UnreadCount is a full count of visible messages thaththe calling user
	// has yet to read
	UnreadCount int64 `json:"unread_count,omitempty"`
	// UnreadCountDisplay is a count of messages that the calling user
	// has yet to read that matter to them (this means it excludes things
	// like join/leave messages).
	UnreadCountDisplay int64 `json:"unread_count_display,omitempty"`
}

// Property represents a generic named property which is used on several
// slack data types.
type Property struct {
	// Value contains the property value.
	Value string `json:"value"`
	// Creator is the user ID of the creator of the property.
	Creator string `json:"creator"`
	// LastSet is the unix timestamp when the property was last set.
	LastSet int64 `json:"last_set"`
}
