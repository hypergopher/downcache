package downcache

type AuthorLink struct {
	Name string `json:"name" yaml:"name" toml:"name"`
	Icon string `json:"icon" yaml:"icon" toml:"icon"`
	URL  string `json:"url" yaml:"url" toml:"url"`
}

type Author struct {
	Username  string       `json:"username" yaml:"username" toml:"username"`
	Name      string       `json:"name" yaml:"name" toml:"name"`
	Country   string       `json:"country" yaml:"country" toml:"country"`
	Active    bool         `json:"active" yaml:"active" toml:"active"`
	Bio       string       `json:"bio" yaml:"bio" toml:"bio"`
	AvatarURL string       `json:"avatarURL" yaml:"avatarURL" toml:"avatarURL"`
	Links     []AuthorLink `json:"links" yaml:"links" toml:"links"`
}
