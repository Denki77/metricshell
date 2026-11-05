package releaseverify

import "testing"

func TestPublishedV022History(t *testing.T) {
	for _, path := range []string{"docs/07-delivery/releases/v0.2.2.md", "docs-ru/07-delivery/releases/v0.2.2.md"} {
		requireAll(t, readProjectText(t, path), "4b19f7e20b2aab366c5cb69edd9e831bc7c89d58", "sha256:20d5b0cec5195d677f49b65bf315637bf0fed6899a936de527392f85df57f8ec")
	}
}
