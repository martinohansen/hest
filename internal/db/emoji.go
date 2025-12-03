package db

import (
	"hash/fnv"
	"strconv"
)

var emojiPool = []string{
	"🦄", "🐍", "🦁", "🐯", "🐺", "🦊", "🐼", "🐻", "🐨", "🐮",
	"🐷", "🐸", "🐵", "🦍", "🦝", "🦓", "🦒", "🦛", "🦘", "🐙",
	"🦑", "🦀", "🦞", "🦐", "🐠", "🐬", "🐳", "🐊", "🦈", "🦚",
	"🦜", "🦢", "🦩", "🦉", "🦅", "🦆", "🐧", "🐤", "🐝", "🌳",
	"🪲", "🦋", "🐞", "🐌", "🐢", "🐇", "🐿", "🦔", "🦥", "🍕",
	"🍔", "🍟", "🌭", "🍿", "🧃", "🍪", "🎂", "🍊", "⚽", "🏀",
	"🏈", "⚾", "🎾", "🏒", "🏓", "🚗", "🚕", "🚌", "🚓", "🚑",
	"✈", "🛸", "👔", "🧤", "🧣", "👞", "🎸", "🎹", "🎺", "🎻",
	"🥁", "🎤", "🎧", "🎵", "🌙", "🌈", "🔥", "💧", "🎮", "🎯",
	"🎲", "🦖", "🦫", "🌸", "🧬", "🦕", "🐉", "🦗", "🕷", "🦂",
	"🦟", "🦠", "🌼", "🌴", "🌲", "🌺", "🌻", "🌷", "🤿", "🍳",
}

func emojiIndexSeed(name string, id int) int {
	if len(emojiPool) == 0 {
		return 0
	}
	h := fnv.New32a()
	_, _ = h.Write([]byte(name))
	_, _ = h.Write([]byte{0})
	_, _ = h.Write([]byte(strconv.Itoa(id)))
	return int(h.Sum32() % uint32(len(emojiPool)))
}

// chooseEmoji picks the first unused emoji from the pool, starting at a stable offset.
func chooseEmoji(name string, id int, used map[string]struct{}) string {
	if len(emojiPool) == 0 {
		return ""
	}
	start := emojiIndexSeed(name, id)
	for i := 0; i < len(emojiPool); i++ {
		idx := (start + i) % len(emojiPool)
		e := emojiPool[idx]
		if _, ok := used[e]; ok {
			continue
		}
		return e
	}
	// Pool exhausted: reuse a deterministic slot to avoid errors.
	return emojiPool[start%len(emojiPool)]
}
