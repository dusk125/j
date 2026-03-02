package job

import (
	"fmt"
	"math/rand"
)

var adjectives = []string{
	"agile", "bold", "brave", "bright", "calm",
	"clever", "cool", "crisp", "deft", "eager",
	"epic", "fair", "fancy", "fast", "fierce",
	"fond", "frank", "fresh", "glad", "grand",
	"happy", "hardy", "hasty", "jolly", "keen",
	"kind", "lame", "large", "lean", "light",
	"lucky", "merry", "mild", "nifty", "noble",
	"odd", "perky", "plain", "plush", "proud",
	"quick", "quiet", "rapid", "sharp", "shiny",
	"silky", "smart", "snug", "spicy", "swift",
}

var nouns = []string{
	"ant", "bear", "bird", "cat", "cobra",
	"crane", "crow", "deer", "dove", "duck",
	"eagle", "eel", "elk", "emu", "falcon",
	"finch", "fox", "frog", "goat", "gull",
	"hare", "hawk", "ibis", "jay", "koala",
	"lark", "lion", "lynx", "mole", "moth",
	"newt", "orca", "otter", "owl", "panda",
	"pike", "puma", "quail", "ram", "raven",
	"robin", "seal", "shark", "snail", "snake",
	"squid", "stork", "swan", "tiger", "toad",
}

func GenerateName() string {
	for i := 0; i < 100; i++ {
		adj := adjectives[rand.Intn(len(adjectives))]
		noun := nouns[rand.Intn(len(nouns))]
		name := fmt.Sprintf("%s_%s", adj, noun)
		if !JobExists(name) {
			return name
		}
	}
	// Fallback with random suffix
	adj := adjectives[rand.Intn(len(adjectives))]
	noun := nouns[rand.Intn(len(nouns))]
	return fmt.Sprintf("%s_%s_%d", adj, noun, rand.Intn(10000))
}
